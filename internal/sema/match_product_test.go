package sema

import (
	"strings"
	"testing"
)

func TestProductMatchExhaustiveness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, diagnostic string
	}{
		{"record with omitted fields", `function f(x: { a: boolean, b: boolean, count: int }): int { return match(x) { case { a: true } => 1, case { b: true, a: false } => 2, case { a: false, b: false } => 3 } }`, ""},
		{"record with explicit default", `function f(x: { a: boolean }): int { return match(x) { case { a: true } => 1, case { a: false } => 0, default => -1 } }`, ""},
		{"nullable field", `function f(x: { value: *int | null }): int { return match(x) { case { value: null } => 0, case { value: const v! } => *v } }`, ""},
		{"named boolean", `type Flag = distinct boolean; function f(x: [1]Flag): int { return match(x) { case [true] => 1, case [false] => 0 } }`, ""},
		{"guarded arm redundant for coverage", `function f(x: [1]boolean, allow: boolean): int { return match(x) { case [true] if (allow) => 2, case [true] => 1, case [false] => 0 } }`, ""},
		{"slice field wildcard", `function f(x: { flag: boolean, values: int[] }): int { return match(x) { case { flag: true, values: [..._] } => 1, case { flag: false } => 0 } }`, ""},
		{"diagonal leaves gaps", `function f(x: [2]boolean): int { return match(x) { case [true, true] => 1, case [false, false] => 0 } }`, "non-exhaustive match"},
		{"field filters", `function f(x: { a: boolean, n: int }): int { return match(x) { case { a: true, n: 7 } => 1, case { a: false } => 0 } }`, "non-exhaustive match"},
		{"guard leaves gap", `function f(x: [1]boolean, allow: boolean): int { return match(x) { case [true] if (allow) => 1, case [false] => 0 } }`, "non-exhaustive match"},
		{"rest indices differ", `function f(x: [3]boolean): int { return match(x) { case [true, ..._] => 1, case [..._, false] => 0 } }`, "non-exhaustive match"},
		{"slice length is a filter", `function f(x: { a: boolean, values: int[] }): int { return match(x) { case { a: true, values: [] } => 1, case { a: false } => 0 } }`, "non-exhaustive match"},
		{"result success gap", `function f(load: () => Result<[2]boolean>): int { return match(load()) { case ok([true, true]) => 1, case ok([false, false]) => 0, case fail(_) => -1 } }`, "Missing full coverage for ok"},
		{"result failure stays required", `function f(load: () => Result<[1]boolean>): int { return match(load()) { case ok([true]) => 1, case ok([false]) => 0 } }`, "Missing full coverage for fail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, tc.input), "\n")
			if tc.diagnostic == "" && diagnostics != "" || tc.diagnostic != "" && !strings.Contains(diagnostics, tc.diagnostic) {
				t.Fatalf("expected %q, got %q", tc.diagnostic, diagnostics)
			}
		})
	}
}
