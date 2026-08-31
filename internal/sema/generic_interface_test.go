package sema

import (
	"strings"
	"testing"
)

func TestGenericInterfaceSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"single parameter dispatch", `interface Source<T> { function get(): T; } class IntSource implements Source<int> { public function get(): int { return 1; } } function use(value: Source<int>): int { return value.get(); }`},
		{"two parameters", `interface Transformer<T, U> { function transform(value: T): U; } class Length implements Transformer<string, int> { public function transform(value: string): int { return len(value); } } function use(value: Transformer<string, int>): int { return value.transform("onsen"); }`},
		{"nested positions", `interface Store<T> { function values(): T[]; function lookup(): Map<string, T>; function callback(): (value: T) => T; }`},
		{"generic function composition", `interface Identity<T> { function apply(value: T): T; } function use<T>(identity: Identity<T>, value: T): T { return identity.apply(value); }`},
		{"generic struct argument", `struct Box<T> { public value: T; } interface Reader<T> { function read(value: T): T; } class BoxReader implements Reader<Box<int>> { public function read(value: Box<int>): Box<int> { return value; } }`},
		{"recursive interface reference", `interface Chain<T> { function next(): Chain<T>; function value(): T; }`},
		{"multiple instantiations", `interface Reader<T> { function read(): T; } class IntReader implements Reader<int> { public function read(): int { return 1; } } class StringReader implements Reader<string> { public function read(): string { return "x"; } } function readNumber(value: Reader<int>): int { return value.read(); } function readText(value: Reader<string>): string { return value.read(); }`},
		{"class implements two instantiations", `interface Marker<T> { function kind(): string; } class Both implements Marker<int>, Marker<string> { public function kind(): string { return "both"; } }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestGenericInterfaceFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing argument", `interface Source<T> { function get(): T; } function bad(value: Source): void {}`, "generic interface Source expects 1 type arguments, got 0"},
		{"too many arguments", `interface Source<T> { function get(): T; } function bad(value: Source<int, string>): void {}`, "generic interface Source expects 1 type arguments, got 2"},
		{"non generic arguments", `interface Source { function get(): int; } function bad(value: Source<int>): void {}`, "interface Source is not generic"},
		{"duplicate parameter", `interface Pair<T, T> { function first(): T; }`, "duplicate generic interface type parameter"},
		{"builtin parameter", `interface Box<string> { function get(): string; }`, "conflicts with a built-in type"},
		{"void argument", `interface Source<T> { function get(): T; } function bad(value: Source<void>): void {}`, "void cannot be used as a generic interface type argument"},
		{"implementation parameter mismatch", `interface Sink<T> { function put(value: T): void; } class Bad implements Sink<int> { public function put(value: string): void {} }`, "method put has an incompatible signature"},
		{"implementation result mismatch", `interface Source<T> { function get(): T; } class Bad implements Source<int> { public function get(): string { return "x"; } }`, "method get has an incompatible signature"},
		{"assignment mismatch", `interface Source<T> { function get(): T; } class IntSource implements Source<int> { public function get(): int { return 1; } } function bad(value: Source<int>): Source<string> { return value; }`, "cannot use Source<int> as Source<string>"},
		{"class instantiation mismatch", `interface Source<T> { function get(): T; } class IntSource implements Source<int> { public function get(): int { return 1; } } function bad(value: IntSource): Source<string> { return value; }`, "cannot use IntSource as Source<string>"},
		{"duplicate implementation", `interface Source<T> { function get(): T; } class Bad implements Source<int>, Source<int> { public function get(): int { return 1; } }`, `duplicate implemented interface "Source<int>"`},
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
