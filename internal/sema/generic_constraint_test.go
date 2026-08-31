package sema

import (
	"strings"
	"testing"
)

func TestComparableTypeParameterConstraintSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"function inferred and explicit", `function equal<T extends comparable>(left: T, right: T): boolean { return left === right; } function use(): boolean { return equal(1, 1) && equal<string>("a", "b"); }`},
		{"struct", `struct Key<T extends comparable> { public value: T; public lookup: Map<T, string>; } function use(value: Key<string>): string { return value.lookup[value.value]; }`},
		{"interface", `interface Matcher<T extends comparable> { function matches(value: T): boolean; } class TextMatcher implements Matcher<string> { public function matches(value: string): boolean { return value === "ok"; } }`},
		{"defined type", `type Lookup<T extends comparable> = distinct Map<T, string>; function use(value: Lookup<int>): int { return len(value); }`},
		{"pointer remains comparable", `function equal<T extends comparable>(left: T, right: T): boolean { return left === right; } function use(left: *int, right: *int): boolean { return equal(left, right); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestComparableTypeParameterConstraintFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"unsupported constraint", `function bad<T extends string>(value: T): T { return value; }`, "constraint must be comparable"},
		{"inferred slice", `function equal<T extends comparable>(left: T, right: T): boolean { return left === right; } function bad(value: int[]): boolean { return equal(value, value); }`, "does not satisfy T type parameter constraint"},
		{"explicit map", `function identity<T extends comparable>(value: T): T { return value; } function bad(value: Map<string, int>): Map<string, int> { return identity<Map<string, int>>(value); }`, "does not satisfy T type parameter constraint"},
		{"struct slice", `struct Key<T extends comparable> { public value: T; } function bad(value: Key<int[]>): void {}`, "does not satisfy T type parameter constraint"},
		{"interface function", `interface Matcher<T extends comparable> { function matches(value: T): boolean; } function bad(value: Matcher<(value: int) => int>): void {}`, "does not satisfy T type parameter constraint"},
		{"defined map", `type Values<T extends comparable> = distinct T[]; function bad(value: Values<int[]>): void {}`, "cannot instantiate generic defined type Values"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
