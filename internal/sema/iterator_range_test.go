package sema

import (
	"strings"
	"testing"
)

func TestIteratorRangeSemanticMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"single value", `function values(yield: (value: int) => boolean): void { yield(1); } function use(): int { let sum = 0; for (const value of values) { sum += value; } return sum; }`, ""},
		{"pair values", `function pairs(yield: (key: string, value: int) => boolean): void { yield("a", 1); } function use(): void { for (const [key, value] of pairs) { const size = len(key) + value; } for (const value of pairs) { const size = value + 1; } }`, ""},
		{"zero values", `function ticks(yield: () => boolean): void { yield(); } function use(): void { for (const _ of ticks) {} }`, ""},
		{"class values", `class Leaf { constructor(public value: int) {} } class Sequence<T> { constructor(private value: T) {} public function values(yield: (value: T) => boolean): void { yield(this.value); } } function use(): int { const sequence = new Sequence<Leaf>(new Leaf(3)); let sum = 0; for (const leaf of sequence.values) { sum += leaf.value; } return sum; }`, ""},
		{"standard library", `import go slices from "slices"; function use(values: int[]): int { let sum = 0; for (const value of slices.Values(values)) { sum += value; } return sum; }`, ""},
		{"outer parameters", `function bad(): void {} function use(): void { for (const _ of bad) {} }`, "taking one yield callback and returning void"},
		{"outer return", `function bad(yield: () => boolean): int { return 1; } function use(): void { for (const _ of bad) {} }`, "taking one yield callback and returning void"},
		{"not callback", `function bad(value: int): void {} function use(): void { for (const _ of bad) {} }`, "yield callback must take zero, one, or two values and return boolean"},
		{"callback return", `function bad(yield: (value: int) => int): void {} function use(): void { for (const _ of bad) {} }`, "yield callback must take zero, one, or two values and return boolean"},
		{"callback arity", `function bad(yield: (a: int, b: int, c: int) => boolean): void {} function use(): void { for (const _ of bad) {} }`, "yield callback must take zero, one, or two values and return boolean"},
		{"callback variadic", `function bad(yield: (...values: int[]) => boolean): void {} function use(): void { for (const _ of bad) {} }`, "yield callback must take zero, one, or two values and return boolean"},
		{"single pair binding", `function values(yield: (value: int) => boolean): void {} function use(): void { for (const [i, value] of values) {} }`, "single-value iterator range requires exactly one binding"},
		{"zero named binding", `function ticks(yield: () => boolean): void {} function use(): void { for (const value of ticks) {} }`, "zero-value iterator range requires a single untyped '_' binding"},
		{"zero annotated binding", `function ticks(yield: () => boolean): void {} function use(): void { for (const _: int of ticks) {} }`, "zero-value iterator range requires a single untyped '_' binding"},
		{"nullable iterator", `alias Iterator = (yield: (value: int) => boolean) => void; function use(values: Iterator | null): void { for (const value of values) {} }`, "nullable iterator must be narrowed before range"},
		{"member narrowing invalidated", `class Leaf { public value: int; } class Holder { public leaf: Leaf | null; } function use(holder: Holder, iterator: (yield: () => boolean) => void): int { if (holder.leaf !== null) { for (const _ of iterator) { return holder.leaf.value; } } return 0; }`, "nullable"},
		{"no constructor guarantee", `function values(yield: (value: int) => boolean): void { yield(1); } class Leaf {} class Holder { private leaf: Leaf; constructor() { for (const value of values) { this.leaf = new Leaf(); } } }`, "must be initialized"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
