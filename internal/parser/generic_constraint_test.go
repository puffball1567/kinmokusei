package parser

import (
	"strings"
	"testing"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/lexer"
)

func TestParsesComparableTypeParameterConstraints(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function equal<T extends comparable>(left: T, right: T): boolean { return left === right; }
struct Key<T extends comparable> { public value: T; }
interface Matcher<T extends comparable> { function matches(value: T): boolean; }
type Lookup<T extends comparable> = distinct Map<T, string>;
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	declarations := [][]ast.TypeParameter{
		program.Declarations[0].(*ast.FunctionDecl).TypeParameters,
		program.Declarations[1].(*ast.StructDecl).TypeParameters,
		program.Declarations[2].(*ast.InterfaceDecl).TypeParameters,
		program.Declarations[3].(*ast.TypeDecl).TypeParameters,
	}
	for index, parameters := range declarations {
		if len(parameters) != 1 || parameters[0].Name != "T" || parameters[0].Constraint == nil || parameters[0].Constraint.Name != "comparable" {
			t.Fatalf("declaration %d type parameters = %#v", index, parameters)
		}
		if parameters[0].Span.End.Offset <= parameters[0].NameSpan.End.Offset {
			t.Fatalf("declaration %d constraint is not included in parameter span", index)
		}
	}
}

func TestComparableTypeParameterConstraintSyntaxFailure(t *testing.T) {
	tokens, lexDiagnostics := lexer.Lex("constraint_failure.otm", `function bad<T extends>(value: T): T { return value; }`)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
	}
	_, diagnostics := Parse(tokens)
	var messages []string
	for _, diagnostic := range diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	if !strings.Contains(strings.Join(messages, "\n"), "expected type name") {
		t.Fatalf("diagnostics = %v", messages)
	}
}
