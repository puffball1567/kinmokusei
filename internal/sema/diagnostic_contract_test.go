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
		{"incompatible constrained range", "constraint Values = ~int[] | ~[2]int;\nfunction use<T extends Values>(values: T): void {\n  for (const value of values) {}\n}", "range type parameter requires a common underlying range type or compatible receive-capable channels", 3, 23},
		{"class field initializer scope", "class Box {\n  public value: int = this.other;\n  public other: int;\n}", "class field initializers cannot reference this or super; use the constructor", 2, 23},
		{"undefined name", `function value(): int { return missing; }`, `undefined name "missing"`, 1, 32},
		{"type parameter constant overflow", "constraint Small = ~int8 | ~int64;\nfunction use<T extends Small>(): T { return T(128); }", "integer constant 128 cannot be converted to every type in T's type set", 2, 47},
		{"zero-yield iterator binding", "function ticks(yield: () => boolean): void {}\nfunction use(): void {\n  for (const value of ticks) {}\n}", "zero-value iterator range requires a single untyped '_' binding", 3, 3},
		{"interface inheritance cycle", "interface A extends A {}", "interface inheritance cycle involving A", 1, 21},
		{"integer range pair", "function use(): void {\n  for (const [i, j] of 3) {}\n}", "integer range requires exactly one binding, got 2", 2, 3},
		{"return type mismatch", `function value(): int { return "x"; }`, "cannot use string as int", 1, 32},
		{"generic method cannot rebind caller parameter", "class Box<T> { constructor(public value: T) {} public function keep<U>(marker: U): T { return this.value; } }\nfunction bad<U>(box: Box<U>): int {\n  return box.keep<int>(1);\n}", "cannot use U as int", 3, 10},
		{"uninitialized constructor field", `class User {} class Holder { private user: User; constructor() {} }`, `non-null field "user" of type User must be initialized on every constructor path; assign this.user or declare it as User | null`, 1, 38},
		{"effectful declaration invalidates range proof", `class User {} class Holder { private user: User; constructor(values: Map<string, int>, callback: (values: Map<string, int>) => int) { if (len(values) > 0) { const changed = callback(values); for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`, `non-null field "user" of type User must be initialized on every constructor path; assign this.user or declare it as User | null`, 1, 38},
		{"mixed zero length switch cannot prove range entry", `class User {} class Holder { private user: User; constructor(values: int[]) { switch (len(values)) { case 0, 1 { for (const value of values) { this.user = new User(); } } default { this.user = new User(); } } } }`, `non-null field "user" of type User must be initialized on every constructor path; assign this.user or declare it as User | null`, 1, 38},
		{"effectful nested guard cannot retain range proof", "class User {}\nclass Holder {\n  private user: User;\n  constructor(values: int[], callback: (values: int[]) => boolean) {\n    if (len(values) > 0) {\n      if (callback(values)) { for (const value of values) { this.user = new User(); } }\n      else { this.user = new User(); }\n    } else { this.user = new User(); }\n  }\n}", `non-null field "user" of type User must be initialized on every constructor path; assign this.user or declare it as User | null`, 3, 11},
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
