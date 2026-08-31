package parser

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
)

func TestParsesTypedExceptions(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function run(err: error): void {
  try {
    throw err;
  } catch (specific: FirstError) {
    throw specific;
  } catch (caught: error) {
    throw;
  } finally {
    finish();
  }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	statement, ok := function.Body.Statements[0].(*ast.TryStmt)
	if !ok {
		t.Fatalf("statement = %T", function.Body.Statements[0])
	}
	if len(statement.Catches) != 2 || statement.Catches[0].Name != "specific" || statement.Catches[0].Type.Name != "FirstError" || statement.Catches[1].Name != "caught" || statement.Catches[1].Type.Name != "error" || statement.FinallyBody == nil {
		t.Fatalf("try statement = %#v", statement)
	}
	if _, ok := statement.Body.Statements[0].(*ast.ThrowStmt); !ok {
		t.Fatalf("try body statement = %T", statement.Body.Statements[0])
	}
	if thrown, ok := statement.Catches[1].Body.Statements[0].(*ast.ThrowStmt); !ok || !thrown.Bare || thrown.Value != nil {
		t.Fatalf("catch body statement = %T", statement.Catches[1].Body.Statements[0])
	}
}

func TestParsesCatchOnlyFinallyOnlyAndBlankCatch(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function run(err: error): void {
  try { throw err; } catch (_: error) {}
  try { work(); } finally { finish(); }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	catchOnly := function.Body.Statements[0].(*ast.TryStmt)
	finallyOnly := function.Body.Statements[1].(*ast.TryStmt)
	if len(catchOnly.Catches) != 1 || catchOnly.Catches[0].Name != "_" || catchOnly.FinallyBody != nil {
		t.Fatalf("catch-only statement = %#v", catchOnly)
	}
	if len(finallyOnly.Catches) != 0 || finallyOnly.FinallyBody == nil {
		t.Fatalf("finally-only statement = %#v", finallyOnly)
	}
}

func TestExceptionParserFailureMatrix(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"try without handler", `function run(): void { try {} }`},
		{"try without block", `function run(): void { try catch (_: error) {} }`},
		{"catch without paren", `function run(): void { try {} catch err: error {} }`},
		{"catch without type", `function run(): void { try {} catch (err) {} }`},
		{"catch without body", `function run(): void { try {} catch (err: error) }`},
		{"finally without body", `function run(): void { try {} finally }`},
		{"throw without semicolon", `function run(err: error): void { throw err }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, diagnosticCount := parseSource(t, test.source)
			if diagnosticCount == 0 {
				t.Fatal("expected parser diagnostic")
			}
		})
	}
}
