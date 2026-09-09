package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericMethodsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "generic_method.km")
	kinmokuseiSource := `
constraint Integer = ~int | ~int8;

class Box<T> {
  constructor(public value: T) {}
  public function choose<U>(value: U): U { return value; }
  public function keep<U>(marker: U): T { return this.value; }
  public static function pair<U>(left: U, right: T): {left: U, right: T} {
    return {left: left, right: right};
  }
}

class Base<T> {
  constructor(protected value: T) {}
  public function echo<U>(value: U): U { return value; }
}
class Child<T> extends Base<T> {
  constructor(value: T) { super(value); }
  public function relay<U>(value: U): U { return super.echo(value); }
}

struct Counter<T extends Integer> {
  public value: T;
  public pointer function update<U extends Integer>(delta: T, marker: U): U {
    this.value += delta;
    return marker;
  }
  public function project<U>(value: U): U { return value; }
}

class Utility {
  public function first<T>(fallback: T, ...values: T[]): Result<T> {
    if (len(values) === 0) { return ok(fallback); }
    return ok(values[0]);
  }
}

let utilityCreations: int = 0;
function makeUtility(): Utility {
  utilityCreations++;
  return new Utility();
}

function ClassChoose(value: string): string {
  const box = new Box<string>(value);
  return box.choose(value) + box.keep<int>(1);
}
function StaticPair(left: int, right: string): string {
  const pair = Box.pair<int>(left, right);
  return pair.right;
}
function InheritedEcho(value: string): string {
  const child = new Child<int>(3);
  return child.relay(child.echo(value));
}
function CounterAfter(value: int, delta: int, marker: int8): int {
  let counter = Counter<int> { value: value };
  const returned = counter.update<int8>(delta, marker);
  return counter.project(counter.value) + int(returned);
}
function FirstString(fallback: string, values: string[]): Result<string> {
  return new Utility().first<string>(fallback, values...);
}
function FactoryFirstString(fallback: string, values: string[]): Result<string> {
  utilityCreations = 0;
  return makeUtility().first<string>(fallback, values...);
}
function UtilityCreationCount(): int {
  return utilityCreations;
}
`
	if err := os.WriteFile(source, []byte(kinmokuseiSource), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericmethod")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"func BoxChoose[U any, T any]",
		"func BoxKeep[U any, T any]",
		"func CounterUpdate[U Integer, T Integer]",
		"func CounterProject[U any, T Integer]",
		"__kinmokuseiUpcastChildToBase[int]",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference
type Box[T any] struct { value T }
func boxChoose[T, U any](box *Box[T], value U) U { return value }
func boxKeep[T, U any](box *Box[T], marker U) T { return box.value }
func boxPair[T, U any](left U, right T) struct{ left U; right T } { return struct{ left U; right T }{left, right} }
type Base[T any] struct { value T }
func baseEcho[T, U any](base *Base[T], value U) U { return value }
type Child[T any] struct { Base[T] }
func childRelay[T, U any](child *Child[T], value U) U { return baseEcho(&child.Base, value) }
type Integer interface { ~int | ~int8 }
type Counter[T Integer] struct { value T }
func counterUpdate[T Integer, U Integer](counter *Counter[T], delta T, marker U) U { counter.value += delta; return marker }
func counterProject[T Integer, U any](counter Counter[T], value U) U { return value }
func first[T any](fallback T, values ...T) (T, error) { if len(values) == 0 { return fallback, nil }; return values[0], nil }
var utilityCreations int
func makeUtility() struct{} { utilityCreations++; return struct{}{} }
func ClassChoose(value string) string { box := &Box[string]{value}; return boxChoose(box, value) + boxKeep(box, 1) }
func StaticPair(left int, right string) string { return boxPair(left, right).right }
func InheritedEcho(value string) string { child := &Child[int]{Base[int]{3}}; return childRelay(child, baseEcho(&child.Base, value)) }
func CounterAfter(value, delta int, marker int8) int { counter := Counter[int]{value}; returned := counterUpdate(&counter, delta, marker); return counterProject(counter, counter.value) + int(returned) }
func FirstString(fallback string, values []string) (string, error) { return first(fallback, values...) }
func FactoryFirstString(fallback string, values []string) (string, error) { utilityCreations = 0; _ = makeUtility(); return first(fallback, values...) }
func UtilityCreationCount() int { return utilityCreations }
`
	testSource := `package genericmethod_test
import (
  "testing"
  generated "genericmethod.test"
  reference "genericmethod.test/reference"
)
func TestBehavior(t *testing.T) {
	box := generated.NewBox[string]("direct")
	if got := generated.BoxChoose[int](box, 9); got != 9 { t.Errorf("BoxChoose public Go helper = %d", got) }
	if got := generated.BoxChoose[int, string](nil, 11); got != 11 { t.Errorf("BoxChoose nil receiver helper = %d", got) }
	counter := generated.Counter[int]{Value: 4}
	if got := generated.CounterUpdate[int8](&counter, 3, 2); got != 2 || counter.Value != 7 { t.Errorf("CounterUpdate public Go helper = (%d, %d)", got, counter.Value) }
  for _, value := range []string{"", "kinmokusei", "金木犀"} {
    if got, want := generated.ClassChoose(value), reference.ClassChoose(value); got != want { t.Errorf("ClassChoose(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.InheritedEcho(value), reference.InheritedEcho(value); got != want { t.Errorf("InheritedEcho(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.StaticPair(len(value), value), reference.StaticPair(len(value), value); got != want { t.Errorf("StaticPair(%q) = %q, Go = %q", value, got, want) }
	for _, values := range [][]string{nil, {}, {value}, {"first", "second"}} {
	  got, gotErr := generated.FirstString(value, values)
	  want, wantErr := reference.FirstString(value, values)
	  if got != want || (gotErr == nil) != (wantErr == nil) { t.Errorf("FirstString(%q, %v) = (%q, %v), Go = (%q, %v)", value, values, got, gotErr, want, wantErr) }
	  got, gotErr = generated.FactoryFirstString(value, values)
	  want, wantErr = reference.FactoryFirstString(value, values)
	  if got != want || (gotErr == nil) != (wantErr == nil) { t.Errorf("FactoryFirstString(%q, %v) = (%q, %v), Go = (%q, %v)", value, values, got, gotErr, want, wantErr) }
	  if gotCount, wantCount := generated.UtilityCreationCount(), reference.UtilityCreationCount(); gotCount != wantCount || gotCount != 1 { t.Errorf("factory receiver count = %d, Go = %d, want 1", gotCount, wantCount) }
	}
  }
  for _, item := range []struct{ value, delta int; marker int8 }{{0, 0, 0}, {7, -2, 3}, {-9, 20, -5}, {100, -100, 127}} {
    if got, want := generated.CounterAfter(item.value, item.delta, item.marker), reference.CounterAfter(item.value, item.delta, item.marker); got != want { t.Errorf("CounterAfter(%v) = %d, Go = %d", item, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "genericmethod.test", generated, referenceSource, testSource)
}

func TestImportedGenericMethodsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dependency := filepath.Join(root, "box.km")
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(dependency, []byte(`
constraint Integer = ~int | ~int8;
class Box<T> {
  constructor(public value: T) {}
  public function choose<U>(value: U): U { return value; }
  public function add<U extends Integer>(left: U, right: U): U { return left + right; }
}
function makeBox<T>(value: T): Box<T> { return new Box<T>(value); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import { Box, makeBox } from "./box";
function LinkedText(value: string): string {
  const box: Box<int> = makeBox<int>(1);
  return box.choose(value);
}
function LinkedInt8(left: int8, right: int8): int8 {
  const box = makeBox<string>("unused");
  return box.add<int8>(left, right);
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "genericmethodlinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
func LinkedText(value string) string { return value }
func LinkedInt8(left, right int8) int8 { return left + right }
`
	testSource := `package genericmethodlinked_test
import (
  "testing"
  generated "genericmethod-linked.test"
  reference "genericmethod-linked.test/reference"
)
func TestBehavior(t *testing.T) {
  for _, value := range []string{"", "linked", "桂花"} {
    if got, want := generated.LinkedText(value), reference.LinkedText(value); got != want { t.Errorf("LinkedText(%q) = %q, Go = %q", value, got, want) }
  }
  for _, item := range [][2]int8{{0, 0}, {1, 2}, {120, 20}, {-128, -1}} {
    if got, want := generated.LinkedInt8(item[0], item[1]), reference.LinkedInt8(item[0], item[1]); got != want { t.Errorf("LinkedInt8(%v) = %d, Go = %d", item, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "genericmethod-linked.test", generated, referenceSource, testSource)
}
