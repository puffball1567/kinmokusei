package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericClassStaticMethodsMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_class_static.km")
	input := `
class Box<T> {
  constructor(public value: T) {}

  private static function normalize(value: T): T { return value; }

  public static function make(value: T): Box<T> {
    return new Box<T>(Box.normalize(value));
  }

  public static function choose(primary: T, fallback: T, first: boolean): Box<T> {
    if (first) { return Box.make(primary); }
    return Box.make(fallback);
  }

  public static function present(value: T): Result<Box<T>> {
    return ok(Box.make(value));
  }
}

class Entry<K extends comparable, V> {
  constructor(public key: K, public value: V) {}
  public static function make(key: K, value: V): Entry<K, V> {
    return new Entry<K, V>(key, value);
  }
}

function InferInteger(value: int): int {
  return Box.make(value).value;
}

function ExplicitString(value: string): string {
  const angle = Box.make<string>(value);
  const bracket = Box.make[string](angle.value + "!");
  return bracket.value;
}

function Choose(value: int, fallback: int, first: boolean): int {
  return Box.choose(value, fallback, first).value;
}

function PartialEntry(key: string, value: int): int {
  const entry = Entry.make<string>(key, value);
  return len(entry.key) * 100 + entry.value;
}

function Present(value: string): Result<string> {
  const box = Box.present(value)?;
  return ok(box.value);
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericclassstatic")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"func __kinmokuseiStaticBoxnormalize[T any](value T) T",
		"func BoxMake[T any](value T) *Box[T]",
		"func BoxChoose[T any](primary T, fallback T, first bool) *Box[T]",
		"func BoxPresent[T any](value T) (*Box[T], error)",
		"func EntryMake[K comparable, V any](key K, value V) *Entry[K, V]",
		"return BoxMake(value)",
		"var angle = BoxMake[string](value)",
		"var entry = EntryMake[string](key, value)",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Box[T any] struct { Value T }
func NewBox[T any](value T) *Box[T] { return &Box[T]{Value: value} }
func normalize[T any](value T) T { return value }
func BoxMake[T any](value T) *Box[T] { return NewBox(normalize(value)) }
func BoxChoose[T any](primary, fallback T, first bool) *Box[T] { if first { return BoxMake(primary) }; return BoxMake(fallback) }
func BoxPresent[T any](value T) (*Box[T], error) { return BoxMake(value), nil }

type Entry[K comparable, V any] struct { Key K; Value V }
func EntryMake[K comparable, V any](key K, value V) *Entry[K, V] { return &Entry[K, V]{Key: key, Value: value} }

func InferInteger(value int) int { return BoxMake(value).Value }
func ExplicitString(value string) string { angle := BoxMake[string](value); bracket := BoxMake[string](angle.Value + "!"); return bracket.Value }
func Choose(value, fallback int, first bool) int { return BoxChoose(value, fallback, first).Value }
func PartialEntry(key string, value int) int { entry := EntryMake[string](key, value); return len(entry.Key)*100 + entry.Value }
func Present(value string) (string, error) { box, err := BoxPresent(value); if err != nil { return "", err }; return box.Value, nil }
`
	testSource := `package genericclassstatic_test

import (
  "testing"
  generated "genericclassstatic.test"
  reference "genericclassstatic.test/reference"
)

func TestGenericClassStaticBehavior(t *testing.T) {
  for _, value := range []int{-9, 0, 42} {
    if got, want := generated.InferInteger(value), reference.InferInteger(value); got != want { t.Errorf("InferInteger(%d) = %d, Go = %d", value, got, want) }
    for _, first := range []bool{false, true} {
      if got, want := generated.Choose(value, 7, first), reference.Choose(value, 7, first); got != want { t.Errorf("Choose(%d, %v) = %d, Go = %d", value, first, got, want) }
    }
  }
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := generated.ExplicitString(value), reference.ExplicitString(value); got != want { t.Errorf("ExplicitString(%q) = %q, Go = %q", value, got, want) }
    gotPresent, gotErr := generated.Present(value)
    wantPresent, wantErr := reference.Present(value)
    if gotPresent != wantPresent || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present(%q) = (%q, %v), Go = (%q, %v)", value, gotPresent, gotErr, wantPresent, wantErr) }
    if got, want := generated.PartialEntry(value, 23), reference.PartialEntry(value, 23); got != want { t.Errorf("PartialEntry(%q) = %d, Go = %d", value, got, want) }
  }
  gotBox, wantBox := generated.BoxMake("public"), reference.BoxMake("public")
  if gotBox.Value != wantBox.Value { t.Errorf("BoxMake = %q, Go = %q", gotBox.Value, wantBox.Value) }
  gotEntry, wantEntry := generated.EntryMake("key", 9), reference.EntryMake("key", 9)
  if gotEntry.Key != wantEntry.Key || gotEntry.Value != wantEntry.Value { t.Errorf("EntryMake = %#v, Go = %#v", gotEntry, wantEntry) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericclassstatic.test", generated, referenceSource, testSource)
}
