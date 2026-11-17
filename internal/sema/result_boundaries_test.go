package sema

import (
	"go/importer"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestResultBoundaryContractsSurviveRechecking(t *testing.T) {
	t.Parallel()
	input := `import go context from "context";
alias F = () => int;
function source(): Result<F> { return ok(() => 7); }
function forward(): Result<F> { return source(); }
function propagate(): Result<F> { const value = source()?; return ok(value); }
function optionalSource(): Result<F | null> { return ok(null); }
function optional(): Result<F | null> { return optionalSource(); }
function optionalPropagate(): Result<F | null> { const value = optionalSource()?; return ok(value); }
function rawSource(): Result<context.CancelFunc> { return ok(nil); }
function raw(): Result<context.CancelFunc> { return rawSource(); }
function rawPropagate(): Result<context.CancelFunc> { const value = rawSource()?; return ok(value); }
function discard(): Result<void> { const _ = source()?; return ok(); }
function loop(): Result<F> {
  let again = false;
  while (again) { const value = source()?; again = false; }
  return source();
}
`
	tokens, diagnostics := lexer.Lex("result.km", input)
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
		functions := map[string]*ast.FunctionDecl{}
		for _, declaration := range program.Declarations {
			if function, ok := declaration.(*ast.FunctionDecl); ok {
				functions[function.Name] = function
			}
		}
		for name, checked := range map[string]bool{"forward": true, "optional": false, "raw": false} {
			returned := functions[name].Body.Statements[0].(*ast.ReturnStmt)
			if returned.ResultKind != ast.ResultForwardReturn || returned.NonNullResult != checked {
				t.Fatalf("%s: lost forwarding contract: %#v", name, returned)
			}
		}
		for name, checked := range map[string]bool{"propagate": true, "optionalPropagate": false, "rawPropagate": false, "discard": false} {
			binding := functions[name].Body.Statements[0].(*ast.VariableDecl)
			propagated := binding.Value.(*ast.PropagateExpr)
			if propagated.NonNull != checked {
				t.Fatalf("%s: lost propagation contract: %#v", name, propagated)
			}
		}
		loop := functions["loop"].Body.Statements[1].(*ast.WhileStmt)
		propagated := loop.Body.Statements[0].(*ast.VariableDecl).Value.(*ast.PropagateExpr)
		if !propagated.NonNull {
			t.Fatal("loop rechecking lost its propagation contract")
		}
	}
}
