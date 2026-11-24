package sema

import (
	"strings"
	"testing"
)

func TestRegexAndMatchDiagnostics(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, input, diagnostic string
	}{
		{"invalid regex", `const bad = re"["`, "invalid regular expression"},
		{"backreferences", `const bad = re"(a)\1"`, "invalid regular expression"},
		{"binary text", `const found = re"x".test(b"x")`, "cannot use bstring as string"},
		{"opaque regex", `const equal = re"a" == re"b"`, "not comparable"},
		{"guarded wildcard", `function f(x: int): int { return match(x) { case _ if (x > 0) => 1 } }`, "non-exhaustive match"},
		{"early wildcard", `function f(x: int): int { return match(x) { case _ => 1, default => 2 } }`, "unreachable match arm"},
		{"wrong result", `function f(x: boolean): int { return match(x) { case true => "text", case false => 0 } }`, "cannot use string as int"},
		{"pattern range", `function f(x: uint8): int { return match(x) { case 256 => 1, default => 0 } }`, "cannot be represented"},
		{"bad guard", `function f(x: int): int { return match(x) { case _ if (x) => 1, default => 0 } }`, "match guard must be boolean"},
		{"unbound later arm", `function f(x: int): int { return match(x) { case const n if (false) => n, default => n } }`, "undefined name"},
		{"duplicate binding", `function f(x: int[]): int { return match(x) { case [const n, const n] => n, default => 0 } }`, "duplicate local name"},
		{"wrong fixed length", `function f(x: [1]int): int { return match(x) { case [_, _] => 1, default => 0 } }`, "sequence pattern has 2 elements"},
		{"oversized fixed rest pattern", `function f(x: [1]int): int { return match(x) { case [_, ..._, _] => 1, default => 0 } }`, "needs at least 2 elements"},
		{"duplicate rest binding", `function f(x: int[]): int { return match(x) { case [const tail, ...const tail] => 1, default => 0 } }`, "duplicate local name"},
		{"mixed nullable element contracts", `alias Pointer = *int; alias Maybe = Pointer | null; type A = distinct Pointer[]; type B = distinct Maybe[]; constraint S = A | B; function f<T extends S>(v: T): Pointer[] { return match(v) { case [...const rest] => rest } }`, "identical element types and source contracts"},
		{"mixed text element contracts", `type A = distinct string[]; type B = distinct bstring[]; constraint S = A | B; function f<T extends S>(v: T): string[] { return match(v) { case [...const rest] => rest } }`, "identical element types and source contracts"},
		{"missing slice length", `function f(x: int[]): int { return match(x) { case [] => 0, case [_, ..._, _] => 1 } }`, "non-exhaustive match"},
		{"filtered slice rest", `function f(x: int[]): int { return match(x) { case [] => 0, case [1, ..._] => 1 } }`, "non-exhaustive match"},
		{"guarded slice rest", `function f(x: int[], allowed: boolean): int { return match(x) { case [] => 0, case [_, ..._] if (allowed) => 1 } }`, "non-exhaustive match"},
		{"exhaustive slice lengths", `function f(x: int[]): int { return match(x) { case [_, ..._] => 1, case [] => 0, default => 2 } }`, "unreachable match arm"},
		{"unknown record field", `struct Item { public id: int; } function f(x: Item): int { return match(x) { case { ids: _ } => 1, default => 0 } }`, "has no direct record pattern field"},
		{"repeated field", `function f(x: { id: int }): int { return match(x) { case { id: _, id: _ } => 1 } }`, "duplicate record pattern field"},
		{"raw nil pattern", `function f(x: int): int { return match(x) { case null => 0, default => 1 } }`, "null match pattern requires a nullable value"},
		{"wrong enum type", `enum State { On } enum Other { On } function f(x: State): int { return match(x) { case Other.On => 1, default => 0 } }`, "cannot use Other as State"},
		{"unknown enum member", `enum State { On } function f(x: State): int { return match(x) { case State.Off => 1, default => 0 } }`, `has no member "Off"`},
		{"runtime selector", `function f(x: int, item: { code: int }): int { return match(x) { case item.code => 1, default => 0 } }`, "qualified match pattern must name an enum member"},
		{"shadowed enum", `enum State { On } function f(x: State, State: { On: State }): int { return match(x) { case State.On => 1, default => 0 } }`, "qualified match pattern must name an enum member"},
		{"shadowed generated member", `enum State { On } function f(x: State, StateOn: State): int { return match(x) { case State.On => 1 } }`, `shadows generated enum member "StateOn"`},
		{"missing enum values", `enum State { On, Enabled = 0, Off, Unknown } function f(x: State): int { return match(x) { case State.Off => 0 } }`, "missing enum members: On, Unknown;"},
		{"guarded enum case", `enum State { On } function f(x: State, enabled: boolean): int { return match(x) { case State.On if (enabled) => 1 } }`, "missing enum members: On;"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := checkSource(t, tc.input)
			if !strings.Contains(strings.Join(diagnostics, "\n"), tc.diagnostic) {
				t.Fatalf("wanted diagnostic %q, got %v", tc.diagnostic, diagnostics)
			}
		})
	}
}
