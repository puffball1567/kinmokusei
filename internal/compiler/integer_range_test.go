package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegerRangeMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "integer_range.km")
	input := `
type Count = distinct uint16;
alias Callback = () => int;
constraint IntegerCount = ~int;
function Generic<T extends IntegerCount>(n: T): T { let total: T = 0; for (const i of n) { total += i; } return total; }
function Sum(n: int): int { let sum = 0; for (const i of n) { sum += i; } return sum; }
function Narrow(n: Count): Count { let sum = Count(0); for (const i of n) { sum += i; } return sum; }
function Closures(): int {
  let callbacks: Callback[] = [];
  for (const i of 5) { callbacks = append(callbacks, (): int => i); }
  let total = 0;
  for (const callback of callbacks) { total = total * 10 + callback(); }
  return total;
}
function observed(count: *int): int { *count += 1; return 5; }
function Once(): int {
  let calls = 0; let sum = 0;
  for (const i of observed(&calls)) { if (i === 1) { continue; } if (i === 4) { break; } sum += i; }
  for (const _ of observed(&calls)) { sum++; }
  for (const unused of observed(&calls)) { sum++; }
  for (let assigned of 2) { assigned = 9; }
  return calls * 100 + sum;
}
function ChangingBound(): int {
  let n = 5; let count = 0;
  for (let i of n) { n = 0; i = 99; count++; }
  return count;
}
function Labels(): int {
  let total = 0;
  outer: for (const i of 4) {
    for (const j of 3) { if (j === 1) { continue outer; } total += i; }
  }
  return total;
}
class Leaf { constructor(public value: int) {} }
class Holder { public leaf: Leaf; constructor() { for (const _ of 2) { this.leaf = new Leaf(7); } } }
function Construct(): int { return new Holder().leaf.value; }
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "integerrange")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "for i := range n") || strings.Contains(string(generated), "for _, i := range n") {
		t.Fatalf("integer lowering:\n%s", generated)
	}
	reference := `package reference
type Count uint16
func Sum(n int) int { sum := 0; for i := range n { sum += i }; return sum }
func Generic[T ~int](n T) T { total := T(0); for i := range n { total += i }; return total }
func Narrow(n Count) Count { sum := Count(0); for i := range n { sum += i }; return sum }
func Closures() int { var callbacks []func() int; for i := range 5 { callbacks = append(callbacks, func() int { return i }) }; total := 0; for _, callback := range callbacks { total = total*10 + callback() }; return total }
func Once() int { calls, sum := 0, 0; observed := func() int { calls++; return 5 }; for i := range observed() { if i == 1 { continue }; if i == 4 { break }; sum += i }; for range observed() { sum++ }; for range observed() { sum++ }; for assigned := range 2 { assigned = 9; _ = assigned }; return calls*100 + sum }
func ChangingBound() int { n, count := 5, 0; for i := range n { n = 0; i = 99; _ = i; count++ }; return count }
func Labels() int { total := 0; outer: for i := range 4 { for j := range 3 { if j == 1 { continue outer }; total += i } }; return total }
func Construct() int { var leaf *struct { value int }; for range 2 { leaf = &struct { value int }{7} }; return leaf.value }
`
	comparison := `package integerrange_test
import (
 "testing"
 generated "integer-range.test"
 reference "integer-range.test/reference"
)
func TestRanges(t *testing.T) {
 for _, n := range []int{-5, 0, 1, 2, 20} { if got, want := generated.Sum(n), reference.Sum(n); got != want { t.Errorf("Sum(%d)=%d want %d", n, got, want) }; if generated.Generic(n) != reference.Generic(n) { t.Error("generic integer range") } }
 for _, n := range []uint16{0, 1, 5, 255, 1000} { if got, want := generated.Narrow(generated.Count(n)), reference.Narrow(reference.Count(n)); uint16(got) != uint16(want) { t.Errorf("Narrow(%d)=%d want %d", n, got, want) } }
 for _, pair := range [][2]int{{generated.Closures(), reference.Closures()}, {generated.Once(), reference.Once()}, {generated.ChangingBound(), reference.ChangingBound()}, {generated.Labels(), reference.Labels()}, {generated.Construct(), reference.Construct()}} { if pair[0] != pair[1] { t.Errorf("got %d want %d", pair[0], pair[1]) } }
}
`
	runGeneratedGoDifferentialTest(t, root, "integer-range.test", generated, reference, comparison)
}
