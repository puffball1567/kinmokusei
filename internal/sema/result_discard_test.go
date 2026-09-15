package sema

import (
	"strings"
	"testing"
)

const discardPrelude = `
function load(): Result<int> { return ok(1); }
function notify(): Result<void> { return ok(); }
`

func TestExplicitResultDiscard(t *testing.T) {
	t.Parallel()
	tests := []string{
		`function use(): void { const _ = load(); let _ = notify(); _ = load(); _ = notify(); }`,
		`function use(): void { const [value, _] = load(); const [_, _] = load(); const [_] = notify(); }`,
		`function use(): void { const [_, err] = load(); _ = err; }`,
		`function use(): void { const [_, err] = load(); const _ = err; }`,
		`function use(): error { const [_, err] = load(); return err; }`,
		`function use(): void { const [_, err] = load(); if (err != nil) { _ = err.Error(); } }`,
		`function use(): ()=>error { const [_, err] = load(); return ()=>err; }`,
		`function use(): void { let value = 0; let err: error = nil; [value, err] = load(); _ = err; }`,
		`function use(): void { const [_, err] = load(); { const [_, err] = load(); _ = err; } _ = err; }`,
		`function use(): Result<void> { const _ = load()?; const _ = notify()?; return ok(); }`,
		`function use(): void { const _ = 1; const _: int = 2; _ = "ignored"; const _ = (): int => 3; const _ = (): int => 4; }`,
		`function use(): void { const first = ()=>second(); const second = ()=>{ const [_, err] = load(); return err; }; _ = first(); }`,
		`function use(): void { for (const _ = load(); false; _ = notify()) {} }`,
		`import go strconv from "strconv"; function use(): void { _ = strconv.Atoi("1"); const _ = strconv.Atoi("2"); }`,
		// Raw Go errors retain their existing policy; this check targets Result bindings.
		`import go strconv from "strconv"; function use(): void { const [value, err] = strconv.Atoi("1"); }`,
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if diagnostics := checkSource(t, discardPrelude+input); len(diagnostics) != 0 {
				t.Fatalf("diagnostics: %v", diagnostics)
			}
		})
	}
}

func TestRejectUnusedResultErrors(t *testing.T) {
	t.Parallel()
	tests := []string{
		`function use(): int { const [value, err] = load(); return value; }`,
		`function use(): void { const [err] = notify(); }`,
		`function use(): void { const [_, _err] = load(); }`,
		`function use(): void { let [_, err] = load(); err = nil; }`,
		`function use(): void { let value = 0; let err: error = nil; [value, err] = load(); }`,
		`function use(): void { const [_, err] = load(); { const [_, err] = load(); _ = err; } }`,
		`function use(): void { const first = ()=>second(); const second = ()=>{ const [_, err] = load(); return 1; }; }`,
		`function use(): void { for (const [_, err] = load(); false;) {} }`,
		`class Loader { public function use(): void { const [_, err] = load(); } }`,
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, discardPrelude+input), "\n")
			if !strings.Contains(diagnostics, "Result error binding") || !strings.Contains(diagnostics, "is never used") {
				t.Fatalf("diagnostics: %s", diagnostics)
			}
		})
	}
}

func TestDiscardStillChecksTypesAndEffects(t *testing.T) {
	t.Parallel()
	tests := []struct{ source, want string }{
		{`function use(): void { const _: int = "bad"; }`, "cannot use string as int"},
		{`function use(): void { const _ = missing; }`, "undefined name"},
		{`function use(): void { const _ = 1; console(_); }`, "undefined name"},
		{`function use(): void { _ += 1; }`, "undefined name"},
		{`function use(): void { const _ = go load(); }`, "Task values cannot be discarded"},
		{`function use(): void { const task = go load(); _ = task; }`, "Task values cannot be discarded"},
		{`function nothing(): void {} function use(): void { const _ = nothing(); }`, "no value"},
		{`function use(): void { const _: int = load(); }`, "Result"},
		{`function use(): void { const _ = load()?; }`, "operator ? may only be used inside"},
		{`function use(): void { load(); }`, "must be consumed"},
		{`function use(): Result<int> { const _ = ok(1); return ok(2); }`, "Result constructors must be returned"},
		{`function use(): void { _ = nil; }`, "cannot infer"},
		{`function use(): void { const _ = null; }`, "cannot infer"},
		{`import go fmt from "fmt"; function use(): void { _ = fmt; }`, "namespace cannot be used"},
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, discardPrelude+test.source), "\n")
			if !strings.Contains(diagnostics, test.want) {
				t.Fatalf("diagnostics: %s, want %q", diagnostics, test.want)
			}
		})
	}
}
