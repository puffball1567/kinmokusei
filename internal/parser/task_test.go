package parser

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
)

func TestParsesStructuredTaskExpressions(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function work(): Result<int> { return ok(1); }
function run(): Result<int> {
  const task: Task<Result<int>> = go work();
  const value = await task?;
  const ignored = go work();
  detach ignored;
  return ok(value);
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	run := program.Declarations[1].(*ast.FunctionDecl)
	task := run.Body.Statements[0].(*ast.VariableDecl)
	if task.Type.Name != "Task" || len(task.Type.GenericArguments) != 1 || task.Type.GenericArguments[0].Name != "Result" {
		t.Fatalf("task type = %#v", task.Type)
	}
	if _, ok := task.Value.(*ast.TaskStartExpr); !ok {
		t.Fatalf("task initializer = %T", task.Value)
	}
	value := run.Body.Statements[1].(*ast.VariableDecl).Value.(*ast.PropagateExpr)
	if _, ok := value.Value.(*ast.AwaitExpr); !ok {
		t.Fatalf("propagation operand = %T", value.Value)
	}
	if _, ok := run.Body.Statements[3].(*ast.DetachStmt); !ok {
		t.Fatalf("detach statement = %T", run.Body.Statements[3])
	}
}

func TestKeepsRawGoStatementDistinctFromTaskExpression(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function notify(): void {}
function run(): void {
  go notify();
  await go notify();
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	run := program.Declarations[1].(*ast.FunctionDecl)
	if statement, ok := run.Body.Statements[0].(*ast.CallControlStmt); !ok || statement.Kind != ast.GoCall {
		t.Fatalf("raw go statement = %#v", run.Body.Statements[0])
	}
	awaited := run.Body.Statements[1].(*ast.ExpressionStmt).Value.(*ast.AwaitExpr)
	if _, ok := awaited.Value.(*ast.TaskStartExpr); !ok {
		t.Fatalf("await operand = %T", awaited.Value)
	}
}

func TestRejectsNonCallTaskStartAndRecovers(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function bad(): void { const task = go 1; }
function recovered(): int { return 2; }
`)
	if diagnosticCount == 0 {
		t.Fatal("expected a parser diagnostic for a non-call go expression")
	}
	if len(program.Declarations) != 2 || program.Declarations[1].(*ast.FunctionDecl).Name != "recovered" {
		t.Fatalf("parser did not recover: %#v", program.Declarations)
	}
}
