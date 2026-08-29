package sema

import (
	"strings"
	"testing"
)

func TestNativeStructSemanticSuccessMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"literal fields and access", `struct Point { public x: int; private label: string; } function make(): Point { const point: Point = Point { x: 1, label: "a" }; return point; } function x(point: Point): int { return point.x; }`},
		{"nominal assignment copy", `struct Point { public x: int; } function copy(value: Point): Point { let duplicate: Point = value; duplicate.x = 9; return duplicate; }`},
		{"explicit pointer sharing", `struct Point { public x: int; } function mutate(value: *Point): int { (*value).x = (*value).x + 1; return (*value).x; } function use(): int { let point = Point { x: 1 }; const pointer: *Point = &point; return mutate(pointer); }`},
		{"comparable and map key", `class Token {} struct Key { public id: int; public label: string; public token: Token; } function lookup(values: Map<Key, int>, key: Key): boolean { return key === key && values[key] >= 0; }`},
		{"forward and nested values", `function first(value: Later): int { return value.point.x; } struct Later { public point: Point; } struct Point { public x: int; }`},
		{"pointer recursive shape", `struct Node { public value: int; public next: *Node; } function end(value: int): Node { return Node { value: value, next: nil }; }`},
		{"slice recursive shape", `struct Tree { public value: int; public children: Tree[]; } function leaf(): Tree { return Tree { value: 1, children: [] }; }`},
		{"fixed array of values", `struct Point { public x: int; } function pair(): [2]Point { return [Point { x: 1 }, Point { x: 2 }]; }`},
		{"value and pointer receiver calls", `struct Counter { public value: int; public function copied(delta: int): int { this.value += delta; return this.value; } public pointer function add(delta: int): void { this.value += delta; } public function read(): int { return this.value; } } function use(): int { let value = Counter { value: 1 }; const copied = value.copied(2); value.add(3); const pointer = &value; pointer.add(4); return copied + pointer.read() + pointer.value; }`},
		{"method values and nil pointer", `struct Node { public value: int; public pointer function isNil(): boolean { return this === nil; } public function read(): int { return this.value; } } function use(): boolean { let value = Node { value: 4 }; const read = value.read; const add = value.read; let pointer: *Node = nil; return read() === add() && pointer.isNil(); }`},
		{"pointer field remains an identifier", `struct Links { public pointer: *Links; public function end(): boolean { return this.pointer === nil; } } function use(): boolean { return Links { pointer: nil }.end(); }`},
		{"external value and pointer receivers", `struct Counter { public value: int; } public function copied(this: Counter, delta: int): Counter { this.value += delta; return this; } public function add(this: *Counter, delta: int): void { this.value += delta; } function use(): [2]int { let value = Counter { value: 1 }; const copy = value.copied(2); value.add(4); return [copy.value, value.value]; }`},
		{"external receiver before struct", `public function read(this: Counter): int { return this.value; } struct Counter { public value: int; } function use(value: Counter): int { return value.read(); }`},
		{"internal and external methods coexist", `struct Counter { public value: int; public function read(): int { return this.value; } } public function add(this: *Counter, delta: int): void { this.value += delta; } function use(value: *Counter): int { value.add(1); return value.read(); }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestNativeStructSemanticFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"duplicate field", `struct Point { public x: int; public x: int; }`, `duplicate struct field "x"`},
		{"missing literal field", `struct Point { public x: int; public y: int; } function make(): Point { return Point { x: 1 }; }`, `is missing field "y"`},
		{"unknown literal field", `struct Point { public x: int; } function make(): Point { return Point { x: 1, y: 2 }; }`, `has no field "y"`},
		{"duplicate literal field", `struct Point { public x: int; } function make(): Point { return Point { x: 1, x: 2 }; }`, `duplicate struct field "x"`},
		{"wrong literal type", `struct Point { public x: int; } function make(): Point { return Point { x: "bad" }; }`, `cannot use string as int`},
		{"unknown member", `struct Point { public x: int; } function read(value: Point): int { return value.y; }`, `has no field "y"`},
		{"nominal mismatch", `struct Left { public x: int; } struct Right { public x: int; } function bad(value: Left): Right { return value; }`, `cannot use Left as Right`},
		{"new requires class", `struct Point { public x: int; } function bad(): Point { return new Point(1); }`, `unknown class "Point"`},
		{"non-nullable value", `struct Point { public x: int; } function bad(): Point | null { return null; }`, `cannot be nullable`},
		{"slice field not comparable", `struct Bucket { public values: int[]; } function bad(value: Bucket): boolean { return value === value; }`, `is not comparable`},
		{"slice field map key", `struct Bucket { public values: int[]; } function bad(value: Map<Bucket, int>): void {}`, `cannot be used as a Map key`},
		{"direct recursion", `struct Node { public next: Node; }`, `recursively contains itself by value`},
		{"mutual recursion", `struct Left { public right: Right; } struct Right { public left: Left; }`, `recursively contains itself by value`},
		{"fixed array recursion", `struct Node { public children: [1]Node; }`, `recursively contains itself by value`},
		{"void field", `struct Bad { public value: void; }`, `cannot have type void`},
		{"result field", `struct Bad { public value: Result<int>; }`, `not for struct fields`},
		{"duplicate method", `struct Bad { function value(): int { return 1; } function value(): int { return 2; } }`, `duplicate struct method "value"`},
		{"field method conflict", `struct Bad { public value: int; function value(): int { return this.value; } }`, `conflicts with a field`},
		{"pointer method temporary", `struct Point { public x: int; pointer function move(): void { this.x++; } } function bad(): void { Point { x: 1 }.move(); }`, `requires an addressable Point value`},
		{"unknown method", `struct Point { public x: int; } function bad(value: Point): void { value.missing(); }`, `has no field "missing" or method`},
		{"method missing return", `struct Point { function bad(flag: boolean): int { if (flag) { return 1; } } }`, `may complete without returning int`},
		{"generated field method collision", `struct Bad { public value: int; private function Value(): int { return 1; } }`, `generated Go struct member name "Value" collides`},
		{"generated predeclared collision", `struct Bad { private len: int; private function len_(): int { return 1; } }`, `generated Go struct member name "len_" collides`},
		{"internal external duplicate", `struct Bad { function value(): int { return 1; } } function value(this: Bad): int { return 2; }`, `duplicate struct method "value"`},
		{"external class receiver", `class Bad {} function value(this: Bad): int { return 1; }`, `is not a native struct`},
		{"external double pointer receiver", `struct Bad { public value: int; } function value(this: **Bad): int { return 1; }`, `must be a native struct value or pointer`},
		{"external nullable receiver", `struct Bad { public value: int; } function value(this: *Bad | null): int { return 1; }`, `must be a native struct value or pointer`},
		{"external pointer method temporary", `struct Point { public x: int; } function move(this: *Point): void { this.x++; } function bad(): void { Point { x: 1 }.move(); }`, `requires an addressable Point value`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			matched := false
			for _, message := range diagnostics {
				matched = matched || strings.Contains(message, test.want)
			}
			if !matched {
				t.Fatalf("diagnostics = %v, want substring %q", diagnostics, test.want)
			}
		})
	}
}
