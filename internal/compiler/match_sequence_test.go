package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSequenceRestDiscardEmitsNoCopy(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "discard.km")
	input := `function f(values: int[]): boolean {
  return match(values) { case [_, ..._] => true, case [] => false }
}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "discard")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("__kinmokuseiCloneMatchRest")) {
		t.Fatalf("ignored rest emitted a clone helper:\n%s", generated)
	}
}

func TestSequenceRestMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "sequence.km")
	input := `import go net from "net"

export function Trim(values: int[]): int[] {
  return match (values) {
    case [1, ...const middle, 3] => middle
    case [...const prefix, 3] => prefix
    case [1, ...const tail] => tail
    case [...const all] => all
  }
}
export function Ends(load: () => int[], guard: (middle: int[]) => boolean): int {
  return match (load()) {
    case [const first, ...const middle, const last] if (guard(middle)) => first + last
    case [const first, ..._, const last] => first - last
    case [const only] => only
    case [] => 0
  }
}
export function Fixed(values: [4]int): int[] {
  return match (values) { case [_, ...const middle, _] => middle }
}
export function IPBytes(values: net.IP): byte[] {
  return match (values) { case [...const bytes] => bytes }
}
constraint Sequence<E> = ~E[]
export function Generic<E, S extends Sequence<E>>(values: S): E[] {
  return match (values) { case [...const all] => all }
}
export function Nested(load: () => Result<int[][]>): int {
  return match (load()) {
    case ok([]) => 0
    case ok([[const first, ..._], ..._]) => first
    case ok([_, ..._]) => -2
    case fail(_) => -1
  }
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "sequencematch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
func Clone[E any](values []E) []E { return append(make([]E, 0, len(values)), values...) }
func Trim(values []int) []int {
  low, high := 0, len(values)
  if high > 0 && values[high-1] == 3 { high-- }
  if high > 0 && values[0] == 1 { low++ }
  return Clone(values[low:high])
}
func Ends(load func() []int, guard func([]int) bool) int {
  values := load()
  switch len(values) {
  case 0: return 0
  case 1: return values[0]
  default:
    first, last := values[0], values[len(values)-1]
    if guard(Clone(values[1:len(values)-1])) { return first + last }
    return first - last
  }
}
func Nested(values [][]int, err error) int {
  if err != nil { return -1 }
  if len(values) == 0 { return 0 }
  if len(values[0]) == 0 { return -2 }
  return values[0][0]
}
`
	comparison := `package sequencematch_test
import (
  "errors"
  "net"
  "reflect"
  "testing"
  g "sequence-rest.test"
  r "sequence-rest.test/reference"
)
type scores []int
func TestMatch(t *testing.T) {
  for _, values := range [][]int{nil, {}, {1}, {3}, {1,3}, {2,3}, {1,4,3}, {8,7,6,5}} {
    original := append([]int(nil), values...)
    before := len(values)
    got, want := g.Trim(values), r.Trim(values)
    if !reflect.DeepEqual(got, want) || cap(got) != len(got) { t.Fatalf("trim %v = %v, want %v", values, got, want) }
    if len(got) > 0 { got[0]++ }
    if len(values) != before || !reflect.DeepEqual(append([]int(nil), values...), original) { t.Fatal("rest aliases the subject") }
    for _, accept := range []bool{true, false} {
      gotSteps, wantSteps := 0, 0
      got := g.Ends(func() []int { gotSteps++; return values }, func(mid []int) bool {
        gotSteps += 10
        if len(mid) != 0 { mid[0] = 99 }
        return accept
      })
      want := r.Ends(func() []int { wantSteps++; return original }, func(mid []int) bool { wantSteps += 10; return accept })
      if got != want || gotSteps != wantSteps || !reflect.DeepEqual(append([]int(nil), values...), original) {
        t.Fatalf("guard on %v = %d/%d, want %d/%d", values, got, gotSteps, want, wantSteps)
      }
    }
  }
  if got := g.Fixed([4]int{1,2,3,4}); !reflect.DeepEqual(got, []int{2,3}) { t.Fatal(got) }
  ip := net.IP{1,2,3,4}
  if got := g.IPBytes(ip); !reflect.DeepEqual(got, []byte(ip)) { t.Fatal(got) }
  if got := g.Generic[int](scores{1,2}); !reflect.DeepEqual(got, []int{1,2}) { t.Fatal(got) }
  inner := []int{5}
  values := [][]int{inner}
  copied := g.Generic[[]int](values)
  copied[0][0] = 6
  copied[0] = nil
  if values[0][0] != 6 { t.Fatal("rest did not preserve nested slice identity") }
  for _, values := range [][][]int{nil, {nil}, {{7}, {8}}} {
    for _, err := range []error{nil, errors.New("failed")} {
      if got, want := g.Nested(func() ([][]int,error) { return values,err }), r.Nested(values,err); got != want { t.Fatal(got,want) }
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "sequence-rest.test", generated, reference, comparison)
}
