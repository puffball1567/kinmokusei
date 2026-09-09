package sema

import (
	"strings"
	"testing"
)

func TestIntegerRangeSemanticMatrix(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"literal index defaults to int", `function use(): int { let total = 0; for (const i of 5) { total += i; } return total; }`, ""},
		{"named bound preserves type", `type Count = distinct int; function use(n: Count): Count { let total = Count(0); for (const i: Count of n) { total += i; } return total; }`, ""},
		{"uint8", `function use(n: uint8): uint8 { let total = uint8(0); for (let i of n) { total += i; i = 0; } return total; }`, ""},
		{"integer type parameter", `constraint Count = ~int; function use<T extends Count>(n: T): T { let total: T = 0; for (const i of n) { total += i; } return total; }`, ""},
		{"mixed integer constraint", `constraint Count = ~int | ~int64; function use<T extends Count>(n: T): void { for (const i of n) {} }`, "integer range type parameter requires a single underlying integer type"},
		{"constructor positive constant", `class Leaf {} class Value { public value: Leaf; constructor() { for (const _ of 1 + 1) { this.value = new Leaf(); } } }`, ""},
		{"constructor zero", `class Leaf {} class Value { public value: Leaf; constructor() { for (const _ of 0) { this.value = new Leaf(); } } }`, "must be initialized"},
		{"pair", `function use(): void { for (const [i, j] of 3) {} }`, "integer range requires exactly one binding"},
		{"float", `function use(n: float64): void { for (const i of n) {} }`, "range requires an integer"},
		{"annotation cannot change generated type", `function use(): void { for (const i: int64 of 3) {} }`, "cannot use int as int64"},
		{"overflow", `function use(): void { for (const _ of 999999999999999999999999999999999999999999) {} }`, "integer range bound overflows int"},
		{"constant assignment", `function use(): void { for (const i of 3) { i = 0; } }`, "cannot assign to const"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
