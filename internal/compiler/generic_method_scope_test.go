package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericMethodScopesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, "base.km")
	entry := filepath.Join(root, "entry.km")
	files := map[string]string{
		base: `
constraint Integer = ~int | ~int8;
class Base<T> {
  constructor(public value: T) {}
  public function echo<U extends Integer>(value: U): U { return value + value; }
  public function pair<U>(value: U): {owner: T, method: U} { return {owner: this.value, method: value}; }
  public function keep<U>(marker: U): T { return this.value; }
}
struct Cell<T> {
  public value: T;
  public function keep<U>(marker: U): T { return this.value; }
  public pointer function replace<V>(value: T, marker: V): T { this.value = value; return this.value; }
}
`,
		entry: `
import { Base, Cell, Integer } from "./base";
class Middle<U> extends Base<U> { constructor(value: U) { super(value); } }
class Child<V> extends Middle<V> {
  constructor(value: V) { super(value); }
  public function relay<W extends Integer>(value: W): W { return super.echo(value); }
}
function keep<U>(box: Base<U>): U { return box.keep<int>(1); }
function cellKeep<U>(cell: Cell<U>): U { return cell.keep<int>(1); }
function cellReplace<U>(cell: *Cell<U>, value: U): U { return cell.replace(value, 1); }
function Owner(value: string): string {
  const child = new Child<string>(value);
  const pair = child.pair<int>(3);
  return pair.owner + keep<string>(child);
}
function Marker(value: int8): int8 { return new Child<string>("unused").relay(value); }
function PairMarker(value: int): int { return new Middle<string>("unused").pair(value).method; }
function CellOwner(value: string, replacement: string): string {
  let cell = Cell<string>{value: value};
  const before = cellKeep(cell);
  const after = cellReplace(&cell, replacement);
  return before + after + cell.value;
}
`,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "genericscopes")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type base[T any] struct { value T }
type middle[U any] struct { base[U] }
type child[V any] struct { middle[V] }
type cell[T any] struct { value T }
type integer interface { ~int | ~int8 }
func echo[T any, U integer](b *base[T], value U) U { return value + value }
func pair[T, U any](b *base[T], value U) struct { owner T; method U } {
  return struct { owner T; method U }{b.value, value}
}
func keep[T, U any](b *base[T], marker U) T { return b.value }
func cellKeep[T, U any](c cell[T], marker U) T { return c.value }
func cellReplace[T, V any](c *cell[T], value T, marker V) T { c.value = value; return c.value }
func Owner(value string) string {
  c := child[string]{middle[string]{base[string]{value}}}
  p := pair(&c.base, 3)
  return p.owner + keep(&c.base, 1)
}
func Marker(value int8) int8 { c := child[string]{}; return echo(&c.base, value) }
func PairMarker(value int) int { b := base[string]{}; return pair(&b, value).method }
func CellOwner(value, replacement string) string {
  c := cell[string]{value}
  before := cellKeep(c, 1)
  after := cellReplace(&c, replacement, 1)
  return before + after + c.value
}
`
	comparison := `package genericscopes_test
import (
  "testing"
  generated "genericmethod-scopes.test"
  reference "genericmethod-scopes.test/reference"
)
func TestScopes(t *testing.T) {
  for _, value := range []string{"", "value", "金木犀"} {
    if got, want := generated.Owner(value), reference.Owner(value); got != want {
      t.Errorf("Owner(%q) = %q, Go = %q", value, got, want)
    }
    for _, replacement := range []string{"", "updated", "桂花"} {
      if got, want := generated.CellOwner(value, replacement), reference.CellOwner(value, replacement); got != want {
        t.Errorf("CellOwner(%q, %q) = %q, Go = %q", value, replacement, got, want)
      }
    }
  }
  for _, value := range []int8{-128, -1, 0, 1, 63, 64, 127} {
    if got, want := generated.Marker(value), reference.Marker(value); got != want {
      t.Errorf("Marker(%d) = %d, Go = %d", value, got, want)
    }
  }
  for _, value := range []int{-100, 0, 42} {
    if got, want := generated.PairMarker(value), reference.PairMarker(value); got != want {
      t.Errorf("PairMarker(%d) = %d, Go = %d", value, got, want)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "genericmethod-scopes.test", generated, reference, comparison)
}
