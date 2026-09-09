package sema

import (
	"fmt"
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

func TestSourceTypeSetConstraintSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"function and defined type", `constraint Integer = ~int | ~int8 | ~uint64; type Score = distinct int; function add<T extends Integer>(left: T, right: T): T { return left + right; } function use(): Score { return add(Score(1), Score(2)); }`},
		{"forward class constraint", `class Box<T extends Integer> { constructor(public value: T) {} public function doubled(): T { return this.value + this.value; } } constraint Integer = ~int | ~int16; function use(): int { return new Box<int>(2).doubled(); }`},
		{"struct interface and defined map", `constraint Key = ~string | ~int; struct Entry<T extends Key> { public key: T; } interface Reader<T extends Key> { function read(): T; } type Lookup<T extends Key> = distinct Map<T, string>; function size(values: Lookup<int>): int { return len(values); }`},
		{"exact type excludes aliases but accepts exact", `constraint OnlyInt = int; function identity<T extends OnlyInt>(value: T): T { return value; } function use(): int { return identity(3); }`},
		{"exact forward nominal type", `constraint TicketOnly = Ticket; type Ticket = distinct string; function identity<T extends TicketOnly>(value: T): T { return value; } function use(value: Ticket): Ticket { return identity(value); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestSourceTypeSetConstraintFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"unsatisfied argument", `constraint Integer = ~int | ~int8; function identity<T extends Integer>(value: T): T { return value; } function bad(): boolean { return identity(true); }`, "does not satisfy T type parameter constraint"},
		{"exact rejects defined type", `constraint OnlyInt = int; type Score = distinct int; function identity<T extends OnlyInt>(value: T): T { return value; } function bad(value: Score): Score { return identity(value); }`, "does not satisfy T type parameter constraint"},
		{"runtime value type", `constraint Integer = ~int | ~int8; function bad(value: Integer): void {}`, "can only be used after 'extends'"},
		{"implements constraint", `constraint Integer = ~int | ~int8; class Bad implements Integer {}`, "can only be used after 'extends'"},
		{"overlap exact and underlying", `constraint Bad = ~int | int;`, "overlaps an earlier term"},
		{"duplicate exact", `constraint Bad = string | string;`, "overlaps an earlier term"},
		{"tilde named type", `type Score = distinct int; constraint Bad = ~Score;`, "must name its own underlying type"},
		{"interface term", `interface Reader { function read(): int; } constraint Bad = Reader;`, "must be a concrete type, not an interface"},
		{"too many terms", "constraint Bad = " + fixedArrayConstraintTerms(101) + ";", "cannot contain more than 100 terms"},
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

func fixedArrayConstraintTerms(count int) string {
	terms := make([]string, count)
	for index := range terms {
		terms[index] = fmt.Sprintf("[%d]int", index)
	}
	return strings.Join(terms, " | ")
}
