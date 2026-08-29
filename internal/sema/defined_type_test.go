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
		{"void alias", `alias Nothing = void;`, "cannot be used as the underlying type"},
		{"Result alias", `alias Outcome = Result<int>;`, "cannot be used as the underlying type"},
		{"distinct class", `class Box {} type OtherBox = distinct Box;`, "cannot yet be used as a distinct defined type"},
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
