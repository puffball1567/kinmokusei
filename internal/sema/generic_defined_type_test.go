package sema

import (
	"strings"
	"testing"
)

func TestGenericDefinedTypeSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"slice conversion and operations", `type Values<T> = distinct T[]; function use(values: int[]): int { let typed = Values<int>(values); typed = append(typed, 3); typed[0] = 4; return len(typed) + typed[0]; }`},
		{"map conversion and mutation", `type Lookup<K, V> = distinct Map<K, V>; function use(values: Map<string, int>): int { const typed = Lookup<string, int>(values); typed["x"] = 2; return typed["x"]; }`},
		{"fixed array comparison and map key", `type Pair<T> = distinct [2]T; function use(left: Pair<int>, right: Pair<int>, values: Map<Pair<int>, string>): boolean { const text = values[left]; return left === right && text === "ok"; }`},
		{"pointer underlying", `type Ref<T> = distinct *T; function use(value: *int): Ref<int> { return Ref<int>(value); }`},
		{"nested defined type", `type Values<T> = distinct T[]; type Rows<T> = distinct Values<T>[]; function use(values: Values<int>[]): Rows<int> { return Rows<int>(values); }`},
		{"generic function", `type Values<T> = distinct T[]; function wrap<T>(values: T[]): Values<T> { return Values<T>(values); } function use(values: string[]): Values<string> { return wrap(values); }`},
		{"defined type argument", `type UserID = distinct string; type Values<T> = distinct T[]; function use(values: UserID[]): Values<UserID> { return Values<UserID>(values); }`},
		{"result", `type Values<T> = distinct T[]; function present(values: int[]): Result<Values<int>> { return ok(Values<int>(values)); }`},
		{"multiple instantiations", `type Values<T> = distinct T[]; function use(numbers: int[], words: string[]): int { return len(Values<int>(numbers)) + len(Values<string>(words)); }`},
		{"unnamed underlying assignment follows Go", `type Values<T> = distinct T[]; function use(values: int[]): Values<int> { return values; }`},
		{"recursive generic slice", `type Values<T> = distinct Values<T>[]; function use(values: Values<int>): int { return len(values); }`},
		{"recursive generic pointer", `type Link<T> = distinct *Link<T>; function use(value: Link<string>): Link<string> { return value; }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestGenericDefinedTypeFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing argument", `type Values<T> = distinct T[]; function bad(value: Values): void {}`, "generic defined type Values expects 1 type arguments, got 0"},
		{"too many arguments", `type Values<T> = distinct T[]; function bad(value: Values<int, string>): void {}`, "generic defined type Values expects 1 type arguments, got 2"},
		{"non generic arguments", `type Values = distinct int[]; function bad(value: Values<int>): void {}`, "defined type Values is not generic"},
		{"nominal instantiations", `type Values<T> = distinct T[]; function bad(value: Values<int>): Values<string> { return value; }`, "cannot use Values<int> as Values<string>"},
		{"different nominal types", `type Values<T> = distinct T[]; type Other<T> = distinct T[]; function bad(value: Other<int>): Values<int> { return value; }`, "cannot use Other<int> as Values<int>"},
		{"wrong conversion", `type Values<T> = distinct T[]; function bad(value: string): Values<int> { return Values<int>(value); }`, "cannot convert string to Values<int>"},
		{"duplicate parameter", `type Lookup<T, T> = distinct Map<T, int>;`, "duplicate generic defined type type parameter"},
		{"builtin parameter", `type Values<string> = distinct string[];`, "conflicts with a built-in type"},
		{"direct parameter underlying", `type Identity<T> = distinct T;`, "cannot use a type parameter directly as its underlying type"},
		{"generic alias", `alias Values<T> = T[];`, "generic aliases are not supported by the minimum Go target"},
		{"void argument", `type Values<T> = distinct T[]; function bad(value: Values<void>): void {}`, "void cannot be used as a generic defined type argument"},
		{"Result argument", `type Values<T> = distinct T[]; function bad(): Result<Values<Result<int>>> { return fail(nil); }`, "Result<int> cannot be used as a generic defined type argument"},
		{"inferred comparable constraint", `type Lookup<K, V> = distinct Map<K, V>; function bad(value: Lookup<int[], int>): void {}`, "does not satisfy comparable"},
		{"direct cycle", `type Values<T> = distinct Values<T>;`, "type declaration cycle"},
		{"fixed array cycle", `type Values<T> = distinct [1]Values<T>;`, "type declaration cycle"},
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
