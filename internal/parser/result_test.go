package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestMalformedMultipleResultsRetainDiagnostics(t *testing.T) {
	for _, input := range []string{
		`function f():(int,) {}`,
		`function f():(int,string {}`,
		`function f():(int,[]) {}`,
		`const f=():(int,)=>0;`,
	} {
		if _, count := parseSource(t, input); count == 0 {
			t.Errorf("missing diagnostic for %s", input)
		}
	}
}

func TestParsesResultReturnTypeAndPropagationExpression(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function parse(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  return ok(value);
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	if function.ReturnType.Name != "Result" || len(function.ReturnType.GenericArguments) != 1 || function.ReturnType.GenericArguments[0].Name != "int" {
		t.Fatalf("return type = %#v", function.ReturnType)
	}
	variable := function.Body.Statements[0].(*ast.VariableDecl)
	propagated, ok := variable.Value.(*ast.PropagateExpr)
	if !ok {
		t.Fatalf("initializer = %T", variable.Value)
	}
	if _, ok := propagated.Value.(*ast.CallExpr); !ok {
		t.Fatalf("propagation operand = %T", propagated.Value)
	}
}

func TestParsesSourceMultipleResultReturnType(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function pair(value: int): (int, string) { return pair(value); }
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	result := program.Declarations[0].(*ast.FunctionDecl).ReturnType
	if len(result.GoResults) != 2 || result.GoResults[0].Name != "int" || result.GoResults[1].Name != "string" {
		t.Fatalf("return type = %#v", result)
	}
}

func TestParsesArrowMultipleResultReturnType(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
const pair = (value: int): (int, string) => makePair(value);
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	arrow := program.Declarations[0].(*ast.VariableDecl).Value.(*ast.ArrowExpr)
	if arrow.ReturnType == nil || len(arrow.ReturnType.GoResults) != 2 {
		t.Fatalf("arrow return type = %#v", arrow.ReturnType)
	}
}

func TestPropagationBindsOutsideAwaitAndCalls(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function work(): Result<int> { return ok(1); }
function run(): Result<int> {
  const task = go work();
  const first = work()?;
  const second = await task?;
  return ok(first + second);
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	run := program.Declarations[1].(*ast.FunctionDecl)
	first := run.Body.Statements[1].(*ast.VariableDecl).Value.(*ast.PropagateExpr)
	if _, ok := first.Value.(*ast.CallExpr); !ok {
		t.Fatalf("call propagation operand = %T", first.Value)
	}
	second := run.Body.Statements[2].(*ast.VariableDecl).Value.(*ast.PropagateExpr)
	if _, ok := second.Value.(*ast.AwaitExpr); !ok {
		t.Fatalf("await propagation operand = %T", second.Value)
	}
}
