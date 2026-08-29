package sema

import (
	"strings"
	"testing"
)

func TestStructuredTaskSemanticMatrix(t *testing.T) {
	diagnostics := checkSource(t, `
import go errors from "errors";
function produceNumber(value: int): int { return value; }
function notify(): void {}
function load(okay: boolean): Result<int> {
  if (okay) { return ok(7); }
  return fail(errors.New("failed"));
}
function ordinary(): int {
  const task: Task<int> = go produceNumber(4);
  return await task;
}
function direct(): int { return await go produceNumber(5); }
function waitVoid(): void { const task = go notify(); await task; }
function detached(): void { const task = go notify(); detach task; }
function detachedDirect(): void { detach go notify(); }
function result(okay: boolean): Result<int> {
  const task: Task<Result<int>> = go load(okay);
  const value = await task?;
  return ok(value);
}
function both(flag: boolean): int {
  const task = go produceNumber(6);
  if (flag) { return await task; }
  return await task;
}
function terminatingBranches(flag: boolean): int {
  const task = go produceNumber(8);
  if (flag) { return await task; } else { return await task; }
}
function nestedAwait(): int {
  const task = go produceNumber(9);
  return 1 + await task;
}
function loopTasks(values: int[]): int {
  let total = 0;
  for (const value of values) {
    const task = go produceNumber(value);
    total += await task;
  }
  return total;
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsInvalidTaskUses(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"await value", `function bad(): void { await 1; }`, "await requires Task"},
		{"detach value", `function bad(): void { detach 1; }`, "detach requires Task"},
		{"unconsumed", `function value(): int { return 1; } function bad(): void { const task = go value(); }`, "must be consumed"},
		{"return before consume", `function value(): int { return 1; } function bad(flag: boolean): int { const task = go value(); if (flag) { return 0; } return await task; }`, "must be consumed"},
		{"double await", `function value(): int { return 1; } function bad(): int { const task = go value(); const first = await task; return first + await task; }`, "already been consumed"},
		{"branch maybe consumed", `function value(): int { return 1; } function bad(flag: boolean): int { const task = go value(); if (flag) { const first = await task; } return await task; }`, "may already have been consumed"},
		{"loop may consume repeatedly", `function value(): int { return 1; } function bad(run: boolean): void { const task = go value(); while (run) { const item = await task; run = false; } detach task; }`, "may already have been consumed"},
		{"copy", `function value(): int { return 1; } function bad(): int { const task = go value(); const copy = task; return await task; }`, "cannot be copied or passed"},
		{"pass", `function value(): int { return 1; } function take(value: int): void {} function bad(): int { const task = go value(); take(task); return await task; }`, "cannot be copied or passed"},
		{"reassign", `function value(): int { return 1; } function bad(): int { let task = go value(); task = go value(); return await task; }`, "cannot be reassigned"},
		{"global", `function value(): int { return 1; } const task = go value();`, "global variables cannot contain Task"},
		{"parameter", `function bad(task: Task<int>): void { detach task; }`, "function parameters cannot contain Task"},
		{"return type", `function bad(): Task<int> { return go bad(); }`, "function return types cannot contain Task"},
		{"field", `class Bad { public task: Task<int>; }`, "class fields cannot contain Task"},
		{"array", `function value(): int { return 1; } function bad(): void { let tasks: Task<int>[] = []; }`, "Task cannot be nested inside an array"},
		{"nested task", `function value(): int { return 1; } function bad(): void { const task: Task<Task<int>> = go value(); }`, "cannot be used as a Task result"},
		{"task inside result", `function bad(): Result<Task<int>> { return fail(null); }`, "Task cannot be nested inside Result"},
		{"result await without propagation", `function load(): Result<int> { return ok(1); } function bad(): void { const task = go load(); const value = await task; }`, "must be consumed with ?"},
		{"raw multiple result", `import go strconv from "strconv"; function bad(): void { const task = go strconv.Atoi("1"); detach task; }`, "raw multiple-result Go call"},
		{"conversion", `function bad(): void { const task = go int(1); detach task; }`, "type conversions are not calls"},
		{"builtin", `function bad(): int { const task = go len([1]); return await task; }`, "does not support compiler built-ins"},
		{"closure capture", `function value(): int { return 1; } function bad(): int { const task = go value(); const wait = (): int => await task; return await task; }`, "cannot be captured by a closure"},
		{"runtime name collision", `struct __ontamaTask {} function value(): int { return 1; } function bad(): int { const task = go value(); return await task; }`, "reserved by the Task runtime"},
		{"task type collision", `struct Task {}`, "conflicts with a built-in type"},
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
