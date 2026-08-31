package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeVariadicDeclarationsMatchIndependentGo(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "variadic.otm")
	if err := os.WriteFile(source, []byte(`
function Sum(prefix: int, ...values: int[]): int {
  let total = prefix;
  for (const value of values) { total += value; }
  return total;
}
function Last<T>(fallback: T, ...values: T[]): T {
  if (len(values) === 0) { return fallback; }
  return values[len(values) - 1];
}
interface Joiner { function join(prefix: string, ...parts: string[]): string; }
class Words implements Joiner {
  public function join(prefix: string, ...parts: string[]): string {
    let result = prefix;
    for (const part of parts) { result += part; }
    return result;
  }
}
class Base {
  public virtual function render(prefix: string, ...parts: string[]): string { return prefix; }
}
class Derived extends Base {
  public override function render(prefix: string, ...parts: string[]): string { return prefix + parts[0]; }
}
class Batch {
  constructor(public ...values: int[]) {}
  public function total(): int { return Sum(0, this.values...); }
}
struct Accumulator {
  public base: int;
  public function sum(...values: int[]): int { return Sum(this.base, values...); }
}
type Scores = distinct int[];
public function sum(this: Scores, ...values: int[]): int { return Sum(len(this), values...); }
function Apply(fn: (...values: int[]) => int, values: int[]): int { return fn(values...); }
function Arrow(values: int[]): int {
  const sum: (...values: int[]) => int = (...items: int[]): int => Sum(0, items...);
  return Apply(sum, values);
}
function BatchTotal(values: int[]): int { return new Batch(values...).total(); }
function StructTotal(base: int, values: int[]): int { return Accumulator{ base: base }.sum(values...); }
function ScoresTotal(values: int[], tail: int[]): int { return Scores(values).sum(tail...); }
function VirtualRender(prefix: string, parts: string[]): string {
  const value: Base = new Derived();
  return value.render(prefix, parts...);
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "variadic")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"func Sum(prefix int, values ...int) int",
		"func Last[T any](fallback T, values ...T) T",
		"Join(prefix string, parts ...string) string",
		"func (this *Words) Join(prefix string, parts ...string) string",
		"func NewBatch(values ...int) *Batch",
		"func (this Accumulator) Sum(values ...int) int",
		"func (this Scores) Sum(values ...int) int",
		"func Apply(fn func(...int) int, values []int) int",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

func Sum(prefix int, values ...int) int { total := prefix; for _, value := range values { total += value }; return total }
func Last[T any](fallback T, values ...T) T { if len(values) == 0 { return fallback }; return values[len(values)-1] }
type Joiner interface { Join(string, ...string) string }
type Words struct{}
func (*Words) Join(prefix string, parts ...string) string { result := prefix; for _, part := range parts { result += part }; return result }
type Base struct{ self interface{ render(string, ...string) string } }
func (value *Base) render(prefix string, parts ...string) string { if value.self != nil { return value.self.render(prefix, parts...) }; return prefix }
type Derived struct{ Base }
func NewDerived() *Derived { value := &Derived{}; value.self = value; return value }
func (value *Derived) render(prefix string, parts ...string) string { return prefix + parts[0] }
type Batch struct{ Values []int }
func NewBatch(values ...int) *Batch { return &Batch{Values: values} }
func (value *Batch) Total() int { return Sum(0, value.Values...) }
type Accumulator struct{ Base int }
func (value Accumulator) Sum(values ...int) int { return Sum(value.Base, values...) }
type Scores []int
func (value Scores) Sum(values ...int) int { return Sum(len(value), values...) }
func Apply(fn func(...int) int, values []int) int { return fn(values...) }
func Arrow(values []int) int { sum := func(items ...int) int { return Sum(0, items...) }; return Apply(sum, values) }
func BatchTotal(values []int) int { return NewBatch(values...).Total() }
func StructTotal(base int, values []int) int { return Accumulator{Base: base}.Sum(values...) }
func ScoresTotal(values, tail []int) int { return Scores(values).Sum(tail...) }
func VirtualRender(prefix string, parts []string) string { value := NewDerived(); return value.render(prefix, parts...) }
`
	testSource := `package variadic_test

import (
  "testing"
  generated "variadic.test"
  reference "variadic.test/reference"
)

type generatedJoiner interface { Join(string, ...string) string }
type referenceJoiner interface { Join(string, ...string) string }

func TestVariadicBehavior(t *testing.T) {
  for _, item := range []struct { prefix int; values []int }{{0, nil}, {-3, []int{1}}, {7, []int{-2, 4, 9}}} {
    if got, want := generated.Sum(item.prefix, item.values...), reference.Sum(item.prefix, item.values...); got != want { t.Errorf("Sum(%d, %v) = %d, Go = %d", item.prefix, item.values, got, want) }
    if got, want := generated.Last(item.prefix, item.values...), reference.Last(item.prefix, item.values...); got != want { t.Errorf("Last(%d, %v) = %d, Go = %d", item.prefix, item.values, got, want) }
    if got, want := generated.Arrow(item.values), reference.Arrow(item.values); got != want { t.Errorf("Arrow(%v) = %d, Go = %d", item.values, got, want) }
    if got, want := generated.BatchTotal(item.values), reference.BatchTotal(item.values); got != want { t.Errorf("BatchTotal(%v) = %d, Go = %d", item.values, got, want) }
    if got, want := generated.StructTotal(item.prefix, item.values), reference.StructTotal(item.prefix, item.values); got != want { t.Errorf("StructTotal(%d, %v) = %d, Go = %d", item.prefix, item.values, got, want) }
    if got, want := generated.ScoresTotal(item.values, []int{5, -1}), reference.ScoresTotal(item.values, []int{5, -1}); got != want { t.Errorf("ScoresTotal(%v) = %d, Go = %d", item.values, got, want) }
  }
  var gotJoiner generatedJoiner = &generated.Words{}
  var wantJoiner referenceJoiner = &reference.Words{}
  if got, want := gotJoiner.Join("a", "b", "温泉"), wantJoiner.Join("a", "b", "温泉"); got != want { t.Errorf("Join = %q, Go = %q", got, want) }
  for _, item := range []struct { prefix string; parts []string }{{"", []string{"x"}}, {"onsen", []string{"tamago"}}, {"温泉", []string{"卵"}}} {
    if got, want := generated.VirtualRender(item.prefix, item.parts), reference.VirtualRender(item.prefix, item.parts); got != want { t.Errorf("VirtualRender(%q, %v) = %q, Go = %q", item.prefix, item.parts, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "variadic.test", generated, referenceSource, testSource)
}
