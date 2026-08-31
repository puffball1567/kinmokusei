package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericDefinedTypesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_defined_type.otm")
	if err := os.WriteFile(source, []byte(`
type Values<T> = distinct T[];
type Lookup<K, V> = distinct Map<K, V>;
type Pair<T> = distinct [2]T;
type Rows<T> = distinct Values<T>[];
type Wrapped<T> = distinct Values<T>;

public function size<U>(this: Values<U>): int { return len(this); }
public function push<U>(this: *Values<U>, value: U): void { *this = append(*this, value); }
public function first<A, B>(this: Lookup<A, B>, key: A): B { return this[key]; }

function ValuesBehavior(values: int[], tail: int): int {
  let typed = Values<int>(values);
  typed = append(typed, tail);
  typed[0] += 2;
  return len(typed) * 100 + typed[0] * 10 + typed[len(typed) - 1];
}
function LookupBehavior(values: Map<string, int>, key: string, value: int): int {
  const typed = Lookup<string, int>(values);
  typed[key] = value;
  return len(typed) * 100 + typed[key];
}
function PairBehavior(values: [2]int): int {
  let typed = Pair<int>(values);
  typed[1] += 1;
  const lookup: Map<Pair<int>, string> = makeMap<Pair<int>, string>();
  lookup[typed] = "present";
  return typed[0] * 100 + typed[1] * 10 + len(lookup[typed]);
}
function Wrap<T>(values: T[]): Values<T> { return Values<T>(values); }
function RowsBehavior(values: Values<int>[]): int {
  const rows = Rows<int>(values);
  return len(rows) * 100 + len(rows[0]);
}
function WrappedBehavior(values: int[]): int {
  const wrapped = Wrapped<int>(Values<int>(values));
  return len(wrapped);
}
function Present(values: int[]): Result<Values<int>> { return ok(Values<int>(values)); }
function WordCount(values: string[]): int { return len(Values<string>(values)); }
function MethodBehavior(values: int[], tail: int): int {
  let typed = Values<int>(values);
  const size = typed.size;
  typed.push(tail);
  return size() * 100 + typed.size() * 10 + typed[len(typed) - 1];
}
function LookupMethodBehavior(values: Map<string, int>, key: string): int {
  const typed = Lookup<string, int>(values);
  return typed.first(key);
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericdefined")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"type Values[T any] []T",
		"type Lookup[K comparable, V any] map[K]V",
		"type Pair[T any] [2]T",
		"type Rows[T any] []Values[T]",
		"type Wrapped[T any] Values[T]",
		"func Wrap[T any](values []T) Values[T]",
		"func (this Values[U]) Size() int",
		"func (this *Values[U]) Push(value U)",
		"func (this Lookup[A, B]) First(key A) B",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Values[T any] []T
type Lookup[K comparable, V any] map[K]V
type Pair[T any] [2]T
type Rows[T any] []Values[T]
type Wrapped[T any] Values[T]
func (values Values[U]) Size() int { return len(values) }
func (values *Values[U]) Push(value U) { *values = append(*values, value) }
func (values Lookup[A, B]) First(key A) B { return values[key] }

func ValuesBehavior(values []int, tail int) int { typed := Values[int](values); typed = append(typed, tail); typed[0] += 2; return len(typed)*100 + typed[0]*10 + typed[len(typed)-1] }
func LookupBehavior(values map[string]int, key string, value int) int { typed := Lookup[string, int](values); typed[key] = value; return len(typed)*100 + typed[key] }
func PairBehavior(values [2]int) int { typed := Pair[int](values); typed[1]++; lookup := make(map[Pair[int]]string); lookup[typed] = "present"; return typed[0]*100 + typed[1]*10 + len(lookup[typed]) }
func Wrap[T any](values []T) Values[T] { return Values[T](values) }
func RowsBehavior(values []Values[int]) int { rows := Rows[int](values); return len(rows)*100 + len(rows[0]) }
func WrappedBehavior(values []int) int { wrapped := Wrapped[int](Values[int](values)); return len(wrapped) }
func Present(values []int) (Values[int], error) { return Values[int](values), nil }
func WordCount(values []string) int { return len(Values[string](values)) }
func MethodBehavior(values []int, tail int) int { typed := Values[int](values); size := typed.Size; typed.Push(tail); return size()*100 + typed.Size()*10 + typed[len(typed)-1] }
func LookupMethodBehavior(values map[string]int, key string) int { typed := Lookup[string, int](values); return typed.First(key) }
`
	testSource := `package genericdefined_test

import (
  "reflect"
  "testing"
  generated "genericdefined.test"
  reference "genericdefined.test/reference"
)

type generatedSizer interface { Size() int }
type referenceSizer interface { Size() int }
type generatedPusher interface { Push(int) }
type referencePusher interface { Push(int) }

func TestGenericDefinedTypeBehavior(t *testing.T) {
  for _, item := range []struct { values []int; tail int }{{[]int{0}, 0}, {[]int{-3, 4}, 9}, {[]int{11}, -7}} {
    gotValues, wantValues := append([]int(nil), item.values...), append([]int(nil), item.values...)
    if got, want := generated.ValuesBehavior(gotValues, item.tail), reference.ValuesBehavior(wantValues, item.tail); got != want || !reflect.DeepEqual(gotValues, wantValues) { t.Errorf("ValuesBehavior(%v, %d) = (%d, %v), Go = (%d, %v)", item.values, item.tail, got, gotValues, want, wantValues) }
    gotMethodValues, wantMethodValues := append([]int(nil), item.values...), append([]int(nil), item.values...)
    if got, want := generated.MethodBehavior(gotMethodValues, item.tail), reference.MethodBehavior(wantMethodValues, item.tail); got != want || !reflect.DeepEqual(gotMethodValues, wantMethodValues) { t.Errorf("MethodBehavior(%v, %d) = (%d, %v), Go = (%d, %v)", item.values, item.tail, got, gotMethodValues, want, wantMethodValues) }
    gotTyped, wantTyped := generated.Values[int](append([]int(nil), item.values...)), reference.Values[int](append([]int(nil), item.values...))
    var gotSizer generatedSizer = gotTyped
    var wantSizer referenceSizer = wantTyped
    if got, want := gotSizer.Size(), wantSizer.Size(); got != want { t.Errorf("external value method set = %d, Go = %d", got, want) }
    var gotPusher generatedPusher = &gotTyped
    var wantPusher referencePusher = &wantTyped
    gotPusher.Push(item.tail); wantPusher.Push(item.tail)
    if !reflect.DeepEqual([]int(gotTyped), []int(wantTyped)) { t.Errorf("external pointer method set = %v, Go = %v", gotTyped, wantTyped) }
  }
  for _, item := range []struct { key string; value int }{{"", 0}, {"onsen", -4}, {"温泉卵", 7}} {
    gotMap, wantMap := map[string]int{"existing": 1}, map[string]int{"existing": 1}
    if got, want := generated.LookupBehavior(gotMap, item.key, item.value), reference.LookupBehavior(wantMap, item.key, item.value); got != want || !reflect.DeepEqual(gotMap, wantMap) { t.Errorf("LookupBehavior(%q, %d) = (%d, %v), Go = (%d, %v)", item.key, item.value, got, gotMap, want, wantMap) }
    if got, want := generated.LookupMethodBehavior(gotMap, item.key), reference.LookupMethodBehavior(wantMap, item.key); got != want { t.Errorf("LookupMethodBehavior(%q) = %d, Go = %d", item.key, got, want) }
  }
  for _, values := range [][2]int{{0, 0}, {-2, 5}, {9, -3}} {
    if got, want := generated.PairBehavior(values), reference.PairBehavior(values); got != want { t.Errorf("PairBehavior(%v) = %d, Go = %d", values, got, want) }
  }
  for _, values := range [][]int{{}, {1}, {-2, 3, 9}} {
    if got, want := []int(generated.Wrap(values)), []int(reference.Wrap(values)); !reflect.DeepEqual(got, want) { t.Errorf("Wrap(%v) = %v, Go = %v", values, got, want) }
    gotResult, gotErr := generated.Present(values)
    wantResult, wantErr := reference.Present(values)
    if !reflect.DeepEqual([]int(gotResult), []int(wantResult)) || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present(%v) = (%v, %v), Go = (%v, %v)", values, gotResult, gotErr, wantResult, wantErr) }
    if got, want := generated.WrappedBehavior(values), reference.WrappedBehavior(values); got != want { t.Errorf("WrappedBehavior(%v) = %d, Go = %d", values, got, want) }
  }
  gotRows := []generated.Values[int]{{1, 2}, {3}}
  wantRows := []reference.Values[int]{{1, 2}, {3}}
  if got, want := generated.RowsBehavior(gotRows), reference.RowsBehavior(wantRows); got != want { t.Errorf("RowsBehavior = %d, Go = %d", got, want) }
  for _, words := range [][]string{{}, {"onsen"}, {"温泉", "卵"}} {
    if got, want := generated.WordCount(words), reference.WordCount(words); got != want { t.Errorf("WordCount(%v) = %d, Go = %d", words, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericdefined.test", generated, referenceSource, testSource)
}

func TestLinkedGenericDefinedTypeMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "values.otm")
	entry := filepath.Join(temporary, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`type Values<T> = distinct T[]; public function size<U>(this: Values<U>): int { return len(this); } function wrap<T>(values: T[]): Values<T> { return Values<T>(values); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Values, wrap } from "./values"; function linked(values: string[]): int { const typed: Values<string> = wrap(values); return typed.size(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "genericdefinedlinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
type Values[T any] []T
func (values Values[U]) Size() int { return len(values) }
func wrap[T any](values []T) Values[T] { return Values[T](values) }
func Linked(values []string) int { return wrap(values).Size() }
`
	testSource := `package genericdefinedlinked
import (
  "testing"
  reference "genericdefined-linked.test/reference"
)
func TestLinked(t *testing.T) {
  for _, values := range [][]string{{}, {"onsen"}, {"温泉", "卵"}} {
    if got, want := linked(values), reference.Linked(values); got != want { t.Errorf("linked(%v) = %v, Go = %v", values, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericdefined-linked.test", generated, referenceSource, testSource)
}
