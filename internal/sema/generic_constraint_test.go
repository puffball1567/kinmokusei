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

func TestGoTypeSetConstraintSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"ordered inference and operators", `import go cmp from "cmp"; function choose<T extends cmp.Ordered>(left: T, right: T): T { if (left < right) { return left + left; } return right + right; } function use(): string { return choose("on", "sen"); }`},
		{"defined underlying type", `import go cmp from "cmp"; type Score = distinct int; function maximum<T extends cmp.Ordered>(left: T, right: T): T { if (left > right) { return left; } return right; } function use(): Score { return maximum(Score(1), Score(2)); }`},
		{"struct class and interface", `import go cmp from "cmp"; struct Range<T extends cmp.Ordered> { public low: T; public high: T; public function contains(value: T): boolean { return value >= this.low && value <= this.high; } } interface Chooser<T extends cmp.Ordered> { function choose(left: T, right: T): T; } class NumberChooser implements Chooser<int> { public function choose(left: int, right: int): int { if (left < right) { return left; } return right; } }`},
		{"defined collection and map key", `import go cmp from "cmp"; type OrderedValues<T extends cmp.Ordered> = distinct T[]; type Lookup<T extends cmp.Ordered> = distinct Map<T, string>; function use(values: OrderedValues<int>, lookup: Lookup<string>): int { return len(values) + len(lookup); }`},
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
		{"non-interface constraint", `function bad<T extends string>(value: T): T { return value; }`, "must be a Go interface constraint"},
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

func TestGoTypeSetConstraintFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"bool inferred argument", `import go cmp from "cmp"; function choose<T extends cmp.Ordered>(left: T, right: T): T { if (left < right) { return left; } return right; } function bad(): boolean { return choose(true, false); }`, "does not satisfy T type parameter constraint"},
		{"slice explicit argument", `import go cmp from "cmp"; function choose<T extends cmp.Ordered>(left: T, right: T): T { return left; } function bad(value: int[]): int[] { return choose<int[]>(value, value); }`, "does not satisfy T type parameter constraint"},
		{"subtraction not common to ordered set", `import go cmp from "cmp"; function bad<T extends cmp.Ordered>(left: T, right: T): T { return left - right; }`, "operator - requires numeric operands"},
		{"remainder not common to ordered set", `import go cmp from "cmp"; function bad<T extends cmp.Ordered>(left: T, right: T): T { return left % right; }`, "operator % requires numeric operands"},
		{"named non-interface", `import go time from "time"; function bad<T extends time.Duration>(value: T): T { return value; }`, "must be a Go interface constraint"},
		{"nullable constraint", `function bad<T extends error | null>(value: T): T { return value; }`, "must be a Go interface constraint"},
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
