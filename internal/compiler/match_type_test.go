package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTypedMatchMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"classes.km": `export class Base {}
export class Child extends Base { constructor(public n: int) { super(); } }
export alias Selected = Child
`,
		"entry.km": `import { Base, Child, Selected } from "./classes"
import go bytes from "bytes"
alias Value = interface {}
alias MaybeValue = Value | null

export function Pick(load: () => MaybeValue, permit: (n: int) => boolean): int {
  return match (load()) {
    case const n as int if (permit(n)) => n + 100
    case const n as int => n
    case const text as bstring => len(text)
    case const _ as *bytes.Buffer => -2
    case nil => -3
    default => -4
  }
}
export function Callable(input: MaybeValue): int {
  return match (input) { case const f as () => int => f(), default => -1 }
}
export function Pair(load: () => Result<MaybeValue[]>): int {
  return match (load()) {
    case ok([const a as int, const b as int]) => a - b
    case ok([const text as bstring, ..._]) => len(text)
    case ok(_) => 0
    case fail(_) => -1
  }
}
export function Classify(mode: int): int {
  let value: Base | null = null
  if (mode == 1) { value = new Child(7) }
  if (mode == 2) { value = new Base() }
  return match (value) { case const child as Selected => child.n, case null => -1, default => -2 }
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "typedmatch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import "bytes"
func Pick(load func() any, permit func(int) bool) int {
  switch value := load().(type) {
  case int:
    if permit(value) { return value + 100 }
    return value
  case string: return len(value)
  case *bytes.Buffer: return -2
  case nil: return -3
  default: return -4
  }
}
func Callable(input any) int {
  if f, ok := input.(func() int); ok && f != nil { return f() }
  return -1
}
func Pair(values []any, err error) int {
  if err != nil { return -1 }
  if len(values) == 2 {
    a, aok := values[0].(int)
    b, bok := values[1].(int)
    if aok && bok { return a - b }
  }
  if len(values) != 0 {
    if text, ok := values[0].(string); ok { return len(text) }
  }
  return 0
}
func Classify(mode int) int {
  switch mode { case 1: return 7; case 2: return -2; default: return -1 }
}
`
	comparison := `package typedmatch_test
import (
  "bytes"
  "errors"
  "testing"
  g "typed-match.test"
  r "typed-match.test/reference"
)
func TestMatch(t *testing.T) {
  for _, value := range []any{nil, 12, false, "月", (*bytes.Buffer)(nil), new(bytes.Buffer), (func() int)(nil), func() int { return 7 }} {
    for _, permit := range []bool{false, true} {
      steps, expectedSteps := 0, 0
      got := g.Pick(func() any { steps++; return value }, func(n int) bool { steps += 10; return permit })
      want := r.Pick(func() any { expectedSteps++; return value }, func(n int) bool { expectedSteps += 10; return permit })
      if got != want || steps != expectedSteps { t.Fatal(got, want, steps, expectedSteps) }
    }
    if got, want := g.Callable(value), r.Callable(value); got != want { t.Fatal(got, want) }
  }
  for _, values := range [][]any{nil, {}, {7}, {7, 2}, {7, nil}, {"a"}, {"abc", true}, {false, 2, 5}} {
    for _, failure := range []error{nil, errors.New("failed")} {
      if got, want := g.Pair(func() ([]any,error) { return values, failure }), r.Pair(values, failure); got != want { t.Fatal(values, got, want) }
    }
  }
  for mode := 0; mode < 4; mode++ {
    if got, want := g.Classify(mode), r.Classify(mode); got != want { t.Fatal(got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "typed-match.test", generated, reference, comparison)
}
