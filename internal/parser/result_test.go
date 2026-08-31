package parser

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
)

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
