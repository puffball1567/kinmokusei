package sema

import (
	"strings"
	"testing"
)

func TestResultMatchDiagnostics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, diagnostic string
	}{
		{"missing failure", `function f(load: () => Result<int>): int { return match(load()) { case ok(const n) => n } }`, "Missing full coverage for fail"},
		{"missing success", `function f(load: () => Result<int>): int { return match(load()) { case fail(_) => -1 } }`, "Missing full coverage for ok"},
		{"guarded success", `function f(load: () => Result<int>): int { return match(load()) { case ok(const n) if (n > 0) => n, case fail(_) => -1 } }`, "Missing full coverage for ok"},
		{"filtered success", `function f(load: () => Result<int>): int { return match(load()) { case ok(42) => 1, case fail(_) => -1 } }`, "Missing full coverage for ok"},
		{"guarded failure", `function f(load: () => Result<int>): int { return match(load()) { case ok(const n) => n, case fail(const err) if (len(err.Error()) > 0) => -1 } }`, "Missing full coverage for fail"},
		{"implicit discard", `function f(load: () => Result<int>): int { return match(load()) { case ok(const n) => n, default => -1 } }`, "Result match arms must use ok or fail patterns"},
		{"missing success pattern", `function f(load: () => Result<int>): int { return match(load()) { case ok() => 0, case fail(_) => -1 } }`, "ok pattern requires a payload pattern"},
		{"missing error pattern", `function f(load: () => Result<void>): int { return match(load()) { case ok() => 0, case fail() => -1 } }`, "fail pattern requires a payload pattern"},
		{"void success with payload", `function f(load: () => Result<void>): int { return match(load()) { case ok(_) => 0, case fail(_) => -1 } }`, "Result<void> success pattern must be ok()"},
		{"ordinary subject", `function f(x: int): int { return match(x) { case ok(_) => 1, default => 0 } }`, "ok and fail patterns require a Result subject"},
		{"wrong payload type", `function f(load: () => Result<int>): int { return match(load()) { case ok("x") => 1, case ok(_) => 0, case fail(_) => -1 } }`, "cannot use string as int"},
		{"unreachable variant", `function f(load: () => Result<int>): int { return match(load()) { case ok(_) => 0, case ok(42) => 1, case fail(_) => -1 } }`, "unreachable match arm"},
		{"unused error", `function f(load: () => Result<int>): int { return match(load()) { case ok(const n) => n, case fail(const err) => -1 } }`, `Result error binding "err" is never used`},
		{"payload scope", `function f(load: () => Result<int>): int { return match(load()) { case ok(const n) => n, case fail(_) => n } }`, `undefined name "n"`},
		{"constructor subject", `function f(): Result<int> { const n = match(ok(1)) { case ok(const n) => n, case fail(_) => -1 }; return ok(n); }`, "Result constructors must be returned"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if diagnostics := checkSource(t, tc.input); !strings.Contains(strings.Join(diagnostics, "\n"), tc.diagnostic) {
				t.Fatalf("expected %q, got %v", tc.diagnostic, diagnostics)
			}
		})
	}
}
