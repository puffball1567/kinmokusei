package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
)

func TestParsesGenericInterfaceDeclaration(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
interface Transformer<T, U> {
  function transform(value: T): U;
  function nested(values: T[]): Map<string, U>;
}
class Length implements Transformer<string, int> {
  public function transform(value: string): int { return len(value); }
  public function nested(values: string[]): Map<string, int> { return makeMap<string, int>(); }
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
	declaration := program.Declarations[0].(*ast.InterfaceDecl)
	if len(declaration.TypeParameters) != 2 || declaration.TypeParameters[0].Name != "T" || declaration.TypeParameters[1].Name != "U" {
		t.Fatalf("type parameters = %#v", declaration.TypeParameters)
	}
	if declaration.Methods[0].Parameters[0].Type.Name != "T" || declaration.Methods[0].ReturnType.Name != "U" {
		t.Fatalf("generic method = %#v", declaration.Methods[0])
	}
	class := program.Declarations[1].(*ast.ClassDecl)
	if len(class.Implements) != 1 || len(class.Implements[0].GenericArguments) != 2 {
		t.Fatalf("implements = %#v", class.Implements)
	}
}

func TestGenericInterfaceSyntaxFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"empty", `interface Box<> {}`, "generic interface type parameter list cannot be empty"},
		{"trailing comma", `interface Pair<T,> {}`, "expected generic interface type parameter name after ','"},
		{"missing close", `interface Box<T {}`, "expected '>' after generic interface type parameters"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("generic_interface_failure.otm", test.source)
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
