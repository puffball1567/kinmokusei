package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuredTasksMatchIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "task.otm")
	input := `
import go errors from "errors";
import go atomic from "sync/atomic";
import go sync from "sync";

function produce(value: int): int { return value; }
function echo64(value: int64): int64 { return value; }
function next(counter: *atomic.Int64): int64 { return counter.Add(1); }
function choose(counter: *atomic.Int64): (value: int64) => int64 {
  counter.Add(1);
  return echo64;
}
function notify(counter: *atomic.Int64): void { counter.Add(1); }
function notifyDone(counter: *atomic.Int64, done: *sync.WaitGroup): void { counter.Add(1); done.Done(); }
function blocked(started: *sync.WaitGroup, release: *sync.WaitGroup, value: int): int {
  started.Done();
  release.Wait();
  return value;
}
function load(ready: boolean): Result<int> {
  if (!ready) { return fail(errors.New("not ready")); }
  return ok(7);
}
function ensure(ready: boolean): Result<void> {
  if (!ready) { return fail(errors.New("not ready")); }
  return ok();
}
function explode(): int { const values = [1]; return values[2]; }

function ordinary(): int {
  const task: Task<int> = go produce(4);
  return await task;
}
function direct(): int { return await go produce(5); }
function argumentEvaluation(): int64 {
  let counter = atomic.Int64{};
  const task = go echo64(next(&counter));
  const before = counter.Load();
  return before * 10 + await task;
}
function calleeEvaluation(): int64 {
  let counter = atomic.Int64{};
  const task = go choose(&counter)(next(&counter));
  const before = counter.Load();
  return before * 10 + await task;
}
function concurrentStart(): int {
  let started = sync.WaitGroup{};
  let release = sync.WaitGroup{};
  started.Add(2);
  release.Add(1);
  const first = go blocked(&started, &release, 20);
  const second = go blocked(&started, &release, 22);
  started.Wait();
  release.Done();
  return await first + await second;
}
function loopTasks(values: int[]): int {
  let total = 0;
  for (const value of values) {
    const task = go produce(value);
    total += await task;
  }
  return total;
}
function awaitVoid(counter: *atomic.Int64): void {
  const task = go notify(counter);
  await task;
}
function detachVoid(counter: *atomic.Int64, done: *sync.WaitGroup): void {
  const task = go notifyDone(counter, done);
  detach task;
}
function resultValue(ready: boolean): Result<int> {
  const task: Task<Result<int>> = go load(ready);
  const value = await task?;
  return ok(value * 2);
}
function resultVoid(ready: boolean): Result<void> {
  const task: Task<Result<void>> = go ensure(ready);
  await task?;
  return ok();
}
function awaitPanic(): int {
  const task = go explode();
  return await task;
}
function detachPanic(): void { detach go explode(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "taskmatrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, want := range []string{"type __ontamaTask[T any] struct", "type __ontamaResultTask[T any] struct", "panic(task.panicValue)", "go func()"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated task runtime does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference
import (
  "errors"
  "sync"
  "sync/atomic"
)
func produce(value int) int { return value }
func echo64(value int64) int64 { return value }
func next(counter *atomic.Int64) int64 { return counter.Add(1) }
func choose(counter *atomic.Int64) func(int64) int64 { counter.Add(1); return echo64 }
func notify(counter *atomic.Int64) { counter.Add(1) }
func notifyDone(counter *atomic.Int64, done *sync.WaitGroup) { counter.Add(1); done.Done() }
func blocked(started, release *sync.WaitGroup, value int) int { started.Done(); release.Wait(); return value }
func load(ready bool) (int, error) { if !ready { return 0, errors.New("not ready") }; return 7, nil }
func ensure(ready bool) error { if !ready { return errors.New("not ready") }; return nil }
func Ordinary() int {
  done := make(chan struct{}); var value int
  go func() { value = produce(4); close(done) }()
  <-done; return value
}
func Direct() int {
  done := make(chan struct{}); var value int
  go func() { value = produce(5); close(done) }()
  <-done; return value
}
func ArgumentEvaluation() int64 {
  var counter atomic.Int64
  argument := next(&counter)
  done := make(chan struct{}); var value int64
  go func() { value = echo64(argument); close(done) }()
  before := counter.Load(); <-done; return before*10 + value
}
func CalleeEvaluation() int64 {
  var counter atomic.Int64
  function := choose(&counter); argument := next(&counter)
  done := make(chan struct{}); var value int64
  go func() { value = function(argument); close(done) }()
  before := counter.Load(); <-done; return before*10 + value
}
func ConcurrentStart() int {
  var started, release sync.WaitGroup; started.Add(2); release.Add(1)
  firstDone, secondDone := make(chan struct{}), make(chan struct{})
  first, second := 0, 0
  go func() { first = blocked(&started, &release, 20); close(firstDone) }()
  go func() { second = blocked(&started, &release, 22); close(secondDone) }()
  started.Wait(); release.Done(); <-firstDone; <-secondDone; return first + second
}
func LoopTasks(values []int) int {
  total := 0
  for _, value := range values {
    done := make(chan struct{}); result := 0
    go func(input int) { result = produce(input); close(done) }(value)
    <-done; total += result
  }
  return total
}
func AwaitVoid(counter *atomic.Int64) {
  done := make(chan struct{}); go func() { notify(counter); close(done) }(); <-done
}
func DetachVoid(counter *atomic.Int64, done *sync.WaitGroup) {
  taskDone := make(chan struct{}); go func() { notifyDone(counter, done); close(taskDone) }()
  go func() { <-taskDone }()
}
func ResultValue(ready bool) (int, error) {
  done := make(chan struct{}); var value int; var err error
  go func() { value, err = load(ready); close(done) }(); <-done
  if err != nil { return 0, err }; return value*2, nil
}
func ResultVoid(ready bool) error {
  done := make(chan struct{}); var err error
  go func() { err = ensure(ready); close(done) }(); <-done; return err
}
func AwaitPanic() int {
  done := make(chan struct{}); var panicValue any
  go func() { defer close(done); defer func() { panicValue = recover() }(); _ = []int{1}[2] }()
  <-done; if panicValue != nil { panic(panicValue) }; return 0
}
func DetachPanic() {
  done := make(chan struct{}); var panicValue any
  go func() { defer close(done); defer func() { panicValue = recover() }(); _ = []int{1}[2] }()
  go func() { <-done; if panicValue != nil { panic(panicValue) } }()
}
`
	testSource := `package taskmatrix
import (
  "os"
  "os/exec"
  "strings"
  "sync"
  "sync/atomic"
  "testing"
  "time"
  reference "task.test/reference"
)
func TestTaskBehavior(t *testing.T) {
  if got, want := ordinary(), reference.Ordinary(); got != want { t.Errorf("ordinary = %d, Go = %d", got, want) }
  if got, want := direct(), reference.Direct(); got != want { t.Errorf("direct = %d, Go = %d", got, want) }
  if got, want := argumentEvaluation(), reference.ArgumentEvaluation(); got != want { t.Errorf("argument evaluation = %d, Go = %d", got, want) }
  if got, want := calleeEvaluation(), reference.CalleeEvaluation(); got != want { t.Errorf("callee evaluation = %d, Go = %d", got, want) }
  if got, want := concurrentStart(), reference.ConcurrentStart(); got != want { t.Errorf("concurrent start = %d, Go = %d", got, want) }
  for _, values := range [][]int{nil, {}, {1}, {1, 2, 3, -4}} {
    if got, want := loopTasks(values), reference.LoopTasks(values); got != want { t.Errorf("loop tasks %v = %d, Go = %d", values, got, want) }
  }
  var generatedCounter, goCounter atomic.Int64
  awaitVoid(&generatedCounter); reference.AwaitVoid(&goCounter)
  if got, want := generatedCounter.Load(), goCounter.Load(); got != want { t.Errorf("await void = %d, Go = %d", got, want) }
  var generatedDone, goDone sync.WaitGroup; generatedDone.Add(1); goDone.Add(1)
  detachVoid(&generatedCounter, &generatedDone); reference.DetachVoid(&goCounter, &goDone)
  generatedDone.Wait(); goDone.Wait()
  if got, want := generatedCounter.Load(), goCounter.Load(); got != want { t.Errorf("detach void = %d, Go = %d", got, want) }
  for _, ready := range []bool{true, false} {
    generatedValue, generatedErr := resultValue(ready); goValue, goErr := reference.ResultValue(ready)
    if generatedValue != goValue || errorText(generatedErr) != errorText(goErr) { t.Errorf("result value(%v) = (%d,%q), Go = (%d,%q)", ready, generatedValue, errorText(generatedErr), goValue, errorText(goErr)) }
    if got, want := errorText(resultVoid(ready)), errorText(reference.ResultVoid(ready)); got != want { t.Errorf("result void(%v) = %q, Go = %q", ready, got, want) }
  }
  if got, want := didPanic(awaitPanic), didPanic(reference.AwaitPanic); got != want || !got { t.Errorf("await panic = %v, Go = %v", got, want) }
}
func errorText(err error) string { if err == nil { return "" }; return err.Error() }
func didPanic(call func() int) (panicked bool) { defer func() { panicked = recover() != nil }(); call(); return false }
func TestDetachedPanicIsFatal(t *testing.T) {
  if mode := os.Getenv("ONTAMA_DETACHED_PANIC_CHILD"); mode != "" {
    if mode == "generated" { detachPanic() } else { reference.DetachPanic() }
    time.Sleep(time.Second)
    t.Fatal("detached panic did not terminate the process")
  }
  for _, mode := range []string{"generated", "reference"} {
    command := exec.Command(os.Args[0], "-test.run=TestDetachedPanicIsFatal")
    command.Env = append(os.Environ(), "ONTAMA_DETACHED_PANIC_CHILD="+mode)
    output, err := command.CombinedOutput()
    if err == nil || !strings.Contains(string(output), "panic") {
      t.Errorf("detached panic %s: err=%v output=%s", mode, err, output)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "task.test", generated, referenceSource, testSource)
}
