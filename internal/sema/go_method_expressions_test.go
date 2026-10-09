package sema

import (
	goast "go/ast"
	"go/importer"
	goparser "go/parser"
	"go/token"
	gotypes "go/types"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestGoMethodExpressions(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go time from "time"; const isZero=time.Time.IsZero; function f(v:time.Time):boolean{return isZero(v);}`,
		`import go { Time } from "time"; function f(v:Time):boolean{return Time.IsZero(v);}`,
		`import go bytes from "bytes"; function f():int{let b:bytes.Buffer=bytes.Buffer{}; const length=(*bytes.Buffer).Len; return length(&b);}`,
		`import go bytes from "bytes"; function f():int{let b:bytes.Buffer=bytes.Buffer{}; const write=(*bytes.Buffer).WriteString; const [n,e]=write(&b,"abc"); return n;}`,
		`import go io from "io"; function f(v:io.Reader, b:byte[]):int{const read=io.Reader.Read; const [n,e]=read(v,b);return n;}`,
	} {
		t.Run(input, func(t *testing.T) {
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
		})
	}
}

func TestGoMethodExpressionBoundaryMatrix(t *testing.T) {
	t.Parallel()
	set := token.NewFileSet()
	file, err := goparser.ParseFile(set, "contracts.go", `package contracts
import "unsafe"
type Value struct{N int}
func(Value)Raw()unsafe.Pointer{return nil}
func(Value)hidden(){}
type Constraint interface{~int;Method()}
type Box[T any]struct{N T}
func(v Box[T])Get()T{return v.N}
type IntBox=Box[int]
type Left struct{}
func(Left)Read()int{return 1}
type Right struct{}
func(Right)Read()int{return 2}
type Ambiguous struct{Left;Right}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&gotypes.Config{GoVersion: "go1.23", Importer: importer.Default()}).Check("bounds.test/constraints", set, []*goast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input, want string
		unsafe      bool
	}{
		{`const f=c.Value.Raw;`, "uses unsafe.Pointer", false},
		{`const f=c.Value.Raw;`, "", true},
		{`const f=c.Value.hidden;`, "no exported method", false},
		{`const f=c.Value.N;`, "no exported method", false},
		{`const f=c.Constraint.Method;`, "constraint interfaces", false},
		{`const f=c.Box.Get;`, "instantiated receiver", false},
		{`const f=c.IntBox.Get;function read():int{return f(c.IntBox{N:42});}`, "", false},
		{`const f=c.Ambiguous.Read;`, "no exported method", false},
	} {
		t.Run(test.input, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("test.km", `import go c from "bounds.test/constraints";`+test.input)
			program, parseDiagnostics := parser.Parse(tokens)
			if len(lexDiagnostics)+len(parseDiagnostics) != 0 {
				t.Fatalf("lex=%v parse=%v", lexDiagnostics, parseDiagnostics)
			}
			diagnostics := CheckScopedWithGoImporterAndPolicy(program, nil, dependentConstraintImporter{importer.Default(), pkg}, GoInteropPolicy{AllowUnsafe: test.unsafe})
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

func TestGoMethodExpressionDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, message string }{
		{`import go bytes from "bytes"; const f=bytes.Buffer.Len;`, "has no exported method"},
		{`import go io from "io"; const f=(*io.Reader).Read;`, "has no exported method"},
		{`import go time from "time"; const f=time.Time.wall;`, "has no exported method"},
		{`import go time from "time"; const f=time.Time.Missing;`, "has no exported method"},
		{`import go atomic from "sync/atomic"; const f=(*atomic.Pointer).Load;`, "instantiated receiver"},
		{`import go time from "time"; function f():boolean{return time.Time.IsZero(1);}`, "cannot use"},
		{`import go bytes from "bytes"; const f=(*bytes.Buffer);`, "cannot be used as a value"},
		{`import go bytes from "bytes"; const f=&(*bytes.Buffer);`, "addressable operand"},
		{`import go bytes from "bytes"; function f():void{let b:bytes.Buffer=bytes.Buffer{}; (*bytes.Buffer).Len(b);}`, "cannot use"},
	} {
		t.Run(test.input, func(t *testing.T) {
			diagnostics := checkSource(t, test.input)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.message) {
				t.Fatalf("want %q, got %v", test.message, diagnostics)
			}
		})
	}
}
