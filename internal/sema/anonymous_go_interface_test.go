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

func TestAnonymousGoInterfaceSemanticMatrix(t *testing.T) {
	set := gotoken.NewFileSet()
	file, err := goparser.ParseFile(set, "contracts.go", `package contracts
import "unsafe"
type Token int
type Secret interface{ hidden() }
type Unsafe interface{ Pointer()unsafe.Pointer }
type Wrapper[T any] interface { Get()interface{Read()T} }
func New()interface{Read()Token}{return nil}
func Use(interface{Read()Token}){}
func UseAny(interface{}){}
func Hidden()interface{Secret}{return nil}
func Raw()interface{Pointer()unsafe.Pointer}{return nil}
func EmbeddedRaw()interface{Unsafe}{return nil}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&gotypes.Config{GoVersion: "go1.23", Importer: importer.Default()}).Check("bounds.test/constraints", set, []*goast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, source, want string }{
		{"method value", `function use():int{const v=c.New();const read=v.Read;return int(read());}`, ""},
		{"nil", `function use():void{c.Use(nil);}`, ""},
		{"empty native interface", `interface I{function read():int;} function use(v:I):void{c.UseAny(v);}`, ""},
		{"missing method", `function use():void{c.Use(1);}`, "cannot use"},
		{"wrong signature", `import go fmt from "fmt";function use(v:fmt.Stringer):void{c.Use(v);}`, "cannot use"},
		{"unknown member", `function use():void{c.New().Missing();}`, "no exported member"},
		{"private inherited member", `function use():void{const v=c.Hidden();}`, "unexported method hidden"},
		{"unsafe direct", `function use():void{const v=c.Raw();}`, "uses unsafe.Pointer"},
		{"unsafe embedded", `function use():void{const v=c.EmbeddedRaw();}`, "uses unsafe.Pointer"},
		{"generic nested", `interface W<T> extends c.Wrapper<T>{} function use(v:W<int>):int{return v.Get().Read();}`, ""},
		{"generic source identity", `class Leaf{public value:int=1;}interface W<T> extends c.Wrapper<T>{} function use(v:W<Leaf>):int{return v.Get().Read().value;}`, ""},
		{"generic source nullable", `class Leaf{public value:int=1;}interface W<T> extends c.Wrapper<T>{} function use(v:W<Leaf|null>):int{return v.Get().Read().value;}`, "nullable"},
		{"generic source mismatch", `class Leaf{}class Other{}interface W<T> extends c.Wrapper<T>{} function use(a:W<Leaf>,b:W<Other>):void{let value=a.Get();value=b.Get();}`, "cannot use"},
		{"generic source nullable mismatch", `class Leaf{}interface W<T> extends c.Wrapper<T>{} function use(a:W<Leaf>,b:W<Leaf|null>):void{let value=a.Get();value=b.Get();}`, "cannot use"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("test.km", `import go c from "bounds.test/constraints";`+test.source)
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
	for _, name := range []string{"Raw", "EmbeddedRaw"} {
		if got := AssessGoInteropObject(pkg.Scope().Lookup(name)); got.Support != GoInteropRequiresUnsafe {
			t.Fatalf("%s assessment=%+v", name, got)
		}
	}
}
