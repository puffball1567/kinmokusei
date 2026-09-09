package sema

import (
	"strings"
	"testing"
)

func TestGenericMethodSemanticSuccessMatrix(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{
			"class inferred explicit and owner parameters",
			`class Box<T> { constructor(public value: T) {} public function keep<U>(marker: U): T { return this.value; } public function echo<U>(value: U): U { return value; } } function use(): string { const box = new Box<string>("keika"); const first = box.keep(1); const second = box.echo<string>(first); return second; }`,
		},
		{
			"class partial explicit parameters",
			`class Utility { public function second<T, U>(left: T, right: U): U { return right; } } function use(value: Utility): string { return value.second<int>(1, "ok"); }`,
		},
		{
			"generic static method",
			`class Box<T> { constructor(public value: T) {} public static function pair<U>(left: U, right: T): {left: U, right: T} { return {left: left, right: right}; } } function use(): string { return Box.pair<int>(1, "ok").right; }`,
		},
		{
			"struct value and pointer methods",
			`struct Holder<T> { public value: T; public function echo<U>(value: U): U { return value; } public pointer function update<U>(value: T, marker: U): U { this.value = value; return marker; } } function use(holder: Holder<string>): string { const marker = holder.update<int>("changed", 1); return holder.echo<string>(holder.value); }`,
		},
		{
			"source constraint",
			`constraint Integer = ~int | ~int8; class Math { public function add<T extends Integer>(left: T, right: T): T { return left + right; } } function use(math: Math): int8 { return math.add<int8>(int8(1), int8(2)); }`,
		},
		{
			"inherited and super calls",
			`class Base<T> { constructor(protected value: T) {} public function echo<U>(value: U): U { return value; } } class Child<V> extends Base<V> { constructor(value: V) { super(value); } public function relay<U>(value: U): U { return super.echo(value); } } function use(): string { const child = new Child<int>(1); return child.relay(child.echo("ok")); }`,
		},
		{
			"variadic and Result propagation",
			`import go errors from "errors"; class Box { public function first<T>(...values: T[]): Result<T> { if (len(values) === 0) { return fail(errors.New("empty")); } return ok(values[0]); } } function use(box: Box, values: string[]): Result<string> { const explicit = box.first<string>(values...)?; return box.first(explicit); }`,
		},
		{
			"expression receivers",
			`class Box { public function echo<T>(value: T): T { return value; } } function makeBox(): Box { return new Box(); } function use(boxes: Box[]): string { const direct = new Box().echo<string>("direct"); const returned = makeBox().echo(direct); return boxes[0].echo<string>(returned); }`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestGenericMethodSemanticFailureMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"class parameter collision", `class Box<T> { public function echo<T>(value: T): T { return value; } }`, `conflicts with a class type parameter`},
		{"struct parameter collision", `struct Box<T> { public function echo<T>(value: T): T { return value; } }`, `conflicts with a struct type parameter`},
		{"constraint mismatch", `constraint Integer = ~int; class Math { public function echo<T extends Integer>(value: T): T { return value; } } function use(math: Math): string { return math.echo("bad"); }`, `does not satisfy T type parameter constraint`},
		{"inconsistent inference", `class Pair { public function choose<T>(left: T, right: T): T { return left; } } function use(pair: Pair): int { return pair.choose(1, "bad"); }`, `cannot use integer literal as string`},
		{"too many arguments", `class Box { public function echo<T>(value: T): T { return value; } } function use(box: Box): int { return box.echo<int, string>(1); }`, `has 1 type parameters, got 2 explicit type arguments`},
		{"virtual", `class Box { public virtual function echo<T>(value: T): T { return value; } }`, `generic methods cannot be virtual`},
		{"instance method value", `class Box { public function echo<T>(value: T): T { return value; } } function use(box: Box): void { const callback = box.echo; }`, `generic methods must be called directly`},
		{"static method value", `class Box { public static function echo<T>(value: T): T { return value; } } function use(): void { const callback = Box.echo; }`, `generic methods must be called directly`},
		{"generated helper collision", `class Box { public function echo<T>(value: T): T { return value; } } function BoxEcho<T>(box: Box, value: T): T { return value; }`, `generated Go name "BoxEcho" collides`},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
