package sema

import (
	"strings"
	"testing"
)

func TestResultPropagationAndSplitSemanticMatrix(t *testing.T) {
	diagnostics := checkSource(t, `
import go strconv from "strconv";
import go errors from "errors";

interface Loader { function load(text: string): Result<int>; }

function parse(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  return ok(value);
}
function forward(text: string): Result<int> { return parse(text); }
function reject(): Result<int> { return fail(errors.New("rejected")); }
function notify(): Result<void> {
  errors.New("stop")?;
  return ok();
}
function split(text: string): int {
  const [value, err] = parse(text);
  if (err != nil) { return -1; }
  return value;
}
function splitVoid(): error {
  const [err] = notify();
  return err;
}
class Parser implements Loader {
  public function load(text: string): Result<int> { return parse(text); }
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsInvalidResultPropagationAndSplitUses(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing type argument", `function bad(): Result { return 1; }`, "Result expects one type argument"},
		{"nested result", `function bad(): Result<Result<int>> { return ok(1); }`, "nested Result"},
		{"parameter result", `function bad(value: Result<int>): void {}`, "not for parameters"},
		{"variable result", `function bad(): void { let value: Result<int> = 1; }`, "not for variables"},
		{"field result", `class Bad { public value: Result<int>; }`, "not for fields"},
		{"ok outside result", `function bad(): int { return ok(1); }`, "only be used inside a Result-returning function"},
		{"wrong ok value", `function bad(): Result<int> { return ok("x"); }`, "cannot use string as int"},
		{"wrong fail value", `function bad(): Result<int> { return fail(1); }`, "as error"},
		{"propagation outside result", `import go strconv from "strconv"; function bad(): int { const value = strconv.Atoi("1")?; return value; }`, "operator ? may only be used inside"},
		{"propagation wrong operand", `function bad(): Result<int> { const value = 1?; return ok(value); }`, "operator ? requires Result<T>"},
		{"propagation nested expression", `function source(): Result<int> { return ok(1); } function bad(): Result<int> { return ok(source()?); }`, "result propagation may only be used as a variable initializer"},
		{"nonvoid propagation statement", `function source(): Result<int> { return ok(1); } function bad(): Result<void> { source()?; return ok(); }`, "must be bound to a variable"},
		{"discarded result", `function source(): Result<int> { return ok(1); } function bad(): Result<void> { source(); return ok(); }`, "must be consumed with ?, explicitly split, or returned"},
		{"result split count", `function source(): Result<int> { return ok(1); } function bad(): void { const [value, err, extra] = source(); }`, "Result binding count mismatch"},
		{"void result split count", `function source(): Result<void> { return ok(); } function bad(): void { const [value, err] = source(); }`, "Result binding count mismatch"},
		{"deferred result", `function source(): Result<int> { return ok(1); } function bad(): Result<void> { defer source(); return ok(); }`, "defer cannot discard a Result"},
		{"implicit raw conversion", `import go strconv from "strconv"; function bad(text: string): Result<int> { return strconv.Atoi(text); }`, "not implicitly converted to Result"},
		{"plain result return", `function bad(): Result<int> { return 1; }`, "must return ok(...) or fail(...)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics = %v, want substring %q", diagnostics, test.want)
			}
		})
	}
}
