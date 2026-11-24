package sema

import (
	"strings"
	"testing"
)

func TestTypedMatchChecking(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, input, diagnostic string }{
		{"interface narrowing", `import go io from "io"; import go strings from "strings"; function f(x: io.Reader): int { return match(x) { case const r as *strings.Reader if (r != nil) => r.Len(), case nil => 0, default => -1 } }`, ""},
		{"class downcast", `class Base {} class Child extends Base { public n: int = 4; } function f(x: Base | null): int { return match(x) { case const y as Child => y.n, case null => 0, default => -1 } }`, ""},
		{"blank type binding", `function f(x: error): int { return match(x) { case const _ as error => 1, default => 0 } }`, ""},
		{"concrete subject", `function f(x: int): int { return match(x) { case const y as int => y, default => 0 } }`, "requires a Go interface value"},
		{"type parameter subject", `function f<T>(x: T): int { return match(x) { case const y as int => y, default => 0 } }`, "requires a Go interface value"},
		{"impossible concrete target", `function f(x: error): int { return match(x) { case const y as int => y, default => 0 } }`, "cannot contain asserted type int"},
		{"invalid typed nil pattern", `function f(x: int): int { return match(x) { case nil => 0, default => x } }`, "nil match pattern requires a Go interface value"},
		{"verified text assertion", `function f(x: interface {}): int { return match(x) { case const text as string => len(text), default => 0 } }`, "cannot establish a UTF-8 string contract"},
		{"target outside class hierarchy", `class A {} class B {} function f(x: A): int { return match(x) { case const _ as B => 1, default => 0 } }`, "not in the same inheritance chain"},
		{"missing fallback", `function f(x: error): int { return match(x) { case const _ as error => 1 } }`, "non-exhaustive match"},
		{"binding scope", `function f(x: interface {}): int { return match(x) { case const y as int => y, default => y } }`, `undefined name "y"`},
		{"unread narrowed error", `import go os from "os"; function f(load: () => Result<int>): int { return match(load()) { case ok(const n) => n, case fail(const path as *os.PathError) => -1, case fail(_) => -2 } }`, `Result error binding "path" is never used`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, tc.input), "\n")
			if tc.diagnostic == "" && got != "" || tc.diagnostic != "" && !strings.Contains(got, tc.diagnostic) {
				t.Fatalf("wanted %q, got %q", tc.diagnostic, got)
			}
		})
	}
}
