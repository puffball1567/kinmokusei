package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeCompositeGenericInferenceMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "inference.km")
	input := `
class Box<T> { constructor(public value: T) {} }
class Child<A, B> extends Box<B> { constructor(value: B) { super(value); } }
struct Cell<T> { public value: T; }
interface Reader<T> { function read(): T; }
class TextReader implements Reader<string> {
  constructor(private value: string) {}
  public function read(): string { return this.value; }
}
class Tools {
  public function cell<T>(cell: Cell<T>): T { return cell.value; }
  public function base<T>(box: Box<T>): Box<T> { return box; }
}
function unwrap<T>(box: Box<T>): T { return box.value; }
function isNull<T>(box: Box<T> | null): boolean { return box === null; }
function read<T>(reader: Reader<T>): T { return reader.read(); }
function fromCells<T>(cells: Map<string, Cell<T>[]>): T { return cells["key"][0].value; }
let creations = 0;
function makeChild(value: string): Child<int, string> {
  creations++;
  return new Child<int, string>(value);
}
function Text(value: string): string {
  const child = new Child<int, string>(value);
  const reader: Reader<string> = new TextReader(value);
  const cell = Cell<string>{value: value};
  return unwrap(child) + read(reader) + new Tools().cell(cell);
}
function Nested(values: Map<string, Cell<int>[]>): int { return fromCells(values); }
function EvaluatedOnce(value: string): boolean {
  creations = 0;
  const box = new Tools().base(makeChild(value));
  return creations === 1 && box.value === value;
}
function Identity(child: Child<int, string> | null): boolean {
  if (child === null) { return true; }
  const base = new Tools().base(child);
  const expected: Box<string> = child;
  return base === expected;
}
function MakeChild(value: string): Child<int, string> { return new Child<int, string>(value); }
function Nullable(child: Child<int, string> | null): boolean { return isNull(child); }
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "nativeinference")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type Box[T any] struct { value T }
type Child[A, B any] struct { Box[B] }
type Cell[T any] struct { Value T }
type Reader[T any] interface { Read() T }
type textReader struct { value string }
func (r *textReader) Read() string { return r.value }
func unwrap[T any](b *Box[T]) T { return b.value }
func read[T any](r Reader[T]) T { return r.Read() }
func cell[T any](c Cell[T]) T { return c.Value }
func base[T any](b *Box[T]) *Box[T] { return b }
func Text(value string) string {
  child := &Child[int, string]{Box[string]{value}}
  var reader Reader[string] = &textReader{value}
  return unwrap(&child.Box) + read(reader) + cell(Cell[string]{value})
}
func Nested(values map[string][]Cell[int]) int { return values["key"][0].Value }
func EvaluatedOnce(value string) bool {
  creations := 0
  makeChild := func() *Child[int, string] { creations++; return &Child[int, string]{Box[string]{value}} }
  b := base(&makeChild().Box)
  return creations == 1 && b.value == value
}
func Identity(child *Child[int, string]) bool {
  if child == nil { return true }
  b := base(&child.Box)
  return b == &child.Box
}
func MakeChild(value string) *Child[int, string] { return &Child[int, string]{Box[string]{value}} }
func Nullable(child *Child[int, string]) bool {
  var b *Box[string]
  if child != nil { b = &child.Box }
  return b == nil
}
`
	comparison := `package nativeinference_test
import (
  "testing"
  generated "native-generic-inference.test"
  reference "native-generic-inference.test/reference"
)
func TestInference(t *testing.T) {
  for _, value := range []string{"", "abc", "金木犀"} {
    if got, want := generated.Text(value), reference.Text(value); got != want {
      t.Errorf("Text(%q) = %q, Go = %q", value, got, want)
    }
    if got, want := generated.EvaluatedOnce(value), reference.EvaluatedOnce(value); got != want {
      t.Errorf("EvaluatedOnce(%q) = %v, Go = %v", value, got, want)
    }
    if got, want := generated.Identity(generated.MakeChild(value)), reference.Identity(reference.MakeChild(value)); got != want {
      t.Errorf("Identity(%q) = %v, Go = %v", value, got, want)
    }
    if got, want := generated.Nullable(generated.MakeChild(value)), reference.Nullable(reference.MakeChild(value)); got != want {
      t.Errorf("Nullable(%q) = %v, Go = %v", value, got, want)
    }
  }
  if got, want := generated.Identity(nil), reference.Identity(nil); got != want {
    t.Errorf("Identity(nil) = %v, Go = %v", got, want)
  }
  if got, want := generated.Nullable(nil), reference.Nullable(nil); got != want {
    t.Errorf("Nullable(nil) = %v, Go = %v", got, want)
  }
  for _, value := range []int{-10, 0, 42} {
    got := generated.Nested(map[string][]generated.Cell[int]{"key": {{Value: value}}})
    want := reference.Nested(map[string][]reference.Cell[int]{"key": {{Value: value}}})
    if got != want { t.Errorf("Nested(%d) = %d, Go = %d", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "native-generic-inference.test", generated, reference, comparison)
}
