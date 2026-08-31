package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
)

func TestParsesGenericStructDeclarationAndLiteral(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
struct Pair<T, U> {
  public first: T;
  public second: U;
  public function value(): T { return this.first; }
}
function make(): Pair<string, int> {
  return Pair<string, int> { first: "onsen", second: 1 };
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
	declaration := program.Declarations[0].(*ast.StructDecl)
	if len(declaration.TypeParameters) != 2 || declaration.TypeParameters[0].Name != "T" || declaration.TypeParameters[1].Name != "U" {
		t.Fatalf("type parameters = %#v", declaration.TypeParameters)
	}
	if declaration.Fields[0].Type.Name != "T" || declaration.Methods[0].ReturnType.Name != "T" {
		t.Fatalf("generic members = fields=%#v methods=%#v", declaration.Fields, declaration.Methods)
	}
	function := program.Declarations[1].(*ast.FunctionDecl)
	if len(function.ReturnType.GenericArguments) != 2 {
		t.Fatalf("generic return type = %#v", function.ReturnType)
	}
	literal, ok := function.Body.Statements[0].(*ast.ReturnStmt).Value.(*ast.GoCompositeLiteralExpr)
	if !ok || literal.Type.Name != "Pair" || len(literal.Type.GenericArguments) != 2 {
		t.Fatalf("generic struct literal = %#v", function.Body.Statements[0])
	}
}

func TestGenericStructSyntaxFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"empty", `struct Box<> {}`, "generic struct type parameter list cannot be empty"},
		{"trailing comma", `struct Box<T,> {}`, "expected generic struct type parameter name after ','"},
		{"missing close", `struct Box<T {}`, "expected '>' after generic struct type parameters"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("generic_struct_failure.otm", test.source)
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
