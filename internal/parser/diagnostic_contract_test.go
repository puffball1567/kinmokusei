package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestParserDiagnosticContracts(t *testing.T) {
	type expected struct {
		message      string
		line, column int
	}
	tests := []struct {
		name   string
		source string
		want   []expected
	}{
		{"empty import", `import {} from "./x";`, []expected{{"import list cannot be empty", 1, 9}}},
		{"missing function name", `function (): void {}`, []expected{{"expected function name", 1, 10}}},
		{"rest parameter is not a slice", `function broken(...values: int): void {}`, []expected{{"rest parameter type must be a slice", 1, 20}}},
		{"empty C ABI export", `export c() function value(): void {}`, []expected{{"C ABI export symbol list cannot be empty", 1, 10}, {"inline C ABI export expects exactly one symbol, got 0", 1, 12}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("diagnostic.km", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			program, diagnostics := Parse(tokens)
			if program == nil {
				t.Fatal("parser returned a nil recovery program")
			}
			if len(diagnostics) != len(test.want) {
				t.Fatalf("diagnostics = %v, want %d", diagnostics, len(test.want))
			}
			for index, want := range test.want {
				got := diagnostics[index]
				if got.Message != want.message || got.Span.Path != "diagnostic.km" || got.Span.Start.Line != want.line || got.Span.Start.Column != want.column {
					t.Fatalf("diagnostic %d = %#v, want %q at diagnostic.km:%d:%d", index, got, want.message, want.line, want.column)
				}
			}
		})
	}
}
