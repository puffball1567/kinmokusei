package sema

import (
	"strings"
	"testing"
)

func TestMatchResultReturnChecking(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, input, diagnostic string }{
		{"construct and forward", `function f(on: boolean, load: () => Result<int>): Result<int> { return match(on) { case true => load(), case false => ok(0) } }`, ""},
		{"generic return", `function f<T>(on: boolean, value: T, err: error): Result<T> { return match(on) { case true => ok(value), case false => fail(err) } }`, ""},
		{"block arrow", `const f = (on: boolean): Result<int> => { return match(on) { case true => ok(1), case false => ok(2) } }`, ""},
		{"ordinary arm", `function f(on: boolean): Result<int> { return match(on) { case true => ok(1), case false => 2 } }`, "must return ok(...) or fail(...)"},
		{"invalid success payload", `function f(on: boolean): Result<int> { return match(on) { case true => ok("wrong"), case false => ok(2) } }`, "cannot use string as int"},
		{"wrong forwarded effect", `function f(on: boolean, load: () => Result<string>): Result<int> { return match(on) { case true => load(), case false => ok(2) } }`, "got Result<string>"},
		{"raw Go results", `import go strconv from "strconv"; function f(on: boolean): Result<int> { return match(on) { case true => strconv.Atoi("1"), case false => ok(2) } }`, "not implicitly converted to Result"},
		{"uncovered arm", `function f(on: boolean): Result<int> { return match(on) { case true => ok(1) } }`, "non-exhaustive match"},
		{"unused error arm", `function f(load: () => Result<int>): Result<int> { return match(load()) { case ok(const n) => ok(n), case fail(const err) => ok(0) } }`, `Result error binding "err" is never used`},
		{"not storable", `function f(on: boolean): Result<int> { const v = match(on) { case true => ok(1), case false => ok(2) }; return ok(1); }`, "Result values must be consumed"},
		{"task missing in one arm", `function f(on: boolean, load: () => Result<int>): Result<int> { const task = go load(); return match(on) { case true => await task, case false => ok(0) } }`, "not consumed on every control-flow path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, tc.input), "\n")
			if tc.diagnostic == "" && got != "" || tc.diagnostic != "" && !strings.Contains(got, tc.diagnostic) {
				t.Fatalf("expected %q, got %q", tc.diagnostic, got)
			}
		})
	}
}
