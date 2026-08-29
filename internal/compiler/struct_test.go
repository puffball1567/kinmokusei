package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeStructsCompileAndMatchIndependentGo(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "struct.otm")
	dependency := filepath.Join(temp, "dependency.otm")
	input := `
import { ImportedPair, importedPair } from "./dependency";
import go fmt from "fmt";

class Box {
  constructor(public value: int) {}
}

struct Point {
  public x: int;
  public y: int;
}
struct Holder {
  public box: Box;
  public values: int[];
}
struct Key {
  public id: int;
  public pair: [2]int;
}
struct Node {
  public value: int;
  public next: *Node;
}
struct Tree {
  public value: int;
  public children: Tree[];
}
struct Secret {
  private value: int;
}

function copyByAssignment(): [4]int {
  let original = Point { x: 1, y: 2 };
  let duplicate: Point = original;
  duplicate.x = 9;
  return [original.x, original.y, duplicate.x, duplicate.y];
}
function mutateParameter(point: Point): Point {
  point.y = 8;
  return point;
}
function copyThroughParameter(): [4]int {
  const original = Point { x: 3, y: 4 };
  const changed = mutateParameter(original);
  return [original.x, original.y, changed.x, changed.y];
}
function mutatePointer(point: *Point): void {
  (*point).x = (*point).x + 5;
}
function explicitPointerAlias(): int {
  let point = Point { x: 7, y: 0 };
  mutatePointer(&point);
  return point.x;
}
function shallowCopy(): [4]int {
  const original = Holder { box: new Box(1), values: [2, 3] };
  let duplicate = original;
  duplicate.box.value = 7;
  duplicate.values[0] = 8;
  duplicate.values = [9];
  return [original.box.value, original.values[0], duplicate.box.value, duplicate.values[0]];
}
function comparableAndMap(): [3]boolean {
  const first = Key { id: 1, pair: [2, 3] };
  const same = Key { id: 1, pair: [2, 3] };
  const other = Key { id: 2, pair: [2, 3] };
  const values = makeMap[Key, string]();
  values[first] = "hot";
  return [first === same, first !== other, values[same] === "hot"];
}
function observe(counter: *int, value: int): int {
  (*counter) = (*counter) * 10 + value;
  return value;
}
function literalEvaluationOrder(): int {
  let counter = 0;
  const point = Point { y: observe(&counter, 2), x: observe(&counter, 1) };
  return counter * 100 + point.x * 10 + point.y;
}
function recursiveShapes(): int {
  const node = Node { value: 4, next: nil };
  const tree = Tree { value: 5, children: [Tree { value: 6, children: [] }] };
  return node.value * 100 + tree.value * 10 + tree.children[0].value;
}
function privateField(): int {
  const value = Secret { value: 42 };
  return value.value;
}
function importedValueCopy(): [2]int {
  let original: ImportedPair = importedPair(4, 5);
  let duplicate = original;
  duplicate.left = 9;
  return [original.left, duplicate.left];
}
function formatValue(): string {
  return fmt.Sprintf("%v", Point { x: 7, y: 8 });
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependency, []byte(`
struct ImportedPair { public left: int; public right: int; }
function importedPair(left: int, right: int): ImportedPair {
  return ImportedPair { left: left, right: right };
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "nativestruct")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, want := range []string{
		"type Point struct",
		"X int",
		"type Node struct",
		"Next  *Node",
		"type Secret struct",
		"value int",
		"Point{Y: observe(&counter, 2), X: observe(&counter, 1)}",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference
import "fmt"

type box struct { value int }
type point struct { x, y int }
type holder struct { box *box; values []int }
type key struct { id int; pair [2]int }
type node struct { value int; next *node }
type tree struct { value int; children []tree }
type secret struct { value int }

func CopyByAssignment() [4]int {
  original := point{x: 1, y: 2}
  duplicate := original
  duplicate.x = 9
  return [4]int{original.x, original.y, duplicate.x, duplicate.y}
}
func mutateParameter(value point) point {
  value.y = 8
  return value
}
func CopyThroughParameter() [4]int {
  original := point{x: 3, y: 4}
  changed := mutateParameter(original)
  return [4]int{original.x, original.y, changed.x, changed.y}
}
func mutatePointer(value *point) { value.x += 5 }
func ExplicitPointerAlias() int {
  value := point{x: 7}
  mutatePointer(&value)
  return value.x
}
func ShallowCopy() [4]int {
  original := holder{box: &box{value: 1}, values: []int{2, 3}}
  duplicate := original
  duplicate.box.value = 7
  duplicate.values[0] = 8
  duplicate.values = []int{9}
  return [4]int{original.box.value, original.values[0], duplicate.box.value, duplicate.values[0]}
}
func ComparableAndMap() [3]bool {
  first := key{id: 1, pair: [2]int{2, 3}}
  same := key{id: 1, pair: [2]int{2, 3}}
  other := key{id: 2, pair: [2]int{2, 3}}
  values := make(map[key]string)
  values[first] = "hot"
  return [3]bool{first == same, first != other, values[same] == "hot"}
}
func observe(counter *int, value int) int {
  *counter = *counter * 10 + value
  return value
}
func LiteralEvaluationOrder() int {
  counter := 0
  value := point{y: observe(&counter, 2), x: observe(&counter, 1)}
  return counter * 100 + value.x * 10 + value.y
}
func RecursiveShapes() int {
  n := node{value: 4, next: nil}
  tr := tree{value: 5, children: []tree{{value: 6, children: []tree{}}}}
  return n.value * 100 + tr.value * 10 + tr.children[0].value
}
func PrivateField() int { return secret{value: 42}.value }
func ImportedValueCopy() [2]int {
  original := struct{ left, right int }{left: 4, right: 5}
  duplicate := original
  duplicate.left = 9
  return [2]int{original.left, duplicate.left}
}
func FormatValue() string { return fmt.Sprintf("%v", point{x: 7, y: 8}) }
`
	testSource := `package nativestruct
import (
  "testing"
  reference "native-struct.test/reference"
)
func TestNativeStructRuntimeContract(t *testing.T) {
  if got, want := copyByAssignment(), reference.CopyByAssignment(); got != want { t.Errorf("assignment copy = %v, Go = %v", got, want) }
  if got, want := copyThroughParameter(), reference.CopyThroughParameter(); got != want { t.Errorf("parameter copy = %v, Go = %v", got, want) }
  if got, want := explicitPointerAlias(), reference.ExplicitPointerAlias(); got != want { t.Errorf("pointer alias = %d, Go = %d", got, want) }
  if got, want := shallowCopy(), reference.ShallowCopy(); got != want { t.Errorf("shallow copy = %v, Go = %v", got, want) }
  if got, want := comparableAndMap(), reference.ComparableAndMap(); got != want { t.Errorf("comparison/map = %v, Go = %v", got, want) }
  if got, want := literalEvaluationOrder(), reference.LiteralEvaluationOrder(); got != want { t.Errorf("evaluation order = %d, Go = %d", got, want) }
  if got, want := recursiveShapes(), reference.RecursiveShapes(); got != want { t.Errorf("recursive shapes = %d, Go = %d", got, want) }
  if got, want := privateField(), reference.PrivateField(); got != want { t.Errorf("private field = %d, Go = %d", got, want) }
  if got, want := importedValueCopy(), reference.ImportedValueCopy(); got != want { t.Errorf("imported copy = %v, Go = %v", got, want) }
  if got, want := formatValue(), reference.FormatValue(); got != want { t.Errorf("format = %q, Go = %q", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "native-struct.test", generated, referenceSource, testSource)
}
