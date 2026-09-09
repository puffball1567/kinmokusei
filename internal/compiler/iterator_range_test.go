package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIteratorRangesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"sequence.km": `
class Sequence<T> {
  constructor(protected items: T[]) {}
  public virtual function iterate(yield: (value: T) => boolean): void {
    for (const value of this.items) { if (!yield(value)) { return; } }
  }
}
class Doubled extends Sequence<int> {
  constructor(items: int[]) { super(items); }
  public override function iterate(yield: (value: int) => boolean): void {
    for (const value of this.items) { if (!yield(value * 2)) { return; } }
  }
  public function original(yield: (value: int) => boolean): void { super.iterate(yield); }
}
function values(items: int[]): (yield: (value: int) => boolean) => void {
  return (yield: (value: int) => boolean): void => {
    for (const value of items) { if (!yield(value)) { return; } }
  };
}
`,
		"entry.km": `
import { Sequence, Doubled, values } from "./sequence";
import go slices from "slices";
alias Callback = () => int;
class Leaf { constructor(public value: int) {} }
function OOP(items: int[]): int {
  const derived = new Doubled(items);
  const base: Sequence<int> = derived;
  let sum = 0;
  for (const value of base.iterate) { sum += value; }
  for (const value of derived.original) { sum += value; }
  return sum;
}
function Leaves(value: int): boolean {
  const leaf = new Leaf(value);
  const sequence = new Sequence<Leaf>([leaf]);
  for (const item of sequence.iterate) { if (item !== leaf || item.value !== value) { return false; } }
  return true;
}
function Standard(items: int[]): int {
  let sum = 0;
  for (const value of slices.Values(items)) { sum += value; }
  for (const [index, value] of slices.All(items)) { sum += index * value; }
  for (const value of slices.All(items)) { sum += value; }
  for (const [index, _] of slices.All(items)) { sum += index; }
  return sum;
}
function Control(items: int[]): int {
  let result = 0;
  for (const value of values(items)) { if (value === 0) { continue; } if (value < 0) { break; } result = result * 10 + value; }
  return result;
}
function Fresh(): int {
  let callbacks: Callback[] = [];
  for (let value of values([1, 2, 3])) { callbacks = append(callbacks, (): int => value); value += 1; }
  let result = 0;
  for (const callback of callbacks) { result = result * 10 + callback(); }
  return result;
}
function ticks(yield: () => boolean): void { for (const _ of 3) { if (!yield()) { return; } } }
function Zero(): int { let count = 0; for (const _ of ticks) { count++; } return count; }
function digit(log: *int, value: int): void { *log = *log * 10 + value; }
function returning(log: *int): int {
  for (const value of values([1, 2, 3])) { defer digit(log, value); if (value === 2) { return value; } }
  return 0;
}
function ReturnAndDefer(): int { let log = 0; const result = returning(&log); return result * 100 + log; }
function Labels(): int {
  let result = 0;
  outer: for (const value of values([1, 2, 3])) {
    for (const inner of values([4, 5])) { result += value * inner; continue outer; }
  }
  for (const value of values([1, 2])) { result += value; goto done; }
  done: return result;
}
function observed(calls: *int): (yield: (value: int) => boolean) => void { *calls += 1; return values([1,2]); }
function Once(): int {
  let calls = 0; let sum = 0;
  for (const unused of observed(&calls)) { sum++; }
  for (const _ of observed(&calls)) { sum++; }
  return calls * 100 + sum;
}
function bad(yield: (value: int) => boolean): void { yield(1); yield(2); }
function ProtocolPanic(): void { for (const value of bad) { break; } }
function NilPanic(iterator: (yield: (value: int) => boolean) => void): void { for (const value of iterator) {} }
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "iteratorrange")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "slices"
func values(items []int) func(func(int) bool) { return func(yield func(int) bool) { for _, value := range items { if !yield(value) { return } } } }
type Sequence[T any] struct { items []T }
func (s *Sequence[T]) Iterate(yield func(T) bool) { for _, value := range s.items { if !yield(value) { return } } }
type Doubled struct { Sequence[int] }
func (d *Doubled) Iterate(yield func(int) bool) { for _, value := range d.items { if !yield(value*2) { return } } }
func OOP(items []int) int { d := &Doubled{Sequence[int]{items}}; var base interface{ Iterate(func(int) bool) } = d; sum := 0; for value := range base.Iterate { sum += value }; for value := range d.Sequence.Iterate { sum += value }; return sum }
func Leaves(value int) bool { leaf := &struct { value int }{value}; sequence := &Sequence[*struct { value int }]{[]*struct { value int }{leaf}}; for item := range sequence.Iterate { if item != leaf || item.value != value { return false } }; return true }
func Standard(items []int) int { sum := 0; for value := range slices.Values(items) { sum += value }; for index, value := range slices.All(items) { sum += index*value }; for _, value := range slices.All(items) { sum += value }; for index := range slices.All(items) { sum += index }; return sum }
func Control(items []int) int { result := 0; for value := range values(items) { if value == 0 { continue }; if value < 0 { break }; result = result*10 + value }; return result }
func Fresh() int { var callbacks []func() int; for value := range values([]int{1,2,3}) { callbacks = append(callbacks, func() int { return value }); value++ }; result := 0; for _, callback := range callbacks { result = result*10 + callback() }; return result }
func ticks(yield func() bool) { for range 3 { if !yield() { return } } }
func Zero() int { count := 0; for range ticks { count++ }; return count }
func digit(log *int, value int) { *log = *log*10 + value }
func returning(log *int) int { for value := range values([]int{1,2,3}) { defer digit(log,value); if value == 2 { return value } }; return 0 }
func ReturnAndDefer() int { log := 0; result := returning(&log); return result*100 + log }
func Labels() int { result := 0; outer: for value := range values([]int{1,2,3}) { for inner := range values([]int{4,5}) { result += value*inner; continue outer } }; for value := range values([]int{1,2}) { result += value; goto done }; done: return result }
func observed(calls *int) func(func(int) bool) { *calls++; return values([]int{1,2}) }
func Once() int { calls, sum := 0, 0; for range observed(&calls) { sum++ }; for range observed(&calls) { sum++ }; return calls*100 + sum }
func bad(yield func(int) bool) { yield(1); yield(2) }
func ProtocolPanic() { for range bad { break } }
func NilPanic(iterator func(func(int) bool)) { for range iterator {} }
`
	comparison := `package iteratorrange_test
import (
 "testing"
 generated "iterator-range.test"
 reference "iterator-range.test/reference"
)
func panics(call func()) (result bool) { defer func() { result = recover() != nil }(); call(); return }
func TestIterators(t *testing.T) {
 for _, items := range [][]int{nil,{}, {0}, {1,2,3}, {2,0,3,-1,9}, {-1,2}} {
  for _, pair := range [][2]int{{generated.OOP(items), reference.OOP(items)}, {generated.Standard(items), reference.Standard(items)}, {generated.Control(items), reference.Control(items)}} { if pair[0] != pair[1] { t.Errorf("got %d want %d", pair[0], pair[1]) } }
 }
 for _, value := range []int{-1,0,42} { if generated.Leaves(value) != reference.Leaves(value) { t.Error("class element identity") } }
 for _, pair := range [][2]int{{generated.Fresh(), reference.Fresh()}, {generated.Zero(), reference.Zero()}, {generated.ReturnAndDefer(), reference.ReturnAndDefer()}, {generated.Labels(), reference.Labels()}, {generated.Once(), reference.Once()}} { if pair[0] != pair[1] { t.Errorf("got %d want %d", pair[0], pair[1]) } }
 if panics(generated.ProtocolPanic) != panics(reference.ProtocolPanic) { t.Error("yield protocol panic") }
 if panics(func(){generated.NilPanic(nil)}) != panics(func(){reference.NilPanic(nil)}) { t.Error("nil iterator panic") }
}
`
	runGeneratedGoDifferentialTest(t, root, "iterator-range.test", generated, reference, comparison)
}
