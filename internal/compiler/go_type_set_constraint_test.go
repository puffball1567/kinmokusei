package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoTypeSetConstraintsMatchIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "go_type_set_constraint.km")
	if err := os.WriteFile(source, []byte(`
import go cmp from "cmp";

type Score = distinct int;
type OrderedLookup<T extends cmp.Ordered> = distinct Map<T, string>;
alias Pair<T extends cmp.Ordered> = [2]T;

function Add<T extends cmp.Ordered>(left: T, right: T): T { return left + right; }
function Clamp<T extends cmp.Ordered>(value: T, low: T, high: T): T {
  if (value < low) { return low; }
  if (value > high) { return high; }
  return value;
}

struct Interval<T extends cmp.Ordered> {
  public low: T;
  public high: T;
  public function contains(value: T): boolean {
    return value >= this.low && value <= this.high;
  }
}

interface Chooser<T extends cmp.Ordered> { function choose(left: T, right: T): T; }
class IntChooser implements Chooser<int> {
  public function choose(left: int, right: int): int {
    if (left < right) { return left; }
    return right;
  }
}

class Box<T extends cmp.Ordered> {
  constructor(public value: T) {}
  public function maximum(other: T): T {
    if (this.value > other) { return this.value; }
    return other;
  }
}

function AddInt(left: int, right: int): int { return Add(left, right); }
function AddString(left: string, right: string): string { return Add(left, right); }
function ClampScore(value: int, low: int, high: int): int {
  return int(Clamp(Score(value), Score(low), Score(high)));
}
function IntervalContains(low: int, high: int, value: int): boolean {
  const interval = Interval<int> { low: low, high: high };
  return interval.contains(value);
}
function ChooseInt(left: int, right: int): int {
  const chooser: Chooser<int> = new IntChooser();
  return chooser.choose(left, right);
}
function BoxMaximum(value: string, other: string): string {
  return new Box<string>(value).maximum(other);
}
function LookupSize(values: Map<string, string>): int {
  return len(OrderedLookup<string>(values));
}
function PairFirst(left: string, right: string): string {
  const pair: Pair<string> = [left, right];
  return pair[0];
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "gotypesetconstraint")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		`func Add[T cmp.Ordered]`,
		`func Clamp[T cmp.Ordered]`,
		`type Interval[T cmp.Ordered] struct`,
		`type Chooser[T cmp.Ordered] interface`,
		`type Box[T cmp.Ordered] struct`,
		`cmp.Ordered`,
		`comparable`,
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference
import "cmp"
type Score int
type OrderedLookup[T cmp.Ordered] map[T]string
type Pair[T cmp.Ordered] [2]T
func Add[T cmp.Ordered](left, right T) T { return left + right }
func Clamp[T cmp.Ordered](value, low, high T) T {
  if value < low { return low }
  if value > high { return high }
  return value
}
type Interval[T cmp.Ordered] struct { Low, High T }
func (value Interval[T]) Contains(item T) bool { return item >= value.Low && item <= value.High }
type Chooser[T cmp.Ordered] interface { Choose(T, T) T }
type IntChooser struct{}
func (*IntChooser) Choose(left, right int) int { if left < right { return left }; return right }
type Box[T cmp.Ordered] struct { Value T }
func (box *Box[T]) Maximum(other T) T { if box.Value > other { return box.Value }; return other }
func AddInt(left, right int) int { return Add(left, right) }
func AddString(left, right string) string { return Add(left, right) }
func ClampScore(value, low, high int) int { return int(Clamp(Score(value), Score(low), Score(high))) }
func IntervalContains(low, high, value int) bool { return (Interval[int]{Low: low, High: high}).Contains(value) }
func ChooseInt(left, right int) int { var chooser Chooser[int] = &IntChooser{}; return chooser.Choose(left, right) }
func BoxMaximum(value, other string) string { return (&Box[string]{Value: value}).Maximum(other) }
func LookupSize(values map[string]string) int { return len(OrderedLookup[string](values)) }
func PairFirst(left, right string) string { pair := Pair[string]{left, right}; return pair[0] }
`
	testSource := `package gotypesetconstraint_test
import (
  "testing"
  generated "gotypesetconstraint.test"
  reference "gotypesetconstraint.test/reference"
)
func TestBehavior(t *testing.T) {
  for _, item := range []struct{ left, right int }{{0, 0}, {-9, 4}, {41, 1}} {
    if got, want := generated.AddInt(item.left, item.right), reference.AddInt(item.left, item.right); got != want { t.Errorf("AddInt(%d, %d) = %d, Go = %d", item.left, item.right, got, want) }
    if got, want := generated.Add(item.left, item.right), reference.Add(item.left, item.right); got != want { t.Errorf("public Add(%d, %d) = %d, Go = %d", item.left, item.right, got, want) }
    if got, want := generated.ChooseInt(item.left, item.right), reference.ChooseInt(item.left, item.right); got != want { t.Errorf("ChooseInt(%d, %d) = %d, Go = %d", item.left, item.right, got, want) }
    var gotChooser generated.Chooser[int] = generated.NewIntChooser()
    var wantChooser reference.Chooser[int] = &reference.IntChooser{}
    if got, want := gotChooser.Choose(item.left, item.right), wantChooser.Choose(item.left, item.right); got != want { t.Errorf("public Chooser(%d, %d) = %d, Go = %d", item.left, item.right, got, want) }
  }
  for _, item := range []struct{ value, low, high int }{{0, 0, 0}, {-10, -2, 4}, {9, -2, 4}, {2, -2, 4}} {
    if got, want := generated.ClampScore(item.value, item.low, item.high), reference.ClampScore(item.value, item.low, item.high); got != want { t.Errorf("ClampScore(%v) = %d, Go = %d", item, got, want) }
    if got, want := generated.IntervalContains(item.low, item.high, item.value), reference.IntervalContains(item.low, item.high, item.value); got != want { t.Errorf("IntervalContains(%v) = %v, Go = %v", item, got, want) }
    gotInterval := generated.Interval[int]{Low: item.low, High: item.high}
    wantInterval := reference.Interval[int]{Low: item.low, High: item.high}
    if got, want := gotInterval.Contains(item.value), wantInterval.Contains(item.value); got != want { t.Errorf("public Interval(%v) = %v, Go = %v", item, got, want) }
  }
  for _, item := range []struct{ left, right string }{{"", ""}, {"on", "sen"}, {"温泉", "卵"}} {
    if got, want := generated.AddString(item.left, item.right), reference.AddString(item.left, item.right); got != want { t.Errorf("AddString(%q, %q) = %q, Go = %q", item.left, item.right, got, want) }
    if got, want := generated.BoxMaximum(item.left, item.right), reference.BoxMaximum(item.left, item.right); got != want { t.Errorf("BoxMaximum(%q, %q) = %q, Go = %q", item.left, item.right, got, want) }
    if got, want := generated.NewBox(item.left).Maximum(item.right), (&reference.Box[string]{Value: item.left}).Maximum(item.right); got != want { t.Errorf("public Box(%q, %q) = %q, Go = %q", item.left, item.right, got, want) }
    if got, want := generated.PairFirst(item.left, item.right), reference.PairFirst(item.left, item.right); got != want { t.Errorf("PairFirst(%q, %q) = %q, Go = %q", item.left, item.right, got, want) }
  }
  for _, values := range []map[string]string{nil, {}, {"": ""}, {"a": "1", "b": "2"}} {
    if got, want := generated.LookupSize(values), reference.LookupSize(values); got != want { t.Errorf("LookupSize(%v) = %d, Go = %d", values, got, want) }
    if got, want := len(generated.OrderedLookup[string](values)), len(reference.OrderedLookup[string](values)); got != want { t.Errorf("public OrderedLookup(%v) = %d, Go = %d", values, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "gotypesetconstraint.test", generated, referenceSource, testSource)
}

func TestErasedGenericAliasConstraintDoesNotLeaveUnusedGoImport(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "alias_constraint.km")
	if err := os.WriteFile(source, []byte(`
import go cmp from "cmp";
alias Pair<T extends cmp.Ordered> = [2]T;
function First(values: Pair<int>): int { return values[0]; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "aliasconstraint")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), `"cmp"`) {
		t.Fatalf("erased alias retained unused constraint import:\n%s", generated)
	}
	if !strings.Contains(string(generated), `func First(values [2]int) int`) {
		t.Fatalf("erased alias signature missing:\n%s", generated)
	}
}
