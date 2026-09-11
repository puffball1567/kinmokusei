package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstructorTypeNameShadowingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "constructor_names.km")
	input := `class Box<T> {
  constructor(public Box: T, public Box_: int, ...values: int[]) {
    this.Box_ += len(values);
  }
}
class Link { constructor(public Link: Link | null) {} }
class Values {
  public total: int;
  constructor(...Values: int[]) {
    this.total = 0;
    for (const value of Values) { this.total += value; }
  }
}
function Run(seed: int): int[] {
  const box = new Box<int>(seed, 7, 10, 20);
  const empty = new Box<int>(seed, 7);
  const root = new Link(null);
  const child = new Link(root);
  let identity = 0;
  let nullable = 0;
  if (child.Link == root) { identity = 1; }
  if (root.Link == null) { nullable = 1; }
  return [box.Box, box.Box_, empty.Box_, new Values(seed, 2, 3).total,
    new Values().total, identity, nullable];
}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "constructornames")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type box[T any] struct { value T; count int }
func newBox[T any](value T, count int, values ...int) *box[T] {
  return &box[T]{value: value, count: count + len(values)}
}
type link struct { parent *link }
func sum(values ...int) int { total := 0; for _, v := range values { total += v }; return total }
func Run(seed int) []int {
  full, empty := newBox(seed, 7, 10, 20), newBox(seed, 7)
  root := &link{}
  child := &link{parent: root}
  identity, nullable := 0, 0
  if child.parent == root { identity = 1 }
  if root.parent == nil { nullable = 1 }
  return []int{full.value, full.count, empty.count, sum(seed, 2, 3), sum(), identity, nullable}
}`
	comparison := `package constructornames
import (
  "reflect"
  "testing"
  reference "constructor-names.test/reference"
)
func TestNames(t *testing.T) {
  for _, seed := range []int{-10, 0, 1, 100} {
    want := reference.Run(seed)
    if got := Run(seed); !reflect.DeepEqual(got, want) { t.Errorf("Run(%d)=%v want %v", seed, got, want) }
    box := NewBox[int](seed, 7, 10, 20)
    if box.Box != want[0] || box.Box_ != want[1] { t.Errorf("Go constructor = %+v", box) }
  }
}`
	runGeneratedGoDifferentialTest(t, root, "constructor-names.test", generated, reference, comparison)
}
