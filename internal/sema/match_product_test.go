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
		{"record with enum and nullable field", `enum State { Idle = -2, Ready = 4, Running = 4 }
			function f(x: { state: State, count: *int | null }): int { return match(x) {
				case { state: State.Idle } => 0,
				case { state: State.Ready, count: null } => 1,
				case { state: State.Running, count: const p! } => *p
			} }`, ""},
		{"enum wildcard rows cover missing alternatives", `enum State { Idle, Ready, Closed }
			function f(x: { state: State, flag: boolean }): int { return match(x) {
				case { state: State.Ready, flag: true } => 1, case { flag: true } => 2, case { flag: false } => 3
			} }`, ""},
		{"distinct enum domains", `enum State { Idle, Ready }
			enum Action { Wait, Start, Stop }
			function f(x: { state: State, action: Action }): int { return match(x) {
				case { state: State.Idle } => 0,
				case { state: State.Ready, action: Action.Wait } => 1,
				case { state: State.Ready, action: Action.Start } => 2,
				case { state: State.Ready, action: Action.Stop } => 3
			} }`, ""},
		{"fixed array enum suffix", `enum State { Idle = 3, Ready = 7 }
			function f(x: [2]State): int { return match(x) { case [..._, State.Idle] => 1, case [_, 7] => 2 } }`, ""},
		{"enum result payload", `enum State { Only = 3, Alias = 3 }
			function f(load: () => Result<State>): int { return match(load()) { case ok(State.Alias) => 1, case fail(_) => -1 } }`, ""},
		{"diagonal leaves gaps", `function f(x: [2]boolean): int { return match(x) { case [true, true] => 1, case [false, false] => 0 } }`, "non-exhaustive match"},
		{"enum diagonal leaves gaps", `enum State { Idle, Ready }
			function f(x: [2]State): int { return match(x) {
				case [State.Idle, State.Idle] => 1, case [State.Ready, State.Ready] => 2
			} }`, "non-exhaustive match"},
		{"guarded enum arm leaves gap", `enum State { Idle, Ready }
			function f(x: { state: State }, allow: boolean): int { return match(x) {
				case { state: State.Idle } => 1, case { state: State.Ready } if (allow) => 2
			} }`, "non-exhaustive match"},
		{"undeclared integer leaves gap", `enum State { Idle = 3, Ready = 7 }
			function f(x: [1]State): int { return match(x) { case [State.Idle] => 1, case [0] => 0 } }`, "non-exhaustive match"},
		{"enum slice elements leave length and contents gaps", `enum State { Idle, Ready }
			function f(x: State[]): int { return match(x) { case [State.Idle] => 1, case [State.Ready] => 2 } }`, "non-exhaustive match"},
		{"field filters", `function f(x: { a: boolean, n: int }): int { return match(x) { case { a: true, n: 7 } => 1, case { a: false } => 0 } }`, "non-exhaustive match"},
		{"guard leaves gap", `function f(x: [1]boolean, allow: boolean): int { return match(x) { case [true] if (allow) => 1, case [false] => 0 } }`, "non-exhaustive match"},
		{"rest indices differ", `function f(x: [3]boolean): int { return match(x) { case [true, ..._] => 1, case [..._, false] => 0 } }`, "non-exhaustive match"},
		{"slice length is a filter", `function f(x: { a: boolean, values: int[] }): int { return match(x) { case { a: true, values: [] } => 1, case { a: false } => 0 } }`, "non-exhaustive match"},
		{"result success gap", `function f(load: () => Result<[2]boolean>): int { return match(load()) { case ok([true, true]) => 1, case ok([false, false]) => 0, case fail(_) => -1 } }`, "Missing full coverage for ok"},
		{"result failure stays required", `function f(load: () => Result<[1]boolean>): int { return match(load()) { case ok([true]) => 1, case ok([false]) => 0 } }`, "Missing full coverage for fail"},
		{"enum result still requires failed case", `enum State { Idle, Ready }
			function f(load: () => Result<State>): int { return match(load()) { case ok(State.Idle) => 1, case ok(State.Ready) => 2 } }`, "Missing full coverage for fail"},
		{"enum result with missing success member", `enum State { Idle, Ready }
			function f(load: () => Result<State>): int { return match(load()) { case ok(State.Idle) => 1, case fail(_) => -1 } }`, "Missing full coverage for ok"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, tc.input), "\n")
			if tc.diagnostic == "" && diagnostics != "" || tc.diagnostic != "" && !strings.Contains(diagnostics, tc.diagnostic) {
				t.Fatalf("expected %q, got %q", tc.diagnostic, diagnostics)
			}
		})
	}
}
