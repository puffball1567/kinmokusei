package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinallyAwaitsTaskBeforeReturnAndPropagation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "finally_task.km")
	input := `
import go errors from "errors";
import go atomic from "sync/atomic";
function work(counter: *atomic.Int64): int { counter.Add(1); return 7; }
function load(ready: boolean): Result<int> {
  if (!ready) { return fail(errors.New("missing")); }
  return ok(11);
}
function early(flag: boolean, counter: *atomic.Int64): int {
  const task = go work(counter);
  try { if (flag) { return 1; } }
  finally { const value = await task; }
  return 2;
}
function propagate(ready: boolean, counter: *atomic.Int64): Result<int> {
  const task = go work(counter);
  try { const value = load(ready)?; return ok(value); }
  catch (_: error) { return ok(99); }
  finally { const completed = await task; }
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "finallytask")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	reference := `package reference
import (
  "errors"
  "sync/atomic"
)
func work(counter *atomic.Int64) int { counter.Add(1); return 7 }
func early(flag bool, counter *atomic.Int64) int {
  done := make(chan int, 1)
  go func() { done <- work(counter) }()
  defer func() { <-done }()
  if flag { return 1 }
  return 2
}
func propagate(ready bool, counter *atomic.Int64) (int, error) {
  done := make(chan int, 1)
  go func() { done <- work(counter) }()
  defer func() { <-done }()
  if !ready { return 0, errors.New("missing") }
  return 11, nil
}
func Early(flag bool, counter *atomic.Int64) int { return early(flag, counter) }
func Propagate(ready bool, counter *atomic.Int64) (int, error) { return propagate(ready, counter) }
`
	comparison := `package finallytask
import (
  "testing"
  "sync/atomic"
  "finallytask.test/reference"
)
func TestFinallyTask(t *testing.T) {
  for _, flag := range []bool{false, true} {
    var generatedCounter, referenceCounter atomic.Int64
    if got, want := early(flag, &generatedCounter), reference.Early(flag, &referenceCounter); got != want {
      t.Errorf("early(%v) = %d, Go = %d", flag, got, want)
    }
    if got, want := generatedCounter.Load(), referenceCounter.Load(); got != want || got != 1 {
      t.Errorf("early(%v) joined worker = %d, Go = %d", flag, got, want)
    }
    generatedCounter.Store(0); referenceCounter.Store(0)
    got, gotErr := propagate(flag, &generatedCounter)
    want, wantErr := reference.Propagate(flag, &referenceCounter)
    if got != want || errText(gotErr) != errText(wantErr) {
      t.Errorf("propagate(%v) = (%d, %q), Go = (%d, %q)", flag, got, errText(gotErr), want, errText(wantErr))
    }
    if got, want := generatedCounter.Load(), referenceCounter.Load(); got != want || got != 1 {
      t.Errorf("propagate(%v) joined worker = %d, Go = %d", flag, got, want)
    }
  }
}
func errText(err error) string { if err == nil { return "" }; return err.Error() }
`
	runGeneratedGoDifferentialTest(t, root, "finallytask.test", generated, reference, comparison)
}
