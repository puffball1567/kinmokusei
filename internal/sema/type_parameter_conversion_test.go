package sema

import (
	"strings"
	"testing"
)

func TestTypeParameterConversionSemanticMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"numeric constant", `constraint Number = ~int | ~int8; function zero<T extends Number>(): T { return T(0); }`, ""},
		{"type parameter shadows builtin", `constraint Number = ~int; function use<len extends Number>(value: int): len { return len(value); }`, ""},
		{"nil slice", `constraint Bytes = ~byte[]; function use<T extends Bytes>(): T { return T(nil); }`, ""},
		{"numeric variable", `constraint Number = ~int8 | ~float64; function convert<T extends Number>(value: int): T { return T(value); }`, ""},
		{"between parameters", `constraint Number = ~int | ~float64; function convert<T extends Number, U extends Number>(value: U): T { return T(value); }`, ""},
		{"identity unconstrained", `function identity<T>(value: T): T { return T(value); }`, ""},
		{"typed readonly value is not a Go constant", `constraint Number = ~int8 | ~int64; function use<T extends Number>(): T { const value: T = 1; return T(value); }`, ""},
		{"generic class constructor and method", `constraint Number = ~int8 | ~int64; class Counter<T extends Number> { private value: T; constructor() { this.value = T(0); } public function add(value: int): T { this.value += T(value); return this.value; } }`, ""},
		{"string conversion", `constraint Text = ~string; function text<T extends Text>(value: int): T { return T(value); }`, ""},
		{"numeric or string conversion", `constraint Value = ~int8 | ~string; function use<T extends Value>(): T { return T(65); }`, ""},
		{"slice conversion", `constraint Bytes = ~byte[]; function use<T extends Bytes>(value: string): T { return T(value); }`, ""},
		{"function shadows type parameter", `function use<T>(): int { { const T = (value: int): int => value; return T(3); } }`, ""},
		{"nested block must return on every path", `function use(flag: boolean): int { { if (flag) { return 1; } } }`, "may complete without returning int"},
		{"narrow overflow", `constraint Number = ~int8 | ~int64; function use<T extends Number>(): T { return T(128); }`, "cannot be converted to every type in T's type set"},
		{"negative unsigned", `constraint Number = ~uint8 | ~int64; function use<T extends Number>(): T { return T(-1); }`, "cannot be converted to every type in T's type set"},
		{"mixed set incompatible", `constraint Mixed = ~int | ~boolean; function use<T extends Mixed>(value: int): T { return T(value); }`, "cannot convert int to T"},
		{"unconstrained unrelated", `function use<T>(value: int): T { return T(value); }`, "cannot convert int to T"},
		{"arity", `constraint Number = ~int; function use<T extends Number>(): T { return T(); }`, "conversion to T expects 1 argument, got 0"},
		{"type arguments", `constraint Number = ~int; function use<T extends Number>(): T { return T<int>(1); }`, "type parameter conversions do not accept type arguments"},
		{"spread", `constraint Number = ~int; function use<T extends Number>(values: int[]): T { return T(values...); }`, "spread arguments cannot be used in type conversions"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" && len(diagnostics) != 0 || test.want != "" && !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
