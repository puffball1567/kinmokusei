package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
)

func TestParsesGenericDefinedTypesAndAliases(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
type Values<T> = distinct T[];
type Lookup<K, V> = distinct Map<K, V>;
alias Legacy<T> = T[];
`)
	if diagnosticCount != 0 {
		t.Fatalf("parser diagnostics = %d", diagnosticCount)
	}
	if len(program.Declarations) != 3 {
		t.Fatalf("declarations = %d", len(program.Declarations))
	}
	values := program.Declarations[0].(*ast.TypeDecl)
	if values.Alias || len(values.TypeParameters) != 1 || values.TypeParameters[0].Name != "T" || !values.Underlying.IsSlice() || values.Underlying.Element.Name != "T" {
		t.Fatalf("generic slice defined type = %#v", values)
	}
	lookup := program.Declarations[1].(*ast.TypeDecl)
	if lookup.Alias || len(lookup.TypeParameters) != 2 || len(lookup.Underlying.GenericArguments) != 2 || lookup.Underlying.GenericArguments[0].Name != "K" || lookup.Underlying.GenericArguments[1].Name != "V" {
		t.Fatalf("generic map defined type = %#v", lookup)
	}
	alias := program.Declarations[2].(*ast.TypeDecl)
	if !alias.Alias || len(alias.TypeParameters) != 1 || alias.Underlying.Element.Name != "T" {
		t.Fatalf("generic alias = %#v", alias)
	}
}

func TestGenericDefinedTypeSyntaxFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"empty", `type Values<> = distinct int[];`, "generic defined type type parameter list cannot be empty"},
		{"trailing comma", `type Lookup<K,> = distinct Map<K, int>;`, "expected generic defined type type parameter name after ','"},
		{"missing close", `type Values<T = distinct T[];`, "expected '>' after generic defined type type parameters"},
		{"alias empty", `alias Values<> = int[];`, "generic alias type parameter list cannot be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("generic_defined_type_failure.otm", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			_, diagnostics := Parse(tokens)
			messages := make([]string, len(diagnostics))
			for index, item := range diagnostics {
				messages[index] = item.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}
