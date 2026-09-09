package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceTypeSetConstraintsMatchIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source_type_set.km")
	kinmokuseiSource := `
constraint Integer = ~int | ~int8 | ~uint16;
constraint ExactString = string;

type Score = distinct int;
type Samples<T extends Integer> = distinct T[];

struct Pair<T extends Integer> {
  public left: T;
  public right: T;
  public function total(): T { return this.left + this.right; }
}

class Counter<T extends Integer> {
  constructor(public value: T) {}
  public function add(delta: T): T {
    this.value += delta;
    return this.value;
  }
}

function transform<T extends Integer>(left: T, right: T): T {
  let value = left + right;
  value ^= left;
  return value;
}
function join<T extends ExactString>(left: T, right: T): T { return left + right; }
function ScoreTransform(left: int, right: int): int { return int(transform(Score(left), Score(right))); }
function Int8Transform(left: int8, right: int8): int8 { return transform(left, right); }
function Uint16Transform(left: uint16, right: uint16): uint16 { return transform(left, right); }
function PairTotal(left: int, right: int): int { return Pair<int> { left: left, right: right }.total(); }
function CounterAfter(value: int8, first: int8, second: int8): int8 {
  const counter = new Counter<int8>(value);
  counter.add(first);
  return counter.add(second);
}
function SampleCount(values: int[]): int { return len(Samples<int>(values)); }
function Joined(left: string, right: string): string { return join(left, right); }
`
	if err := os.WriteFile(source, []byte(kinmokuseiSource), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "sourcetypeset")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"type Integer interface",
		"~int | ~int8 | ~uint16",
		"type ExactString interface",
		"func transform[T Integer]",
		"type Pair[T Integer] struct",
		"type Counter[T Integer] struct",
		"type Samples[T Integer] []T",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference
type Integer interface { ~int | ~int8 | ~uint16 }
type ExactString interface { string }
type Score int
type Samples[T Integer] []T
type Pair[T Integer] struct { Left, Right T }
func (value Pair[T]) Total() T { return value.Left + value.Right }
type Counter[T Integer] struct { Value T }
func (value *Counter[T]) Add(delta T) T { value.Value += delta; return value.Value }
func transform[T Integer](left, right T) T { value := left + right; value ^= left; return value }
func join[T ExactString](left, right T) T { return left + right }
func ScoreTransform(left, right int) int { return int(transform(Score(left), Score(right))) }
func Int8Transform(left, right int8) int8 { return transform(left, right) }
func Uint16Transform(left, right uint16) uint16 { return transform(left, right) }
func PairTotal(left, right int) int { return Pair[int]{Left: left, Right: right}.Total() }
func CounterAfter(value, first, second int8) int8 { counter := &Counter[int8]{Value: value}; counter.Add(first); return counter.Add(second) }
func SampleCount(values []int) int { return len(Samples[int](values)) }
func Joined(left, right string) string { return join(left, right) }
`
	testSource := `package sourcetypeset_test
import (
  "testing"
  generated "sourcetypeset.test"
  reference "sourcetypeset.test/reference"
)
func TestBehavior(t *testing.T) {
  for _, item := range []struct{ left, right int }{{0, 0}, {-7, 3}, {5, 9}, {100, -25}} {
    if got, want := generated.ScoreTransform(item.left, item.right), reference.ScoreTransform(item.left, item.right); got != want { t.Errorf("ScoreTransform(%v) = %d, Go = %d", item, got, want) }
    if got, want := generated.PairTotal(item.left, item.right), reference.PairTotal(item.left, item.right); got != want { t.Errorf("PairTotal(%v) = %d, Go = %d", item, got, want) }
  }
  for _, item := range []struct{ left, right int8 }{{0, 0}, {-7, 3}, {120, 20}, {-128, -1}} {
    if got, want := generated.Int8Transform(item.left, item.right), reference.Int8Transform(item.left, item.right); got != want { t.Errorf("Int8Transform(%v) = %d, Go = %d", item, got, want) }
    if got, want := generated.CounterAfter(item.left, item.right, 3), reference.CounterAfter(item.left, item.right, 3); got != want { t.Errorf("CounterAfter(%v) = %d, Go = %d", item, got, want) }
  }
  for _, item := range []struct{ left, right uint16 }{{0, 0}, {1, 2}, {65535, 1}, {40000, 30000}} {
    if got, want := generated.Uint16Transform(item.left, item.right), reference.Uint16Transform(item.left, item.right); got != want { t.Errorf("Uint16Transform(%v) = %d, Go = %d", item, got, want) }
  }
  for _, values := range [][]int{nil, {}, {0}, {-1, 2, 3}} {
    if got, want := generated.SampleCount(values), reference.SampleCount(values); got != want { t.Errorf("SampleCount(%v) = %d, Go = %d", values, got, want) }
  }
  for _, item := range []struct{ left, right string }{{"", ""}, {"kin", "mokusei"}, {"金", "木犀"}} {
    if got, want := generated.Joined(item.left, item.right), reference.Joined(item.left, item.right); got != want { t.Errorf("Joined(%q, %q) = %q, Go = %q", item.left, item.right, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "sourcetypeset.test", generated, referenceSource, testSource)
}

func TestImportedSourceTypeSetConstraintMatchesIndependentGo(t *testing.T) {
	root := t.TempDir()
	constraintSource := filepath.Join(root, "numeric.km")
	entrySource := filepath.Join(root, "main.km")
	if err := os.WriteFile(constraintSource, []byte(`constraint Integer = ~int | ~int16;`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entrySource, []byte(`
import { Integer } from "./numeric";
function Doubled<T extends Integer>(value: T): T { return value + value; }
function DoubleInt(value: int): int { return Doubled(value); }
function DoubleInt16(value: int16): int16 { return Doubled(value); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entrySource}, "sourcetypesetlinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "~int | ~int16") || !strings.Contains(string(generated), "value + value") {
		t.Fatalf("generated linked constraint is incomplete:\n%s", generated)
	}
	referenceSource := `package reference
type Integer interface { ~int | ~int16 }
func doubled[T Integer](value T) T { return value + value }
func DoubleInt(value int) int { return doubled(value) }
func DoubleInt16(value int16) int16 { return doubled(value) }
`
	testSource := `package sourcetypesetlinked_test
import (
  "testing"
  generated "sourcetypeset-linked.test"
  reference "sourcetypeset-linked.test/reference"
)
func TestBehavior(t *testing.T) {
  for _, value := range []int{0, 1, -7, 1000} {
    if got, want := generated.DoubleInt(value), reference.DoubleInt(value); got != want { t.Errorf("DoubleInt(%d) = %d, Go = %d", value, got, want) }
  }
  for _, value := range []int16{0, 1, -7, 20000, 32767} {
    if got, want := generated.DoubleInt16(value), reference.DoubleInt16(value); got != want { t.Errorf("DoubleInt16(%d) = %d, Go = %d", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "sourcetypeset-linked.test", generated, referenceSource, testSource)
}
