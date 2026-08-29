package parser

import (
	"strings"
	"testing"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/lexer"
)

func TestParsesDefinedTypesAndAliases(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
type UserID = distinct string;
type Scores = distinct int[];
alias UserIDText = string;
`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
	if len(program.Declarations) != 3 {
		t.Fatalf("declarations = %d", len(program.Declarations))
	}
	userID := program.Declarations[0].(*ast.TypeDecl)
	if userID.Name != "UserID" || userID.Alias || userID.Underlying.Name != "string" {
		t.Fatalf("defined type = %#v", userID)
	}
	scores := program.Declarations[1].(*ast.TypeDecl)
	if scores.Alias || !scores.Underlying.IsSlice() || scores.Underlying.Element.Name != "int" {
		t.Fatalf("slice defined type = %#v", scores)
	}
	alias := program.Declarations[2].(*ast.TypeDecl)
	if alias.Name != "UserIDText" || !alias.Alias || alias.Underlying.Name != "string" {
		t.Fatalf("alias = %#v", alias)
	}
}

func TestDefinedTypeSyntaxFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing name", `type = distinct string;`, "expected type name"},
		{"missing assignment", `type UserID distinct string;`, "expected '=' after type name"},
		{"missing distinct", `type UserID = string;`, "defined type requires 'distinct'"},
		{"distinct alias", `alias UserID = distinct string;`, "alias declarations are transparent"},
		{"missing underlying", `type UserID = distinct;`, "expected type"},
		{"missing semicolon", `alias UserID = string`, "expected ';' after type declaration"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("defined_failure.otm", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			_, diagnostics := Parse(tokens)
			var messages []string
			for _, item := range diagnostics {
				messages = append(messages, item.Message)
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestDefinedTypeWordsRemainContextualIdentifiers(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
class type {}
function use(type: int, alias: int, distinct: int): int {
  const value: int = type + alias;
  return value + distinct;
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
	if len(program.Declarations) != 2 {
		t.Fatalf("declarations = %d", len(program.Declarations))
	}
	class := program.Declarations[0].(*ast.ClassDecl)
	function := program.Declarations[1].(*ast.FunctionDecl)
	if class.Name != "type" || function.Parameters[0].Name != "type" || function.Parameters[1].Name != "alias" || function.Parameters[2].Name != "distinct" {
		t.Fatalf("contextual identifiers were not preserved: class=%q parameters=%#v", class.Name, function.Parameters)
	}
}
