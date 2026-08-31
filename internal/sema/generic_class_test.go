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
		{"generic extends", `class Base {} class Child<T> extends Base {}`, "generic classes cannot currently use extends"},
		{"generic base", `class Base<T> {} class Child extends Base<int> {}`, "generic class inheritance is not yet supported"},
		{"static method", `class Box<T> { public static function make(value: T): Box<T> { return new Box<T>(); } }`, "generic class static methods are not yet supported"},
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
