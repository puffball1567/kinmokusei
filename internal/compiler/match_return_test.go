package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchResultReturnMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "match_return.km")
	input := `export class Box { constructor(public n: int) {} }

export function Transform(load: () => Result<int>, allow: (n: int) => boolean, encode: (n: int) => Result<string>): Result<string> {
  return match (load()) {
    case ok(const n) if (allow(n)) => encode(n)
    case ok(_) => ok("skipped")
    case fail(const err) => fail(err)
  }
}
export function Notify(load: () => Result<int>, notify: () => Result<void>, done: () => void): Result<void> {
  try {
    return match (load()) {
      case ok(const n) => match (n) {
        case 0 => ok()
        default => notify()
      }
      case fail(const err) => fail(err)
    }
  } finally {
    done()
  }
}
export function Async(on: boolean, load: () => Result<int>): Result<int> {
  const work = go load()
  return match (on) {
    case true => await work
    case false => match (await work) {
      case ok(const n) => ok(n + 1)
      case fail(const err) => fail(err)
    }
  }
}
export function Forward(on: boolean, load: () => Result<Box>, fallback: () => Result<Box>): Result<Box> {
  const Box = 10
  return match (on) { case true => load(), case false => fallback() }
}
export const Arrow = (value: int): Result<int> => {
  return match (value) { case 0 => ok(1), default => ok(value) }
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "matchreturn")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
func Transform(load func() (int,error), allow func(int) bool, encode func(int) (string,error)) (string,error) {
  n, err := load()
  if err != nil { return "", err }
  if allow(n) { return encode(n) }
  return "skipped", nil
}
func Notify(load func() (int,error), notify func() error, done func()) error {
  defer done()
  n, err := load()
  if err != nil || n == 0 { return err }
  return notify()
}
func Async(on bool, load func() (int,error)) (int,error) {
  n, err := load()
  if !on && err == nil { n++ }
  if !on && err != nil { n = 0 }
  return n, err
}
`
	comparison := `package matchreturn_test
import (
  "errors"
  "fmt"
  "strings"
  "testing"
  g "match-return.test"
  r "match-return.test/reference"
)
func TestMatch(t *testing.T) {
  failure := errors.New("load failed")
  encodingFailure := errors.New("encode failed")
  for _, input := range []int{0, 3} {
    for _, inputErr := range []error{nil, failure} {
      for _, outputErr := range []error{nil, encodingFailure} {
        makeLoad := func(trace *string) func() (int,error) { return func() (int,error) { *trace += "L"; return input, inputErr } }
        makeGuard := func(trace *string) func(int) bool { return func(n int) bool { *trace += "G"; return n > 0 } }
        makeEncode := func(trace *string) func(int) (string,error) { return func(n int) (string,error) { *trace += "E"; return fmt.Sprint(n), outputErr } }
        var actual, expected string
        got, err := g.Transform(makeLoad(&actual), makeGuard(&actual), makeEncode(&actual))
        want, wantErr := r.Transform(makeLoad(&expected), makeGuard(&expected), makeEncode(&expected))
        if got != want || err != wantErr || actual != expected { t.Fatal(got, err, actual, want, wantErr, expected) }
        actual, expected = "", ""
        err = g.Notify(makeLoad(&actual), func() error { actual += "N"; return outputErr }, func(){ actual += "F" })
        wantErr = r.Notify(makeLoad(&expected), func() error { expected += "N"; return outputErr }, func(){ expected += "F" })
        if err != wantErr || actual != expected { t.Fatal(err, actual, wantErr, expected) }
      }
      for _, on := range []bool{true, false} {
        count := 0
        got, err := g.Async(on, func() (int,error) { count++; return input, inputErr })
        want, wantErr := r.Async(on, func() (int,error) { return input, inputErr })
        if got != want || err != wantErr || count != 1 { t.Fatal(got, err, count, want, wantErr) }
      }
    }
  }
  if got, err := g.Arrow(0); got != 1 || err != nil { t.Fatal(got, err) }
  if got, err := g.Forward(true, func() (*g.Box,error) { return nil, failure }, nilFallback); got != nil || err != failure { t.Fatal(got, err) }
  box := g.NewBox(4)
  if got, err := g.Forward(false, nilFallback, func() (*g.Box,error) { return box, nil }); got != box || err != nil { t.Fatal(got, err) }
  cleanup := 0
  if got := capturePanic(func() { g.Notify(func() (int,error) { panic("load panic") }, func() error { return nil }, func() { cleanup++ }) }); got != "load panic" || cleanup != 1 { t.Fatal(got, cleanup) }
  panicText := fmt.Sprint(capturePanic(func() { g.Forward(true, func() (*g.Box,error) { return nil, nil }, nilFallback) }))
  if !strings.Contains(panicText, "non-null result") { t.Fatal(panicText) }
}
func nilFallback() (*g.Box,error) { panic("unselected arm") }
func capturePanic(run func()) (result any) { defer func(){ result = recover() }(); run(); return nil }
`
	runGeneratedGoDifferentialTest(t, root, "match-return.test", generated, reference, comparison)
}
