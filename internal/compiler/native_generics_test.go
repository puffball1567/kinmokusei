package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeGenericFunctionsMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "native_generics.otm")
	input := `
function identity<T>(value: T): T { return value; }
function second<T, U>(left: T, right: U): U { return right; }
function choose<T>(left: T, right: T): T { return left; }
function first<T>(items: T[]): T { return items[0]; }
function present<T>(value: T): Result<T> { return ok(value); }
function dereference<T>(value: *T): T { return *value; }
function lookup<T>(values: Map<string, T>): T { return values["value"]; }
function unwrap<T>(holder: { value: T }): T { return holder.value; }
function apply<T>(value: T, transform: (item: T) => T): T { return transform(value); }
function relay<T>(value: T): T { return identity(value); }

struct Point { public x: int; }
class Box {}

let orderState: int = 0;
function mark(value: int): int { orderState = orderState * 10 + value; return value; }
function inferredInt(value: int): int { return identity(value); }
function explicitString(value: string): string { return identity<string>(value); }
function bracketString(value: string): string { return identity[string](value); }
function partialSecond(value: string): string { return second<int>(1, value); }
function inferredFirst(items: int[]): int { return first(items); }
function ordered(): int { orderState = 0; const value = second(mark(1), mark(2)); return orderState * 10 + value; }
function copiedPoint(value: int): int { let source = Point{ x: value }; let copied = identity(source); copied.x += 1; return source.x * 100 + copied.x; }
function samePoint(point: *Point): boolean { return identity(point) === point; }
function resultValue(value: string): Result<string> { return present(value); }
function chosen(left: int, right: int): int { return choose(left, right); }
function dereferencedPoint(value: int): int { let point = Point{ x: value }; let copied = dereference(&point); copied.x += 1; return point.x * 100 + copied.x; }
function mapped(value: string): string { const values = makeMap[string, string](); values["value"] = value; return lookup(values); }
function unwrapped(value: string): string { const holder: { value: string } = { value: value }; return unwrap(holder); }
function applied(value: int): int { return apply(value, (item: int): int => item * 2); }
function relayed(value: string): string { return relay(value); }
function nullableMissing(value: Box | null): boolean { return identity(value) === null; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "nativegenerics")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"func identity[T any](value T) T",
		"func second[T any, U any](left T, right U) U",
		"identity[string](value)",
		"second[int](1, value)",
		"func present[T any](value T) (T, error)",
		"func dereference[T any](value *T) T",
		"func lookup[T any](values map[string]T) T",
		"func unwrap[T any](holder struct {",
		"Value T `json:\"value\"`",
		"func apply[T any](value T, transform func(T) T) T",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Point struct { X int }

var orderState int

func identity[T any](value T) T { return value }
func second[T, U any](left T, right U) U { return right }
func choose[T any](left, right T) T { return left }
func first[T any](items []T) T { return items[0] }
func present[T any](value T) (T, error) { return value, nil }
func dereference[T any](value *T) T { return *value }
func lookup[T any](values map[string]T) T { return values["value"] }
func unwrap[T any](holder struct { Value T }) T { return holder.Value }
func apply[T any](value T, transform func(T) T) T { return transform(value) }
func relay[T any](value T) T { return identity(value) }
func mark(value int) int { orderState = orderState*10 + value; return value }

func InferredInt(value int) int { return identity(value) }
func ExplicitString(value string) string { return identity[string](value) }
func BracketString(value string) string { return identity[string](value) }
func PartialSecond(value string) string { return second[int](1, value) }
func InferredFirst(items []int) int { return first(items) }
func Ordered() int { orderState = 0; value := second(mark(1), mark(2)); return orderState*10 + value }
func CopiedPoint(value int) int { source := Point{X: value}; copied := identity(source); copied.X++; return source.X*100 + copied.X }
func SamePoint(point *Point) bool { return identity(point) == point }
func ResultValue(value string) (string, error) { return present(value) }
func Chosen(left, right int) int { return choose(left, right) }
func First(items []int) int { return first(items) }
func DereferencedPoint(value int) int { point := Point{X: value}; copied := dereference(&point); copied.X++; return point.X*100 + copied.X }
func Mapped(value string) string { return lookup(map[string]string{"value": value}) }
func Unwrapped(value string) string { return unwrap(struct { Value string }{Value: value}) }
func Applied(value int) int { return apply(value, func(item int) int { return item*2 }) }
func Relayed(value string) string { return relay(value) }
func NullableMissing(value *struct{}) bool { return identity(value) == nil }
`
	testSource := `package nativegenerics

import (
  "testing"
  reference "nativegenerics.test/reference"
)

func panics(operation func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  operation()
  return false
}

func TestNativeGenerics(t *testing.T) {
  for _, value := range []int{-1, 0, 1, 42} {
    if got, want := inferredInt(value), reference.InferredInt(value); got != want { t.Errorf("inferredInt(%d) = %d, Go = %d", value, got, want) }
  }
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := explicitString(value), reference.ExplicitString(value); got != want { t.Errorf("explicitString(%q) = %q, Go = %q", value, got, want) }
    if got, want := bracketString(value), reference.BracketString(value); got != want { t.Errorf("bracketString(%q) = %q, Go = %q", value, got, want) }
    if got, want := partialSecond(value), reference.PartialSecond(value); got != want { t.Errorf("partialSecond(%q) = %q, Go = %q", value, got, want) }
    if got, want := mapped(value), reference.Mapped(value); got != want { t.Errorf("mapped(%q) = %q, Go = %q", value, got, want) }
    if got, want := unwrapped(value), reference.Unwrapped(value); got != want { t.Errorf("unwrapped(%q) = %q, Go = %q", value, got, want) }
    if got, want := relayed(value), reference.Relayed(value); got != want { t.Errorf("relayed(%q) = %q, Go = %q", value, got, want) }
    got, gotErr := resultValue(value)
    want, wantErr := reference.ResultValue(value)
    if got != want || (gotErr == nil) != (wantErr == nil) { t.Errorf("resultValue(%q) = (%q, %v), Go = (%q, %v)", value, got, gotErr, want, wantErr) }
  }
  for _, items := range [][]int{{1}, {-1, 2}, {42, 0, -3}} {
    if got, want := inferredFirst(items), reference.InferredFirst(items); got != want { t.Errorf("inferredFirst(%v) = %d, Go = %d", items, got, want) }
  }
  if got, want := ordered(), reference.Ordered(); got != want { t.Errorf("ordered = %d, Go = %d", got, want) }
  for _, value := range []int{-1, 0, 7} {
    if got, want := copiedPoint(value), reference.CopiedPoint(value); got != want { t.Errorf("copiedPoint(%d) = %d, Go = %d", value, got, want) }
    if got, want := dereferencedPoint(value), reference.DereferencedPoint(value); got != want { t.Errorf("dereferencedPoint(%d) = %d, Go = %d", value, got, want) }
    if got, want := applied(value), reference.Applied(value); got != want { t.Errorf("applied(%d) = %d, Go = %d", value, got, want) }
    if got, want := samePoint(&Point{X: value}), reference.SamePoint(&reference.Point{X: value}); got != want { t.Errorf("samePoint(%d) = %v, Go = %v", value, got, want) }
  }
  if got, want := nullableMissing(nil), reference.NullableMissing(nil); got != want || !got { t.Errorf("nullableMissing(nil) = %v, Go = %v", got, want) }
  if got := nullableMissing(&Box{}); got { t.Errorf("nullableMissing(Box) = true") }
  for _, values := range [][2]int{{0, 0}, {-1, 2}, {9, 3}} {
    if got, want := chosen(values[0], values[1]), reference.Chosen(values[0], values[1]); got != want { t.Errorf("chosen(%v) = %d, Go = %d", values, got, want) }
  }
  if got, want := panics(func() { inferredFirst(nil) }), panics(func() { reference.First(nil) }); got != want || !got { t.Errorf("empty first panic = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "nativegenerics.test", generated, referenceSource, testSource)
}

func TestLinkedNativeGenericFunctionsMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "generic_helpers.otm")
	entry := filepath.Join(temporary, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`
function identity<T>(value: T): T { return value; }
function second<T, U>(left: T, right: U): U { return right; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import { identity, second } from "./generic_helpers";
function linkedInferred(value: string): string { return identity(value); }
function linkedExplicit(value: int): int { return identity<int>(value); }
function linkedPartial(value: string): string { return second<int>(1, value); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "nativegenericslinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference

func identity[T any](value T) T { return value }
func second[T, U any](left T, right U) U { return right }
func LinkedInferred(value string) string { return identity(value) }
func LinkedExplicit(value int) int { return identity[int](value) }
func LinkedPartial(value string) string { return second[int](1, value) }
`
	testSource := `package nativegenericslinked

import (
  "testing"
  reference "nativegenerics-linked.test/reference"
)

func TestLinkedNativeGenerics(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := linkedInferred(value), reference.LinkedInferred(value); got != want { t.Errorf("linkedInferred(%q) = %q, Go = %q", value, got, want) }
    if got, want := linkedPartial(value), reference.LinkedPartial(value); got != want { t.Errorf("linkedPartial(%q) = %q, Go = %q", value, got, want) }
  }
  for _, value := range []int{-1, 0, 42} {
    if got, want := linkedExplicit(value), reference.LinkedExplicit(value); got != want { t.Errorf("linkedExplicit(%d) = %d, Go = %d", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "nativegenerics-linked.test", generated, referenceSource, testSource)
}

func TestPublicNativeGenericFunctionsMatchFromExternalGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "public_generics.otm")
	if err := os.WriteFile(source, []byte(`
function Identity<T>(value: T): T { return value; }
function Present<T>(value: T): Result<T> { return ok(value); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "nativegenericspublic")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference

func Identity[T any](value T) T { return value }
func Present[T any](value T) (T, error) { return value, nil }
`
	testSource := `package nativegenericspublic_test

import (
  "testing"
  generated "nativegenerics-public.test"
  reference "nativegenerics-public.test/reference"
)

func TestExternalGenericAPI(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := generated.Identity(value), reference.Identity(value); got != want { t.Errorf("Identity(%q) = %q, Go = %q", value, got, want) }
    got, gotErr := generated.Present(value)
    want, wantErr := reference.Present(value)
    if got != want || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present(%q) = (%q, %v), Go = (%q, %v)", value, got, gotErr, want, wantErr) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "nativegenerics-public.test", generated, referenceSource, testSource)
}
