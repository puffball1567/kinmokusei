package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResultMatchMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "results.km")
	input := `export struct Entry { public items: int[] }
export class Box { constructor(public n: int) {} }

export function Inspect(load: () => Result<Entry>, accept: (n: int) => boolean): int {
  return match(load()) {
    case ok({ items: [const n] }) if (accept(n)) => n
    case ok({ items: [] }) => 0
    case ok(_) => -1
    case fail(const problem) => -len(problem.Error())
  }
}
export function Switch(load: () => Result<boolean>): int {
  return match(load()) { case ok(true) => 1, case ok(false) => 0, case fail(_) => -1 }
}
export function Nullable(load: () => Result<*int | null>): int {
  return match(load()) { case ok(null) => 0, case ok(const n!) => *n, case fail(_) => -1 }
}
export function Void(run: () => Result<void>): int {
  return match(run()) { case ok() => 1, case fail(const problem) => -len(problem.Error()) }
}
export function Native(load: () => Result<Box>): int {
  return match(load()) { case ok(const value) => value.n, case fail(_) => -1 }
}
export function Await(load: () => Result<int>): int {
  const pending = go load()
  return match(await pending) { case ok(const n) => n, case fail(_) => -1 }
}
export function Recover<T>(load: () => Result<T>, fallback: T): T {
  return match(load()) { case ok(const value) => value, case fail(_) => fallback }
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "resultmatch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
func Inspect(load func() ([]int, error), accept func(int) bool) int {
  items, err := load()
  if err != nil { return -len(err.Error()) }
  if len(items) == 1 && accept(items[0]) { return items[0] }
  if len(items) == 0 { return 0 }
  return -1
}
func Switch(value bool, err error) int {
  if err != nil { return -1 }
  if value { return 1 }
  return 0
}
func Nullable(value *int, err error) int {
  if err != nil { return -1 }
  if value == nil { return 0 }
  return *value
}
func Void(err error) int {
  if err != nil { return -len(err.Error()) }
  return 1
}
func Recover[T any](load func() (T, error), fallback T) T {
  value, err := load()
  if err != nil { return fallback }
  return value
}
`
	comparison := `package resultmatch_test
import (
  "errors"
  "fmt"
  "strings"
  "testing"
  g "result-match.test"
  r "result-match.test/reference"
)
type typedFailure struct{}
func (*typedFailure) Error() string { return "typed nil" }
func TestMatch(t *testing.T) {
  var typedNil *typedFailure
  for _, failure := range []error{nil, errors.New("failed"), typedNil} {
    for _, items := range [][]int{nil, {7}, {-7}, {1, 2}} {
      gotCalls, wantCalls := 0, 0
      got := g.Inspect(func() (g.Entry, error) {
        gotCalls++
        return g.Entry{Items: items}, failure
      }, func(n int) bool { gotCalls += 10; return n > 0 })
      want := r.Inspect(func() ([]int, error) {
        wantCalls++
        return items, failure
      }, func(n int) bool { wantCalls += 10; return n > 0 })
      if got != want || gotCalls != wantCalls {
        t.Errorf("match %v/%v = %d in %d steps, want %d in %d steps", items, failure, got, gotCalls, want, wantCalls)
      }
    }
    for _, value := range []bool{true, false} {
      if got, want := g.Switch(func() (bool, error) { return value, failure }), r.Switch(value, failure); got != want {
        t.Errorf("boolean = %d, want %d", got, want)
      }
    }
    n := 4
    for _, value := range []*int{nil, &n} {
      if got, want := g.Nullable(func() (*int, error) { return value, failure }), r.Nullable(value, failure); got != want {
        t.Errorf("nullable = %d, want %d", got, want)
      }
    }
    if got, want := g.Void(func() error { return failure }), r.Void(failure); got != want {
      t.Errorf("void = %d, want %d", got, want)
    }
    load := func() (int, error) { return n, failure }
    if got, want := g.Await(load), r.Recover(load, -1); got != want {
      t.Errorf("await = %d, want %d", got, want)
    }
    if got, want := g.Recover(load, -3), r.Recover(load, -3); got != want {
      t.Errorf("generic = %d, want %d", got, want)
    }
  }
  if got := g.Native(func() (*g.Box, error) { return nil, typedNil }); got != -1 {
    t.Errorf("failure exposed nil success payload, got %d", got)
  }
  if got := g.Native(func() (*g.Box, error) { return g.NewBox(9), nil }); got != 9 {
    t.Errorf("native value = %d, want 9", got)
  }
  defer func() {
    if got := fmt.Sprint(recover()); !strings.Contains(got, "match binding failed at results.km:") {
      t.Errorf("nil success panic = %q", got)
    }
  }()
  g.Native(func() (*g.Box, error) { return nil, nil })
}
`
	runGeneratedGoDifferentialTest(t, root, "result-match.test", generated, reference, comparison)
}
