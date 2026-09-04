package sema

import (
	"strings"
	"testing"
)

func TestDefinedTypeAndAliasSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"string conversion", `type UserID = distinct string; function use(value: string): UserID { return UserID(value); }`},
		{"reverse conversion", `type UserID = distinct string; function use(value: UserID): string { return string(value); }`},
		{"same defined string operation", `type UserID = distinct string; function use(left: UserID, right: UserID): UserID { return left + right; }`},
		{"numeric operation", `type Score = distinct int; function use(left: Score, right: Score): Score { return left + right; }`},
		{"ordering", `type Score = distinct int; function use(left: Score, right: Score): boolean { return left < right; }`},
		{"map key", `type UserID = distinct string; function use(values: Map<UserID, int>, key: UserID): int { return values[key]; }`},
		{"defined slice", `type Tags = distinct string[]; function use(values: string[]): int { const tags = Tags(values); return len(tags); }`},
		{"defined map", `type Scores = distinct Map<string, int>; function use(values: Map<string, int>): int { const scores = Scores(values); return scores["x"]; }`},
		{"defined fixed array", `type Pair = distinct [2]int; function use(values: [2]int): int { const pair = Pair(values); return pair[1]; }`},
		{"transparent alias", `alias UserID = string; function use(value: string): UserID { return value; }`},
		{"alias to defined", `type UserID = distinct string; alias PrimaryID = UserID; function use(value: UserID): PrimaryID { return value; }`},
		{"forward alias", `alias Identifier = UserID; type UserID = distinct string; function use(value: UserID): Identifier { return value; }`},
		{"untyped constant", `type Score = distinct int; function use(): Score { return 1; }`},
		{"alias to class reference", `class Box {} alias BoxRef = Box; function use(value: Box): BoxRef { return value; }`},
		{"contextual declaration words", `function use(type: int, alias: int, distinct: int): int { return type + alias + distinct; }`},
		{"recursive pointer", `type Link = distinct *Link; function use(value: Link): Link { return value; }`},
		{"recursive slice", `type Chain = distinct Chain[]; function use(value: Chain): int { return len(value); }`},
		{"recursive map value", `type Tree = distinct Map<string, Tree>; function use(value: Tree): int { return len(value); }`},
		{"recursive function", `type Visitor = distinct (next: Visitor) => void;`},
		{"recursive channel", `type Stream = distinct GoChannel<Stream>;`},
		{"mutual recursion through pointer", `type Left = distinct Right; type Right = distinct *Left;`},
		{"distinct native struct conversions and fields", `struct Point { public x: int; public y: int; } type Offset = distinct Point; function use(point: Point): int { const offset = Offset(point); const restored = Point(offset); return offset.x + restored.y; }`},
		{"distinct native struct literal", `struct Point { public x: int; public y: int; } type Offset = distinct Point; function use(): Offset { return Offset { x: 1, y: 2 }; }`},
		{"generic distinct native struct", `struct Box<T> { public value: T; } type NamedBox<T> = distinct Box<T>; public function get<U>(this: NamedBox<U>): U { return this.value; } function use(value: string): string { const box = NamedBox<string> { value: value }; return box.get(); }`},
		{"distinct struct with nested native values", `class User { public name: string; constructor(name: string) { this.name = name; } } struct Entry { public owner: User; public tags: string[]; } type NamedEntry = distinct Entry; function use(value: Entry): string { const entry = NamedEntry(value); return entry.owner.name; }`},
		{"conversion from structurally identical native struct", `struct Point { public x: int; } struct Coordinate { public x: int; } type Offset = distinct Point; function use(value: Coordinate): int { return Offset(value).x; }`},
		{"constrained generic distinct native struct", `struct KeyBox<T extends comparable> { public key: T; } type NamedKeyBox<T extends comparable> = distinct KeyBox<T>; function use(value: NamedKeyBox<string>): string { return value.key; }`},
		{"nested distinct native struct", `struct Point { public x: int; } type Offset = distinct Point; type TaggedOffset = distinct Offset; function use(value: Offset): int { return TaggedOffset(value).x; }`},
		{"distinct struct with composite storage", `struct Point { public x: int; } struct Wrapper { public callback: (value: Point) => Point; public payload: { point: Point }; public stream: GoChannel<Point>; } type NamedWrapper = distinct Wrapper; function use(value: Wrapper): NamedWrapper { return NamedWrapper(value); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestDefinedTypeAndAliasFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"implicit underlying assignment", `type UserID = distinct string; function bad(value: string): UserID { return value; }`, "cannot use string as UserID"},
		{"different nominal types", `type UserID = distinct string; type OrderID = distinct string; function bad(value: OrderID): UserID { return value; }`, "cannot use OrderID as UserID"},
		{"mixed arithmetic", `type Score = distinct int; function bad(value: Score, other: int): Score { return value + other; }`, "cannot mix Score and int"},
		{"wrong conversion", `type UserID = distinct string; function bad(value: boolean): UserID { return UserID(value); }`, "cannot convert boolean to UserID"},
		{"conversion arity", `type UserID = distinct string; function bad(): UserID { return UserID(); }`, "expects 1 argument"},
		{"type used as value", `type UserID = distinct string; function bad(): void { const value = UserID; }`, "cannot be used as a value"},
		{"direct cycle", `alias First = Second; alias Second = First;`, "type declaration cycle"},
		{"direct distinct cycle", `type Loop = distinct Loop;`, "type declaration cycle"},
		{"fixed array cycle", `type Loop = distinct [1]Loop;`, "type declaration cycle"},
		{"mutual infinite cycle", `type First = distinct Second; type Second = distinct First;`, "type declaration cycle"},
		{"recursive alias through pointer", `alias Loop = *Loop;`, "type declaration cycle"},
		{"void alias", `alias Nothing = void;`, "cannot be used as the underlying type"},
		{"Result alias", `alias Outcome = Result<int>;`, "cannot be used as the underlying type"},
		{"distinct class", `class Box {} type OtherBox = distinct Box;`, "cannot yet be used as a distinct defined type"},
		{"implicit native struct assignment", `struct Point { public x: int; } type Offset = distinct Point; function bad(value: Point): Offset { return value; }`, "cannot use Point as Offset"},
		{"native struct method is not inherited", `struct Point { public x: int; public function read(): int { return this.x; } } type Offset = distinct Point; function bad(value: Offset): int { return value.read(); }`, "has no field or method"},
		{"different native struct conversion", `struct Point { public x: int; } struct Label { public value: string; } type Offset = distinct Point; function bad(value: Label): Offset { return Offset(value); }`, "cannot convert Label to Offset"},
		{"native struct field method conflict", `struct Point { public value: int; } type NamedPoint = distinct Point; public function value(this: NamedPoint): int { return this.value; }`, "conflicts with underlying struct field"},
		{"generic native struct constraint", `struct KeyBox<T extends comparable> { public key: T; } type NamedKeyBox<T> = distinct KeyBox<T>;`, "does not satisfy"},
		{"builtin collision", `type string = distinct int;`, "conflicts with a built-in type"},
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
