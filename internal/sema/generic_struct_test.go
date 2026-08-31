package sema

import (
	"strings"
	"testing"
)

func TestGenericStructSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"literal and field substitution", `struct Box<T> { public value: T; } function use(): string { const box: Box<string> = Box<string> { value: "onsen" }; return box.value; }`},
		{"two parameters", `struct Pair<T, U> { public first: T; public second: U; } function use(pair: Pair<string, int>): int { return pair.second; }`},
		{"value method", `struct Box<T> { public value: T; public function get(): T { return this.value; } } function use(box: Box<string>): string { return box.get(); }`},
		{"pointer method", `struct Box<T> { public value: T; public pointer function set(value: T): void { this.value = value; } } function use(box: *Box<string>): void { box.set("new"); }`},
		{"nested generic", `struct Box<T> { public value: T; } struct Pair<T> { public left: Box<T>; public right: Box<T>; } function use(value: Pair<int>): int { return value.left.value + value.right.value; }`},
		{"recursive pointer", `struct Node<T> { public value: T; public next: *Node<T>; }`},
		{"generic function composition", `struct Box<T> { public value: T; } function identity<T>(value: T): T { return value; } function use(value: Box<int>): Box<int> { return identity(value); }`},
		{"class argument", `class User { public name: string; constructor(name: string) { this.name = name; } } struct Box<T> { public value: T; } function use(user: User): User { const box: Box<User> = Box<User> { value: user }; return box.value; }`},
		{"defined type argument", `type UserID = distinct string; struct Box<T> { public value: T; } function use(value: UserID): UserID { const box: Box<UserID> = Box<UserID> { value: value }; return box.value; }`},
		{"collection fields", `struct Store<T> { public values: T[]; public lookup: Map<string, T>; } function use(store: Store<int>): int { return store.values[0] + store.lookup["x"]; }`},
		{"pointer to parameter", `struct Ref<T> { public value: *T; } function use(value: *int): *int { const ref: Ref<int> = Ref<int> { value: value }; return ref.value; }`},
		{"fixed array argument", `struct Box<T> { public value: T; } function use(value: Box<[2]int>): int { return value.value[0]; }`},
		{"interface argument", `interface Reader { function read(): string; } class Item implements Reader { public function read(): string { return "ok"; } } struct Box<T> { public value: T; } function use(value: Box<Reader>): string { return value.value.read(); }`},
		{"external value receiver", `struct Box<T> { public value: T; } public function get<U>(this: Box<U>): U { return this.value; } function use(box: Box<string>): string { return box.get(); }`},
		{"external pointer receiver", `struct Box<T> { public value: T; } public function set<U>(this: *Box<U>, value: U): void { this.value = value; } function use(box: *Box<string>): void { box.set("new"); }`},
		{"external two parameter receiver", `struct Pair<T, U> { public first: T; public second: U; } function readSecond<A, B>(this: Pair<A, B>): B { return this.second; } function use(pair: Pair<string, int>): int { return pair.readSecond(); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestGenericStructFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing argument", `struct Box<T> { public value: T; } function bad(value: Box): void {}`, "generic struct Box expects 1 type arguments, got 0"},
		{"too many arguments", `struct Box<T> { public value: T; } function bad(value: Box<int, string>): void {}`, "generic struct Box expects 1 type arguments, got 2"},
		{"non generic arguments", `struct Point { public x: int; } function bad(value: Point<int>): void {}`, "struct Point is not generic"},
		{"nominal instantiations", `struct Box<T> { public value: T; } function bad(value: Box<int>): Box<string> { return value; }`, "cannot use Box<int> as Box<string>"},
		{"literal substitution", `struct Box<T> { public value: T; } function bad(): Box<int> { return Box<int> { value: "wrong" }; }`, "cannot use string as int"},
		{"method parameter substitution", `struct Box<T> { public pointer function set(value: T): void {} } function bad(box: *Box<int>): void { box.set("wrong"); }`, "cannot use string as int"},
		{"duplicate parameter", `struct Pair<T, T> { public value: T; }`, "duplicate generic struct type parameter"},
		{"builtin parameter", `struct Box<string> { public value: string; }`, "conflicts with a built-in type"},
		{"void argument", `struct Box<T> { public value: T; } function bad(value: Box<void>): void {}`, "void cannot be used as a generic struct type argument"},
		{"Result argument", `struct Box<T> { public value: T; } function bad(): Result<Box<Result<int>>> { return fail(nil); }`, "Result<int> cannot be used as a generic struct type argument"},
		{"direct recursion", `struct Node<T> { public next: Node<T>; }`, "recursively contains itself by value"},
		{"external receiver missing binders", `struct Box<T> { public value: T; } public function get(this: Box<T>): T { return this.value; }`, "requires 1 receiver type parameters, got 0"},
		{"external receiver concrete argument", `struct Box<T> { public value: T; } public function get<U>(this: Box<int>): int { return this.value; }`, "must be receiver type parameter U"},
		{"external receiver wrong binder", `struct Box<T> { public value: T; } public function get<U>(this: Box<T>): T { return this.value; }`, "must be receiver type parameter U"},
		{"external receiver wrong arity", `struct Pair<T, U> { public first: T; public second: U; } public function first<A>(this: Pair<A, A>): A { return this.first; }`, "requires 2 receiver type parameters, got 1"},
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
