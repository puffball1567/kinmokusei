package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericAliasesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_alias.km")
	if err := os.WriteFile(source, []byte(`
import go atomic from "sync/atomic";
alias Identity<T> = T;
alias Values<T> = T[];
alias Lookup<K, V> = Map<K, V>;
alias Pair<T> = [2]T;
alias Rows<T> = Values<T>[];
alias Ref<T> = *T;
alias Transform<T, U> = (value: T) => U;
alias AtomicRef<T> = atomic.Pointer<T>;
type Tagged<T> = distinct T[];
alias TaggedAlias<T> = Tagged<T>;
interface Reader<T> { function read(): T; }
alias ReaderAlias<T> = Reader<T>;
class TextReader implements Reader<string> {
  constructor(private value: string) {}
  public function read(): string { return this.value; }
}
class Box<T> { constructor(public value: T) {} }
alias BoxRef<T> = Box<T>;

function GenericIdentity<T>(value: Identity<T>): Identity<T> { return Identity<T>(value); }
function ValuesBehavior(values: Values<int>, tail: int): int {
  let converted = Values<int>(values);
  converted = append(converted, tail);
  converted[0] += 2;
  return len(converted) * 100 + converted[0] * 10 + converted[len(converted) - 1];
}
function LookupBehavior(values: Lookup<string, int>, key: string, value: int): int {
  values[key] = value;
  return len(values) * 100 + values[key];
}
function PairBehavior(value: Pair<int>): int {
  value[1] += 1;
  const table: Lookup<Pair<int>, string> = makeMap<Pair<int>, string>();
  table[value] = "present";
  return value[0] * 100 + value[1] * 10 + len(table[value]);
}
function RowsBehavior(values: Rows<int>): int { return len(values) * 100 + len(values[0]); }
function RefBehavior(value: Ref<int>): int { *value += 2; return *value; }
function Apply(transform: Transform<int, string>, value: int): string { return transform(value); }
function AtomicRoundTrip(cell: *AtomicRef<int>, value: *int): *int { cell.Store(value); return cell.Load(); }
function MakeTagged(values: int[]): TaggedAlias<int> { return TaggedAlias<int>(values); }
function TaggedSize(values: TaggedAlias<int>): int { return len(values); }
function Read(value: ReaderAlias<string>): string { return value.read(); }
function MakeReader(value: string): TextReader { return new TextReader(value); }
function BoxValue(value: BoxRef<string>): string { return value.value; }
function Present(values: Values<int>): Result<Values<int>> { return ok(values); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericalias")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, unexpected := range []string{"type Identity[", "type Values[", "type Lookup[", "type Pair[", "type Rows[", "type Ref[", "type Transform[", "type AtomicRef[", "type TaggedAlias[", "type ReaderAlias[", "type BoxRef["} {
		if strings.Contains(string(generated), unexpected) {
			t.Errorf("generated Go retained generic alias %q:\n%s", unexpected, generated)
		}
	}
	for _, expected := range []string{
		"func GenericIdentity[T any](value T) T",
		"func ValuesBehavior(values []int, tail int) int",
		"func LookupBehavior(values map[string]int, key string, value int) int",
		"func PairBehavior(value [2]int) int",
		"func RowsBehavior(values [][]int) int",
		"func RefBehavior(value *int) int",
		"func Apply(transform func(int) string, value int) string",
		"func AtomicRoundTrip(cell *atomic.Pointer[int], value *int) *int",
		"func MakeTagged(values []int) Tagged[int]",
		"func TaggedSize(values Tagged[int]) int",
		"func Read(value Reader[string]) string",
		"func BoxValue(value *Box[string]) string",
		"func Present(values []int) ([]int, error)",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

import "sync/atomic"

type Reader[T any] interface { Read() T }
type TextReader struct { value string }
func NewTextReader(value string) *TextReader { return &TextReader{value: value} }
func (reader *TextReader) Read() string { return reader.value }
type Box[T any] struct { Value T }
func NewBox[T any](value T) *Box[T] { return &Box[T]{Value: value} }
type Tagged[T any] []T

func GenericIdentity[T any](value T) T { return T(value) }
func ValuesBehavior(values []int, tail int) int { converted := []int(values); converted = append(converted, tail); converted[0] += 2; return len(converted)*100 + converted[0]*10 + converted[len(converted)-1] }
func LookupBehavior(values map[string]int, key string, value int) int { values[key] = value; return len(values)*100 + values[key] }
func PairBehavior(value [2]int) int { value[1]++; table := make(map[[2]int]string); table[value] = "present"; return value[0]*100 + value[1]*10 + len(table[value]) }
func RowsBehavior(values [][]int) int { return len(values)*100 + len(values[0]) }
func RefBehavior(value *int) int { *value += 2; return *value }
func Apply(transform func(int) string, value int) string { return transform(value) }
func AtomicRoundTrip(cell *atomic.Pointer[int], value *int) *int { cell.Store(value); return cell.Load() }
func MakeTagged(values []int) Tagged[int] { return Tagged[int](values) }
func TaggedSize(values Tagged[int]) int { return len(values) }
func Read(value Reader[string]) string { return value.Read() }
func MakeReader(value string) *TextReader { return NewTextReader(value) }
func BoxValue(value *Box[string]) string { return value.Value }
func Present(values []int) ([]int, error) { return values, nil }
`
	testSource := `package genericalias_test

import (
  "reflect"
  "strconv"
  "sync/atomic"
  "testing"
  generated "genericalias.test"
  reference "genericalias.test/reference"
)

func TestGenericAliasBehavior(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := generated.GenericIdentity(value), reference.GenericIdentity(value); got != want { t.Errorf("GenericIdentity(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.Read(generated.MakeReader(value)), reference.Read(reference.MakeReader(value)); got != want { t.Errorf("Read(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.BoxValue(generated.NewBox(value)), reference.BoxValue(reference.NewBox(value)); got != want { t.Errorf("BoxValue(%q) = %q, Go = %q", value, got, want) }
  }
  for _, item := range []struct { values []int; tail int }{{[]int{0}, 0}, {[]int{-3, 4}, 9}, {[]int{11}, -7}} {
    gotValues, wantValues := append([]int(nil), item.values...), append([]int(nil), item.values...)
    if got, want := generated.ValuesBehavior(gotValues, item.tail), reference.ValuesBehavior(wantValues, item.tail); got != want || !reflect.DeepEqual(gotValues, wantValues) { t.Errorf("ValuesBehavior(%v, %d) = (%d, %v), Go = (%d, %v)", item.values, item.tail, got, gotValues, want, wantValues) }
    gotPresent, gotErr := generated.Present(gotValues)
    wantPresent, wantErr := reference.Present(wantValues)
    if !reflect.DeepEqual(gotPresent, wantPresent) || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present = (%v, %v), Go = (%v, %v)", gotPresent, gotErr, wantPresent, wantErr) }
  }
  for _, item := range []struct { key string; value int }{{"", 0}, {"onsen", -4}, {"温泉卵", 7}} {
    gotMap, wantMap := map[string]int{"existing": 1}, map[string]int{"existing": 1}
    if got, want := generated.LookupBehavior(gotMap, item.key, item.value), reference.LookupBehavior(wantMap, item.key, item.value); got != want || !reflect.DeepEqual(gotMap, wantMap) { t.Errorf("LookupBehavior(%q, %d) = (%d, %v), Go = (%d, %v)", item.key, item.value, got, gotMap, want, wantMap) }
  }
  for _, value := range [][2]int{{0, 0}, {-2, 5}, {9, -3}} {
    if got, want := generated.PairBehavior(value), reference.PairBehavior(value); got != want { t.Errorf("PairBehavior(%v) = %d, Go = %d", value, got, want) }
  }
  for _, rows := range [][][]int{{{}}, {{1, 2}, {3}}, {{-1}}} {
    if got, want := generated.RowsBehavior(rows), reference.RowsBehavior(rows); got != want { t.Errorf("RowsBehavior(%v) = %d, Go = %d", rows, got, want) }
  }
  for _, value := range []int{-3, 0, 9} {
    gotValue, wantValue := value, value
    if got, want := generated.RefBehavior(&gotValue), reference.RefBehavior(&wantValue); got != want || gotValue != wantValue { t.Errorf("RefBehavior(%d) = (%d, %d), Go = (%d, %d)", value, got, gotValue, want, wantValue) }
    if got, want := generated.Apply(strconv.Itoa, value), reference.Apply(strconv.Itoa, value); got != want { t.Errorf("Apply(%d) = %q, Go = %q", value, got, want) }
    var gotCell, wantCell atomic.Pointer[int]
    gotAtomicValue, wantAtomicValue := value, value
    gotPointer, wantPointer := generated.AtomicRoundTrip(&gotCell, &gotAtomicValue), reference.AtomicRoundTrip(&wantCell, &wantAtomicValue)
    if *gotPointer != *wantPointer || gotPointer != &gotAtomicValue || wantPointer != &wantAtomicValue { t.Errorf("AtomicRoundTrip(%d) did not preserve value and identity", value) }
    gotTagged, wantTagged := generated.MakeTagged([]int{value, value + 1}), reference.MakeTagged([]int{value, value + 1})
    if got, want := generated.TaggedSize(gotTagged), reference.TaggedSize(wantTagged); got != want || !reflect.DeepEqual([]int(gotTagged), []int(wantTagged)) { t.Errorf("Tagged(%d) = (%d, %v), Go = (%d, %v)", value, got, gotTagged, want, wantTagged) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericalias.test", generated, referenceSource, testSource)
}

func TestLinkedGenericAliasesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "values.km")
	entry := filepath.Join(temporary, "entry.km")
	if err := os.WriteFile(dependency, []byte(`alias Values<T> = T[]; function wrap<T>(values: T[]): Values<T> { return Values<T>(values); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Values, wrap } from "./values"; function linked(values: Values<string>): string { return wrap(values)[0]; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "genericaliaslinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), "type Values[") || !strings.Contains(string(generated), "func linked(values []string) string") {
		t.Fatalf("linked generated Go did not erase the generic alias:\n%s", generated)
	}
	referenceSource := `package reference
func wrap[T any](values []T) []T { return []T(values) }
func Linked(values []string) string { return wrap(values)[0] }
`
	testSource := `package genericaliaslinked
import (
  "testing"
  reference "genericalias-linked.test/reference"
)
func TestLinked(t *testing.T) {
  for _, values := range [][]string{{"onsen"}, {"温泉", "卵"}} {
    if got, want := linked(values), reference.Linked(values); got != want { t.Errorf("linked(%v) = %q, Go = %q", values, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericalias-linked.test", generated, referenceSource, testSource)
}
