package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstraintMethodDiagnostics(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"conflicting method", `constraint Bad=c.IntMethod&c.StringMethod;`, "incompatible signatures for method Value"},
		{"conflicting generic methods", `constraint Bad=c.Getter<int>&c.Getter<string>;`, "incompatible signatures for method Get"},
		{"same generic methods", `constraint A<E>=c.Getter<E>&c.Getter<E>; function get<T extends A<int>>(value:T):int{return value.Get();}`, ""},
		{"self constraint", `constraint Equal<E>=c.Equal<E>; function same<T extends Equal<T>>(a:T,b:T):boolean{return a.Equal(b);}`, ""},
		{"pointer receiver required", `constraint A=c.Getter<int>; function get<T extends A>(value:T):int{return value.Get();} function bad(value:c.Box):int{return get(value);}`, "does not satisfy"},
		{"private method identity", `constraint A=c.Sealed; function get<T extends A>(value:T):int{return value.Get();} function bad(value:c.Box):int{return get(&value);}`, "does not satisfy"},
		{"sealed Go implementation", `constraint A=c.Sealed; function get<T extends A>(value:T):int{return value.Get();} function good(value:c.Closed):int{return get(value);}`, ""},
		{"private method inaccessible", `constraint A=c.Sealed; function bad<T extends A>(value:T):void{value.seal();}`, "no exported member"},
		{"nullable method argument", `alias Maybe=*c.Node|null; constraint A<E>=c.Getter<E>; constraint Bad=A<Maybe>;`, "method constraint arguments must preserve source type information"},
		{"direct nullable Go argument", `alias Maybe=*c.Node|null; constraint Bad=c.Getter<Maybe>;`, "method constraint arguments must preserve source type information"},
		{"native record method argument", `struct Record{public node:*c.Node|null;} constraint A<E>=c.Getter<E>; constraint Bad=A<Record>;`, "method constraint arguments must preserve source type information"},
		{"Result callback method argument", `constraint A<E>=c.Getter<E>; constraint Bad=A<()=>Result<int>>;`, "method constraint arguments must preserve source type information"},
		{"nullable argument through generic call", `alias Maybe=*c.Node|null; constraint A<E>=c.Getter<E>; function get<E,T extends A<E>>(value:T):E{return value.Get();} function bad(value:c.Nodes):Maybe{return get<Maybe,c.Nodes>(value);}`, "method constraint arguments must preserve source type information"},
		{"invalid method dependency", `constraint A=c.IntMethod&c.StringMethod; constraint B=A;`, "incompatible signatures"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"go.mod": "module method-diagnostics.test\n\ngo 1.23\n",
				"contracts/contracts.go": `package contracts
type IntMethod interface{Value()int}
type StringMethod interface{Value()string}
type Getter[E any]interface{Get()E}
type Equal[E any]interface{Equal(E)bool}
type Box struct{}
func(*Box)Get()int{return 1}
type Sealed interface{seal();Get()int}
type Closed struct{}
func(Closed)seal(){}
func(Closed)Get()int{return 1}
type Node struct{Value int}
type Nodes struct{}
func(Nodes)Get()*Node{return nil}
`,
				"entry.km": `import go c from "method-diagnostics.test/contracts";` + test.source,
			}
			for name, source := range files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "diagnostics")
			var messages []string
			for _, diagnostic := range diagnostics {
				messages = append(messages, diagnostic.Message)
			}
			if err != nil || test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("err=%v diagnostics=%v want=%q", err, diagnostics, test.want)
			}
		})
	}
}
