package sema

import (
	goast "go/ast"
	"go/importer"
	goparser "go/parser"
	gotoken "go/token"
	gotypes "go/types"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestGoInterfaceInheritanceSemanticMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"method call and value", `import go fmt from "fmt"; interface Named extends fmt.Stringer {} function use(value:Named):string{const read=value.String;return read();}`, ""},
		{"class and ancestor", `import go fmt from "fmt"; interface Named extends fmt.Stringer {} class Value implements Named {public function string():string{return "value";}} function use():fmt.Stringer{return new Value();} function cast(value:Named):fmt.Stringer{return value;}`, ""},
		{"generic diamond", `import go fmt from "fmt"; interface Root<T> extends fmt.Stringer {function read():T;} interface A<T> extends Root<T>{} interface B<T> extends Root<T>{} interface C<T> extends A<T>,B<T>{} class Value implements C<int>{public function string():string{return "value";}public function read():int{return 1;}} function use(value:C<int>):fmt.Stringer{return value;}`, ""},
		{"same Go spelling", `import go fmt from "fmt"; interface Named extends fmt.Stringer {function string():string;} class Value implements Named{public function string():string{return "value";}}`, ""},
		{"multiple result implementation", `import go io from "io"; interface Reader extends io.Reader {} class Value implements Reader{public function read(buffer:byte[]):Result<int>{return ok(0);}} function use(reader:Reader):int{const [n,err]=reader.Read([]);return n;}`, ""},
		{"source ancestor conflict", `import go fmt from "fmt"; interface A extends fmt.Stringer {} interface B{function string():int;} interface C extends A,B{}`, "incompatible signatures for Go method String"},
		{"own conflict", `import go fmt from "fmt"; interface Named extends fmt.Stringer {function string():int;}`, "incompatible signatures for Go method String"},
		{"missing", `import go fmt from "fmt"; interface Named extends fmt.Stringer {} class Value implements Named{}`, "missing method String"},
		{"private", `import go fmt from "fmt"; interface Named extends fmt.Stringer {} class Value implements Named{private function string():string{return "value";}}`, "must be public"},
		{"static", `import go fmt from "fmt"; interface Named extends fmt.Stringer {} class Value implements Named{public static function string():string{return "value";}}`, "cannot be static"},
		{"wrong result", `import go fmt from "fmt"; interface Named extends fmt.Stringer {} class Value implements Named{public function string():int{return 0;}}`, "incompatible signature"},
		{"reverse not implicit", `import go fmt from "fmt"; interface Named extends fmt.Stringer {} function use(value:fmt.Stringer):Named{return value;}`, "cannot use"},
		{"duplicate", `import go fmt from "fmt"; interface Named extends fmt.Stringer,fmt.Stringer {}`, "duplicate extended interface"},
		{"nullable rejected", `import go fmt from "fmt"; interface Named extends fmt.Stringer|null {}`, "interface extends expects"},
		{"pointer rejected", `import go fmt from "fmt"; interface Named extends *fmt.Stringer {}`, "interface extends expects"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}

func TestGoInterfaceInheritanceGenericAndRestrictedContracts(t *testing.T) {
	set := gotoken.NewFileSet()
	file, err := goparser.ParseFile(set, "contracts.go", `package contracts
import "unsafe"
type Reader[E any] interface { Read() E }
type Loader[E any] interface { Load() (E, error) }
type Hidden interface { hidden() }
type Unsafe interface { Pointer() unsafe.Pointer }
type Anonymous interface { Read(interface{Read() int}) }
type Integers interface { ~int }
type Property[E any] interface { GetValue() E; SetValue(E) }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&gotypes.Config{GoVersion: "go1.23", Importer: importer.Default()}).Check("bounds.test/constraints", set, []*goast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, source, want string }{
		{"property Go ancestor", `interface Cell<T> extends c.Property<T>{get value():T;set value(v:T);}class Box<T> implements Cell<T>{constructor(private raw:T){}public get value():T{return this.raw;}public set value(v:T){this.raw=v;}}function f(c:Cell<int>):int{c.value++;return c.GetValue();}`, ""},
		{"property Go ancestor conflict", `interface Cell extends c.Property<int>{get value():string;}`, "incompatible signatures for Go method GetValue"},
		{"property split Go ancestor conflict", `interface R{get value():string;}interface G extends c.Property<int>{}interface Cell extends R,G{}`, "incompatible signatures for Go method GetValue"},
		{"property reverse Go ancestor conflict", `interface R{get value():string;}interface G extends c.Property<int>{}interface Cell extends G,R{}`, "incompatible signatures for Go method GetValue"},
		{"generic inherited call", `interface Reader<T> extends c.Reader<T>{} class Box<T> implements Reader<T>{constructor(private value:T){}public function read():T{return this.value;}} function use(value:Reader<int>):int{return value.Read();} function make():c.Reader<int>{return new Box<int>(1);}`, ""},
		{"nullable generic result", `class Leaf{public value:int=1;} alias Maybe=Leaf|null; interface Reader<T> extends c.Reader<T>{} function bad(value:Reader<Maybe>):int{return value.Read().value;}`, "nullable"},
		{"concrete nullable result", `class Leaf{public value:int=1;} alias Maybe=Leaf|null; interface Generic<T> extends c.Reader<T>{} interface Reader extends Generic<Maybe>{} function bad(value:Reader):int{return value.Read().value;}`, "nullable"},
		{"native class identity", `class Leaf{public value:int=1;} interface Generic<T> extends c.Reader<T>{} interface Reader extends Generic<Leaf>{} class Box implements Reader{public function read():Leaf{return new Leaf();}} function use(value:Reader):int{return value.Read().value;}`, ""},
		{"nullable Result implementation", `class Leaf{} interface Generic<T> extends c.Loader<T>{} interface Loader extends Generic<Leaf|null>{} class Box implements Loader{public function load():Result<Leaf|null>{return ok(null);}}`, ""},
		{"nullable Result mismatch", `class Leaf{} interface Generic<T> extends c.Loader<T>{} interface Loader extends Generic<Leaf>{} class Bad implements Loader{public function load():Result<Leaf|null>{return ok(null);}}`, "incompatible signature"},
		{"nullable Go ancestor conflict", `class Leaf{} interface Generic<T> extends c.Reader<T>{} interface A extends Generic<Leaf>{} interface B extends Generic<Leaf|null>{} interface Bad extends A,B{}`, "incompatible signatures"},
		{"hidden", `interface Hidden extends c.Hidden{}`, "unexported Go method"},
		{"unsafe", `interface Unsafe extends c.Unsafe{}`, "uses unsafe.Pointer"},
		{"anonymous", `interface Anonymous extends c.Anonymous{}`, ""},
		{"type set", `interface Integers extends c.Integers{}`, "runtime Go interface"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("test.km", `import go c from "bounds.test/constraints"; `+test.source)
			program, parseDiagnostics := parser.Parse(tokens)
			if len(lexDiagnostics)+len(parseDiagnostics) != 0 {
				t.Fatalf("lex=%v parse=%v", lexDiagnostics, parseDiagnostics)
			}
			diagnostics := CheckScopedWithGoImporter(program, nil, dependentConstraintImporter{importer.Default(), pkg})
			var messages []string
			for _, d := range diagnostics {
				messages = append(messages, d.Message)
			}
			if test.want == "" && len(messages) != 0 || test.want != "" && !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics=%v want=%q", messages, test.want)
			}
		})
	}
}
