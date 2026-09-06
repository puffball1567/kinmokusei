package sema

import (
	"go/importer"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestSemanticDiagnosticContracts(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		message      string
		line, column int
	}{
		{"undefined name", `function value(): int { return missing; }`, `undefined name "missing"`, 1, 32},
		{"return type mismatch", `function value(): int { return "x"; }`, "cannot use string as int", 1, 32},
		{"uninitialized constructor field", `class User {} class Holder { private user: User; constructor() {} }`, `non-null field "user" of type User must be initialized on every constructor path; assign this.user or declare it as User | null`, 1, 38},
		{"mixed zero length switch cannot prove range entry", `class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case 0, 1 { for (const value of values) { this.user = new User(); } } default { this.user = new User(); } } } }`, `non-null field "user" of type User must be initialized on every constructor path; assign this.user or declare it as User | null`, 1, 38},
		{"nullable member access", "class User { public name: string; }\nfunction read(user: User | null): string {\n  return user.name;\n}", "nullable value User | null must be checked against null before member access", 3, 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("diagnostic.km", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			program, parseDiagnostics := parser.Parse(tokens)
			if len(parseDiagnostics) != 0 {
				t.Fatalf("parser diagnostics = %v", parseDiagnostics)
			}
			diagnostics := CheckScopedWithGoImporter(program, nil, importer.Default())
			if len(diagnostics) != 1 {
				t.Fatalf("diagnostics = %v, want exactly one", diagnostics)
			}
			got := diagnostics[0]
			if got.Message != test.message || got.Span.Path != "diagnostic.km" || got.Span.Start.Line != test.line || got.Span.Start.Column != test.column {
				t.Fatalf("diagnostic = %#v, want %q at diagnostic.km:%d:%d", got, test.message, test.line, test.column)
			}
		})
	}
}
