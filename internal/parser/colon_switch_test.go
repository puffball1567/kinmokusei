package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestColonSwitchClauseBodies(t *testing.T) {
	t.Parallel()
	source := `function run(n: int): int {
  switch (n) {
    case 0:
    case 1:
      switch (n) { default { return 1 } }
      break
    default:
      { return 2 }
    case 3:
      return 3
  }
  return 0
}`
	tokens, lexDiagnostics := lexer.Lex("colon-switch.km", source)
	program, diagnostics := Parse(tokens)
	if len(lexDiagnostics) != 0 || len(diagnostics) != 0 {
		t.Fatalf("lex=%v parse=%v", lexDiagnostics, diagnostics)
	}
	stmt := program.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.ValueSwitchStmt)
	if len(stmt.Cases) != 4 || len(stmt.Cases[0].Body.Statements) != 0 || !stmt.Cases[2].Default {
		t.Fatalf("unexpected clause structure: %#v", stmt.Cases)
	}
	for _, clause := range stmt.Cases {
		if !clause.ColonBody {
			t.Fatalf("lost colon clause form: %#v", clause)
		}
	}
	if _, ok := stmt.Cases[1].Body.Statements[0].(*ast.ValueSwitchStmt); !ok {
		t.Fatal("nested switch must stay in its case body")
	}
	if _, ok := stmt.Cases[2].Body.Statements[0].(*ast.BlockStmt); !ok {
		t.Fatal("explicit case block must keep its scope")
	}
}

func TestColonSwitchRejectsMixedForms(t *testing.T) {
	t.Parallel()
	cases := []struct{ source, want string }{
		{`switch (n) { case 0: break; default {} }`, "cannot mix ':' clauses and braced clauses"},
		{`switch (n) { case 0 {} default: break; }`, "cannot mix ':' clauses and braced clauses"},
		{`switch (n) { case const err as error {} default: break; }`, "only supported in value switches"},
		{`switch (n) { case 0,: break; }`, "expected switch case value after ','"},
	}
	for _, tc := range cases {
		tokens, _ := lexer.Lex("colon-switch.km", "function run(n: int): void { "+tc.source+" }")
		_, diagnostics := Parse(tokens)
		found := false
		for _, diagnostic := range diagnostics {
			found = found || strings.Contains(diagnostic.Message, tc.want)
		}
		if !found {
			t.Errorf("%s: want %q, got %v", tc.source, tc.want, diagnostics)
		}
	}
}
