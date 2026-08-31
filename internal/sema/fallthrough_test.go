package sema

import (
	"strings"
	"testing"
)

func TestFallthroughSemanticSuccessMatrix(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"case chain", `function value(input: int): int { let result = 0; switch (input) { case 0 { result += 1; fallthrough; } case 1 { result += 2; fallthrough; } default { result += 4; } } return result; }`},
		{"default in middle", `function value(input: int): int { switch (input) { case 0 { fallthrough; } default { fallthrough; } case 2 { return 2; } } return 3; }`},
		{"return proof through next case", `function value(input: int): int { switch (input) { case 0 { fallthrough; } case 1 { return 1; } default { return 2; } } }`},
		{"constructor initialization", `class Item {} class Holder { private item: Item; constructor(input: int) { switch (input) { case 0 { fallthrough; } case 1 { this.item = new Item(); } default { this.item = new Item(); } } } }`},
		{"nested value switch", `function value(outer: int, inner: int): int { switch (outer) { case 0 { switch (inner) { case 0 { fallthrough; } default { return 1; } } } default { return 2; } } return 3; }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
		})
	}
}

func TestFallthroughSemanticFailureMatrix(t *testing.T) {
	const placement = "fallthrough may only be used as the final statement of a non-final value switch case"
	for _, test := range []struct{ name, source, want string }{
		{"outside switch", `function bad(): void { fallthrough; }`, placement},
		{"final case", `function bad(input: int): void { switch (input) { case 0 { fallthrough; } } }`, placement},
		{"final default", `function bad(input: int): void { switch (input) { case 0 {} default { fallthrough; } } }`, placement},
		{"two fallthrough statements", `function bad(input: int): void { switch (input) { case 0 { fallthrough; fallthrough; } default {} } }`, placement},
		{"before another statement", `function bad(input: int): void { switch (input) { case 0 { fallthrough; call(); } default {} } }`, placement},
		{"nested block", `function bad(input: int): void { switch (input) { case 0 { { fallthrough; } } default {} } }`, placement},
		{"nested if", `function bad(input: int): void { switch (input) { case 0 { if (input > 0) { fallthrough; } } default {} } }`, placement},
		{"labeled", `function bad(input: int): void { switch (input) { case 0 { next: fallthrough; } default {} } }`, placement},
		{"type switch", `function bad(input: error): void { switch (input) { case const value as error { fallthrough; } default {} } }`, placement},
		{"select", `function bad(): void { select { default { fallthrough; } } }`, placement},
		{"constructor direct case still uninitialized", `class Item {} class Holder { private item: Item; constructor(input: int) { switch (input) { case 0 { this.item = new Item(); fallthrough; } case 1 {} default { this.item = new Item(); } } } }`, `non-null field "item"`},
		{"direct next case does not inherit nullable fact", `class User { constructor(public name: string) {} } function bad(user: User | null, input: int): string { switch (input) { case 0 { if (user === null) { return ""; } fallthrough; } case 1 { return user.name; } default { return ""; } } }`, `must be checked against null`},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, test.source), "\n")
			if !strings.Contains(diagnostics, test.want) {
				t.Fatalf("diagnostics=%q want=%q", diagnostics, test.want)
			}
		})
	}
}
