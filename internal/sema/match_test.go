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
		{"unknown record field", `struct Item { public id: int; } function f(x: Item): int { return match(x) { case { ids: _ } => 1, default => 0 } }`, "has no direct record pattern field"},
		{"repeated field", `function f(x: { id: int }): int { return match(x) { case { id: _, id: _ } => 1 } }`, "duplicate record pattern field"},
		{"raw nil pattern", `function f(x: int): int { return match(x) { case null => 0, default => 1 } }`, "null match pattern requires a nullable value"},
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
