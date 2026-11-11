package sema

import (
	"go/importer"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestCheckedStorageBoundaryConditionsAndRechecking(t *testing.T) {
	t.Parallel()
	input := `alias F=()=>int;
function source():Result<F>{return ok(()=>3);}
function checked(m:Map<int,F>):void{let cb:F=()=>1;let ok=false;[cb,ok]=m[0];if(ok){cb();}}
function selected(ch:GoChannel<F>):void{let cb:F=()=>1;let open=false;select{case [cb,open]=<-ch{if(open){cb();}}}}
function result():void{let cb:F=()=>1;let err:error=nil;[cb,err]=source();if(err!==nil){return;}cb();}
function discarded(m:Map<int,F>):void{let cb:F=()=>1;[cb,_]=m[0];}
function raw(m:Map<int,int>):void{let value=0;let ok=false;[value,ok]=m[0];}
`
	tokens, ds := lexer.Lex("checked.km", input)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	program, ds := parser.Parse(tokens)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	for pass := 0; pass < 2; pass++ {
		if ds := CheckScopedWithGoImporterAndPolicy(program, nil, importer.Default(), GoInteropPolicy{}); len(ds) != 0 {
			t.Fatal(ds)
		}
		for _, test := range []struct {
			index     int
			condition ast.NonNullCondition
			statement int
		}{{2, ast.NonNullIfPresent, 2}, {4, ast.NonNullIfNoError, 2}, {5, ast.NonNullIfPresent, 1}} {
			assignment := program.Declarations[test.index].(*ast.FunctionDecl).Body.Statements[test.statement].(*ast.MultiAssignmentStmt)
			if assignment.NonNullCondition != test.condition || len(assignment.NonNullResults) != 2 || assignment.NonNullResults[0] == nil || assignment.NonNullResults[1] != nil || !assignment.NonNullResults[0].IsFunction() {
				t.Fatalf("lost conditional contract: %#v", assignment)
			}
		}
		selected := program.Declarations[3].(*ast.FunctionDecl).Body.Statements[2].(*ast.SelectStmt).Cases[0]
		if selected.NonNullCondition != ast.NonNullIfPresent || len(selected.NonNullResults) != 2 || selected.NonNullResults[0] == nil || selected.NonNullResults[1] != nil {
			t.Fatalf("lost selected contract: %#v", selected)
		}
		raw := program.Declarations[6].(*ast.FunctionDecl).Body.Statements[2].(*ast.MultiAssignmentStmt)
		for _, boundary := range raw.NonNullResults {
			if boundary != nil {
				t.Fatal("scalar received a reference check")
			}
		}
	}
}

func TestCheckedStorageStillRequiresPresenceProof(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"map":            `alias F=()=>int;function f(m:Map<int,F>):int{let cb:F=()=>1;let ok=false;[cb,ok]=m[0];return cb();}`,
		"receive":        `alias F=()=>int;function f(ch:GoChannel<F>):int{let cb:F=()=>1;let ok=false;[cb,ok]=<-ch;return cb();}`,
		"selected":       `alias F=()=>int;function f(ch:GoChannel<F>):int{let cb:F=()=>1;let ok=false;select{case [cb,ok]=<-ch{return cb();}}}`,
		"discarded flag": `alias F=()=>int;function f(m:Map<int,F>):int{let cb:F=()=>1;[cb,_]=m[0];return cb();}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(checkSource(t, input), "\n")
			if !strings.Contains(got, "check its corresponding ok") {
				t.Fatalf("missing presence diagnostic: %s", got)
			}
		})
	}
}

func TestSplitStorageContractsSurviveRechecking(t *testing.T) {
	t.Parallel()
	input := `alias F = () => int;
function source(): Result<F> { return ok(() => 3); }
function map(m: Map<int, F>): void { const [value, present] = m[0]; }
function result(): void { const [value, _] = source(); }
function raw(ch: GoChannel<*int>): void { const [value, present] = <-ch; }
function optional(ch: GoChannel<F | null>): void { const [value, present] = <-ch; }
function discard(ch: GoChannel<F>): void { const [_, present] = <-ch; }
function selected(ch: GoChannel<F>): void {
  select { case const [value, _] = <-ch {} }
}
function loop(m: Map<int, F>): void {
  for (let [value, present] = m[0]; present; present = false) {}
}
function tuple(): (F, F, boolean) { return () => 1, () => 2, false; }
function inferred(): void { const [a, b, flag] = tuple(); }
function discarded(): void { const [_, b, _] = tuple(); }
function initial(): void { for (let [a, b, flag] = tuple(); false;) {} }
`
	tokens, diagnostics := lexer.Lex("split.km", input)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	program, diagnostics := parser.Parse(tokens)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	for pass := 0; pass < 2; pass++ {
		if diagnostics := CheckScopedWithGoImporterAndPolicy(program, nil, importer.Default(), GoInteropPolicy{}); len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		for _, test := range []struct {
			index     int
			condition ast.NonNullCondition
			checked   bool
		}{
			{2, ast.NonNullIfPresent, true},
			{3, ast.NonNullIfNoError, true},
			{4, ast.NonNullIfPresent, false},
			{5, ast.NonNullIfPresent, false},
			{6, ast.NonNullIfPresent, false},
		} {
			declaration := program.Declarations[test.index].(*ast.FunctionDecl).Body.Statements[0].(*ast.MultiVariableDecl)
			if declaration.NonNullCondition != test.condition || declaration.Bindings[0].NonNull != test.checked || declaration.Bindings[1].NonNull {
				t.Fatalf("lost split contract: %#v", declaration)
			}
			if !declaration.Bindings[1].ResolvedType.IsSpecified() {
				t.Fatal("discarded flag lost its result type")
			}
		}
		selected := program.Declarations[7].(*ast.FunctionDecl).Body.Statements[0].(*ast.SelectStmt).Cases[0]
		if selected.NonNullCondition != ast.NonNullIfPresent || !selected.Bindings[0].NonNull || selected.Bindings[1].NonNull {
			t.Fatalf("lost selected split contract: %#v", selected)
		}
		loop := program.Declarations[8].(*ast.FunctionDecl).Body.Statements[0].(*ast.ForStmt).Initializer.(*ast.MultiVariableDecl)
		if loop.NonNullCondition != ast.NonNullIfPresent || !loop.Bindings[0].NonNull {
			t.Fatalf("lost loop split contract: %#v", loop)
		}
		ordinary := program.Declarations[10].(*ast.FunctionDecl).Body.Statements[0].(*ast.MultiVariableDecl)
		discarded := program.Declarations[11].(*ast.FunctionDecl).Body.Statements[0].(*ast.MultiVariableDecl)
		initial := program.Declarations[12].(*ast.FunctionDecl).Body.Statements[0].(*ast.ForStmt).Initializer.(*ast.MultiVariableDecl)
		for _, declaration := range []*ast.MultiVariableDecl{ordinary, initial} {
			if declaration.NonNullCondition != ast.NonNullAlways || !declaration.Bindings[0].NonNull || !declaration.Bindings[1].NonNull || declaration.Bindings[2].NonNull {
				t.Fatalf("lost ordinary inferred contract: %#v", declaration)
			}
		}
		if discarded.Bindings[0].NonNull || !discarded.Bindings[1].NonNull || discarded.Bindings[2].NonNull {
			t.Fatalf("lost ordinary discard contract: %#v", discarded)
		}
	}
}
