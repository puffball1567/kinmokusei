package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
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
	tokens, lexDiagnostics := lexer.Lex("constraint_failure.km", `function bad<T extends>(value: T): T { return value; }`)
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

func TestParsesSourceTypeSetConstraint(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
constraint Integer = ~int | ~int8 | uint64;
function combine<T extends Integer>(left: T, right: T): T { return left + right; }
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	declaration, ok := program.Declarations[0].(*ast.InterfaceDecl)
	if !ok || !declaration.Constraint || declaration.Name != "Integer" || len(declaration.Terms) != 3 {
		t.Fatalf("constraint declaration = %#v", program.Declarations[0])
	}
	if !declaration.Terms[0].Underlying || declaration.Terms[0].Type.Name != "int" || !declaration.Terms[1].Underlying || declaration.Terms[1].Type.Name != "int8" || declaration.Terms[2].Underlying || declaration.Terms[2].Type.Name != "uint64" {
		t.Fatalf("constraint terms = %#v", declaration.Terms)
	}
	parameter := program.Declarations[1].(*ast.FunctionDecl).TypeParameters[0]
	if parameter.Constraint == nil || parameter.Constraint.Name != "Integer" {
		t.Fatalf("type parameter = %#v", parameter)
	}
}

func TestSourceTypeSetConstraintSyntaxFailures(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{`constraint = ~int;`, "expected constraint name"},
		{`constraint Integer ~int;`, "expected '=' after constraint name"},
		{`constraint Integer = ;`, "expected type name"},
		{`constraint Integer = ~int |;`, "expected constraint term after '|'"},
		{`constraint Integer = ~int`, "expected ';' after constraint declaration"},
	}
	for _, test := range tests {
		tokens, lexDiagnostics := lexer.Lex("constraint_failure.km", test.source)
		if len(lexDiagnostics) != 0 {
			t.Fatalf("lexer diagnostics for %q = %v", test.source, lexDiagnostics)
		}
		_, diagnostics := Parse(tokens)
		var messages []string
		for _, diagnostic := range diagnostics {
			messages = append(messages, diagnostic.Message)
		}
		if !strings.Contains(strings.Join(messages, "\n"), test.want) {
			t.Fatalf("diagnostics for %q = %v, want %q", test.source, messages, test.want)
		}
	}
}
