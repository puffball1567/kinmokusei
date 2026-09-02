package sema

import (
	"strings"
	"testing"
)

func TestGenericClassSemanticSuccessMatrix(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{
			"constructor fields and methods",
			`class Box<T> { constructor(public value: T) {} public function get(): T { return this.value; } public function set(value: T): void { this.value = value; } } function use(): string { const box: Box<string> = new Box<string>("first"); box.set("second"); return box.get(); }`,
		},
		{
			"multiple instantiations",
			`class Box<T> { constructor(public value: T) {} } function use(): int { const numeric: Box<int> = new Box<int>(41); const word: Box<string> = new Box<string>("x"); return numeric.value + len(word.value); }`,
		},
		{
			"comparable constraint and map",
			`class Lookup<K extends comparable, V> { constructor(public values: Map<K, V>) {} public function get(key: K): V { return this.values[key]; } } function use(values: Map<string, int>): int { return new Lookup<string, int>(values).get("answer"); }`,
		},
		{
			"generic interface implementation",
			`interface Reader<T> { function read(): T; } class Value<T> implements Reader<T> { constructor(private value: T) {} public function read(): T { return this.value; } } function use(value: Reader<string>): string { return value.read(); } function make(): Reader<string> { return new Value<string>("ok"); }`,
		},
		{
			"nested generic class reference",
			`class Node<T> { constructor(public value: T, public next: Node<T> | null) {} } function use(): Node<int> { return new Node<int>(2, new Node<int>(1, null)); }`,
		},
		{
			"generic function result and nested instantiation",
			`class Box<T> { constructor(public value: T) {} } function present<T>(value: T): Result<Box<T>> { return ok(new Box<T>(value)); } function nested(): Box<Box<int>> { return new Box<Box<int>>(new Box<int>(7)); }`,
		},
		{
			"class reference map key",
			`class Box<T> { constructor(public value: T) {} } function lookup(values: Map<Box<int>, string>, key: Box<int>): string { return values[key]; }`,
		},
		{
			"static inference and explicit arguments",
			`class Box<T> { constructor(public value: T) {} public static function make(value: T): Box<T> { return new Box<T>(value); } } function use(): int { const inferred = Box.make(40); const angle = Box.make<int>(1); const bracket = Box.make[int](1); return inferred.value + angle.value + bracket.value; }`,
		},
		{
			"constrained static method",
			`class Key<T extends comparable> { constructor(public value: T) {} public static function make(value: T): Key<T> { return new Key<T>(value); } } function use(): string { return Key.make("key").value; }`,
		},
		{
			"static result",
			`class Box<T> { constructor(public value: T) {} public static function present(value: T): Result<Box<T>> { return ok(new Box<T>(value)); } } function use(): Result<int> { const box = Box.present(42)?; return ok(box.value); }`,
		},
		{
			"zero argument static explicit type",
			`class Box<T> { public static function empty(): Box<T> { return new Box<T>(); } } function use(): Box<int> { return Box.empty<int>(); }`,
		},
		{
			"generic inheritance and super",
			`class Base<T> { constructor(protected value: T) {} public function get(): T { return this.value; } } class Child<T> extends Base<T> { constructor(value: T) { super(value); } public function read(): T { return this.value; } } function use(): string { return new Child<string>("ok").get(); }`,
		},
		{
			"concrete generic base",
			`class Base<T> { constructor(public value: T) {} } class IntegerBox extends Base<int> { constructor(value: int) { super(value); } } function use(): int { return new IntegerBox(42).value; }`,
		},
		{
			"remapped and multi level inheritance",
			`class Base<T> { constructor(public value: T) {} public function get(): T { return this.value; } } class Middle<U, V> extends Base<V> { constructor(value: V) { super(value); } } class Leaf<X> extends Middle<int, X> { constructor(value: X) { super(value); } } function use(): string { return new Leaf<string>("leaf").get(); }`,
		},
		{
			"generic inherited interface",
			`interface Reader<T> { function read(): T; } class Base<T> implements Reader<T> { constructor(public value: T) {} public function read(): T { return this.value; } } class Child<U> extends Base<U> { constructor(value: U) { super(value); } } function use(reader: Reader<string>): string { return reader.read(); } function make(): string { return use(new Child<string>("ok")); }`,
		},
		{
			"inherited comparable constraint",
			`class Base<T extends comparable> { constructor(public value: T) {} } class Child<U extends comparable> extends Base<U> { constructor(value: U) { super(value); } } function use(): string { return new Child<string>("ok").value; }`,
		},
		{
			"generic upcast and downcast",
			`class Base<T> { constructor(public value: T) {} } class Child<U> extends Base<U> { constructor(value: U) { super(value); } } function up(value: Child<string>): Base<string> { return value; } function down(value: Base<string>): Child<string> | null { const [child, present] = value as? Child<string>; if (!present) { return null; } return child; }`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestGenericClassSemanticFailureMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"missing type argument", `class Box<T> { constructor(public value: T) {} } function use(): void { new Box(1); }`, "expects 1 type arguments, got 0"},
		{"too many type arguments", `class Box<T> {} function use(): void { new Box<int, string>(); }`, "expects 1 type arguments, got 2"},
		{"nongeneric arguments", `class Box {} function use(): void { new Box<int>(); }`, "class Box is not generic"},
		{"constructor mismatch", `class Box<T> { constructor(public value: T) {} } function use(): void { new Box<int>("wrong"); }`, "cannot use string as int"},
		{"instantiation mismatch", `class Box<T> { constructor(public value: T) {} } function use(): Box<int> { return new Box<string>("wrong"); }`, "cannot use Box<string> as Box<int>"},
		{"constraint mismatch", `class Key<T extends comparable> { constructor(public value: T) {} } function use(items: int[]): void { new Key<int[]>(items); }`, "does not satisfy T type parameter constraint"},
		{"missing base arguments", `class Base<T> {} class Child extends Base {}`, "generic class Base expects 1 type arguments, got 0"},
		{"nongeneric base arguments", `class Base {} class Child extends Base<int> {}`, "class Base is not generic"},
		{"base constraint mismatch", `class Base<T extends comparable> {} class Child extends Base<int[]> {}`, "does not satisfy T type parameter constraint"},
		{"base type parameter constraint mismatch", `class Base<T extends comparable> {} class Child<U> extends Base<U> {}`, "does not satisfy T type parameter constraint"},
		{"incompatible generic upcast", `class Base<T> {} class Child<T> extends Base<T> {} function use(value: Child<int>): Base<string> { return value; }`, "cannot use Child<int> as Base<string>"},
		{"generic super mismatch", `class Base<T> { constructor(value: T) {} } class Child<U> extends Base<U> { constructor(value: U) { super(1); } }`, "cannot use integer literal as U"},
		{"uninferred static argument", `class Box<T> { public static function empty(): Box<T> { return new Box<T>(); } } function use(): void { Box.empty(); }`, "cannot infer type argument T"},
		{"static constraint mismatch", `class Key<T extends comparable> { public static function make(value: T): void {} } function use(values: int[]): void { Key.make(values); }`, "does not satisfy T type parameter constraint"},
		{"inconsistent static inference", `class Pair<T> { public static function make(left: T, right: T): void {} } function use(): void { Pair.make(1, "two"); }`, "T was already inferred as int, not string"},
		{"virtual method", `class Box<T> { constructor(private value: T) {} public virtual function read(): T { return this.value; } }`, "generic class virtual"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
