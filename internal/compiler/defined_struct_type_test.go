package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinedNativeStructTypesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "defined_struct_type.km")
	if err := os.WriteFile(source, []byte(`
import go json from "encoding/json";

struct Point {
  public x: int;
  public y: int;
}

type Offset = distinct Point;
type TaggedOffset = distinct Offset;

public function total(this: Offset): int { return this.x + this.y; }
public function shift(this: *Offset, dx: int, dy: int): void {
  this.x += dx;
  this.y += dy;
}

function NewOffset(x: int, y: int): Offset {
  return Offset { x: x, y: y };
}
function OffsetBehavior(point: Point, dx: int, dy: int): int {
  let offset = Offset(point);
  const before = offset.total;
  offset.shift(dx, dy);
  const restored = Point(offset);
  return before() * 10000 + offset.total() * 100 + restored.x * 10 + restored.y;
}
function EncodeOffset(value: Offset): Result<byte[]> {
  const encoded = json.Marshal(value)?;
  return ok(encoded);
}
function LookupOffset(values: Map<Offset, string>, key: Offset): string { return values[key]; }
function TaggedBehavior(value: Offset): int {
  const tagged = TaggedOffset(value);
  return tagged.x * 10 + tagged.y;
}

struct Segment {
  public start: Point;
  public end: Point;
}
type NamedSegment = distinct Segment;
function SegmentBehavior(value: Segment): int {
  const named = NamedSegment(value);
  return named.start.x * 1000 + named.start.y * 100 + named.end.x * 10 + named.end.y;
}

struct Bucket { public values: int[]; }
type NamedBucket = distinct Bucket;
function BucketBehavior(value: Bucket): int {
  const named = NamedBucket(value);
  named.values[0] += 1;
  return value.values[0] * 100 + named.values[0];
}

struct Node {
  public value: int;
  public next: *Node;
}
type NamedNode = distinct Node;
function NodeBehavior(value: int): int {
  let tail = Node { value: value + 1, next: nil };
  const named = NamedNode { value: value, next: &tail };
  return named.value * 10 + named.next.value;
}

struct Box<T> {
  public value: T;
}

type NamedBox<T> = distinct Box<T>;

public function get<U>(this: NamedBox<U>): U { return this.value; }
public function set<U>(this: *NamedBox<U>, value: U): void { this.value = value; }

function GenericBehavior(value: string, replacement: string): string {
  let box = NamedBox<string> { value: value };
  const get = box.get;
  box.set(replacement);
  return get() + ":" + box.get();
}
function EncodeBox(value: string): Result<byte[]> {
  const encoded = json.Marshal(NamedBox<string> { value: value })?;
  return ok(encoded);
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "definedstruct")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"type Offset Point",
		"func (this Offset) Total() int",
		"func (this *Offset) Shift(dx int, dy int)",
		"type NamedBox[T any] Box[T]",
		"func (this NamedBox[U]) Get() U",
		"func (this *NamedBox[U]) Set(value U)",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

import "encoding/json"

type Point struct {
  X int ` + "`json:\"x\"`" + `
  Y int ` + "`json:\"y\"`" + `
}
type Offset Point
type TaggedOffset Offset
func (value Offset) Total() int { return value.X + value.Y }
func (value *Offset) Shift(dx, dy int) { value.X += dx; value.Y += dy }
func NewOffset(x, y int) Offset { return Offset{X: x, Y: y} }
func OffsetBehavior(point Point, dx, dy int) int {
  offset := Offset(point)
  before := offset.Total
  offset.Shift(dx, dy)
  restored := Point(offset)
  return before()*10000 + offset.Total()*100 + restored.X*10 + restored.Y
}
func EncodeOffset(value Offset) ([]byte, error) { return json.Marshal(value) }
func LookupOffset(values map[Offset]string, key Offset) string { return values[key] }
func TaggedBehavior(value Offset) int { tagged := TaggedOffset(value); return tagged.X*10 + tagged.Y }

type Segment struct { Start Point ` + "`json:\"start\"`" + `; End Point ` + "`json:\"end\"`" + ` }
type NamedSegment Segment
func SegmentBehavior(value Segment) int {
  named := NamedSegment(value)
  return named.Start.X*1000 + named.Start.Y*100 + named.End.X*10 + named.End.Y
}

type Bucket struct { Values []int ` + "`json:\"values\"`" + ` }
type NamedBucket Bucket
func BucketBehavior(value Bucket) int {
  named := NamedBucket(value)
  named.Values[0]++
  return value.Values[0]*100 + named.Values[0]
}

type Node struct { Value int ` + "`json:\"value\"`" + `; Next *Node ` + "`json:\"next\"`" + ` }
type NamedNode Node
func NodeBehavior(value int) int {
  tail := Node{Value: value + 1}
  named := NamedNode{Value: value, Next: &tail}
  return named.Value*10 + named.Next.Value
}

type Box[T any] struct { Value T ` + "`json:\"value\"`" + ` }
type NamedBox[T any] Box[T]
func (value NamedBox[U]) Get() U { return value.Value }
func (value *NamedBox[U]) Set(replacement U) { value.Value = replacement }
func GenericBehavior(value, replacement string) string {
  box := NamedBox[string]{Value: value}
  get := box.Get
  box.Set(replacement)
  return get() + ":" + box.Get()
}
func EncodeBox(value string) ([]byte, error) { return json.Marshal(NamedBox[string]{Value: value}) }
`
	testSource := `package definedstruct_test

import (
  "bytes"
  "testing"
  generated "definedstruct.test"
  reference "definedstruct.test/reference"
)

type generatedTotaler interface { Total() int }
type referenceTotaler interface { Total() int }
type generatedSetter interface { Set(string) }
type referenceSetter interface { Set(string) }

func TestDefinedStructBehavior(t *testing.T) {
  points := []struct { x, y, dx, dy int }{
    {0, 0, 0, 0}, {-3, 7, 2, -5}, {11, -4, -8, 13},
  }
  for _, item := range points {
    got := generated.OffsetBehavior(generated.Point{X: item.x, Y: item.y}, item.dx, item.dy)
    want := reference.OffsetBehavior(reference.Point{X: item.x, Y: item.y}, item.dx, item.dy)
    if got != want { t.Errorf("OffsetBehavior(%+v) = %d, Go = %d", item, got, want) }

    gotOffset, wantOffset := generated.NewOffset(item.x, item.y), reference.NewOffset(item.x, item.y)
    var gotTotaler generatedTotaler = gotOffset
    var wantTotaler referenceTotaler = wantOffset
    if gotTotaler.Total() != wantTotaler.Total() { t.Errorf("value method set = %d, Go = %d", gotTotaler.Total(), wantTotaler.Total()) }
    gotJSON, gotErr := generated.EncodeOffset(gotOffset)
    wantJSON, wantErr := reference.EncodeOffset(wantOffset)
    if !bytes.Equal(gotJSON, wantJSON) || (gotErr == nil) != (wantErr == nil) {
      t.Errorf("EncodeOffset = (%s, %v), Go = (%s, %v)", gotJSON, gotErr, wantJSON, wantErr)
    }
    if got, want := generated.LookupOffset(map[generated.Offset]string{gotOffset: "present"}, gotOffset), reference.LookupOffset(map[reference.Offset]string{wantOffset: "present"}, wantOffset); got != want {
      t.Errorf("LookupOffset = %q, Go = %q", got, want)
    }
    if got, want := generated.TaggedBehavior(gotOffset), reference.TaggedBehavior(wantOffset); got != want {
      t.Errorf("TaggedBehavior = %d, Go = %d", got, want)
    }
    gotSegment := generated.Segment{Start: generated.Point{X: item.x, Y: item.y}, End: generated.Point{X: item.dx, Y: item.dy}}
    wantSegment := reference.Segment{Start: reference.Point{X: item.x, Y: item.y}, End: reference.Point{X: item.dx, Y: item.dy}}
    if got, want := generated.SegmentBehavior(gotSegment), reference.SegmentBehavior(wantSegment); got != want {
      t.Errorf("SegmentBehavior = %d, Go = %d", got, want)
    }
    gotValues, wantValues := []int{item.x}, []int{item.x}
    if got, want := generated.BucketBehavior(generated.Bucket{Values: gotValues}), reference.BucketBehavior(reference.Bucket{Values: wantValues}); got != want || gotValues[0] != wantValues[0] {
      t.Errorf("BucketBehavior = (%d, %v), Go = (%d, %v)", got, gotValues, want, wantValues)
    }
    if got, want := generated.NodeBehavior(item.x), reference.NodeBehavior(item.x); got != want {
      t.Errorf("NodeBehavior = %d, Go = %d", got, want)
    }
  }

  strings := []struct { value, replacement string }{
    {"", ""}, {"onsen", "tamago"}, {"温泉", "卵"},
  }
  for _, item := range strings {
    if got, want := generated.GenericBehavior(item.value, item.replacement), reference.GenericBehavior(item.value, item.replacement); got != want {
      t.Errorf("GenericBehavior(%q, %q) = %q, Go = %q", item.value, item.replacement, got, want)
    }
    gotBox, wantBox := generated.NamedBox[string]{Value: item.value}, reference.NamedBox[string]{Value: item.value}
    var gotSetter generatedSetter = &gotBox
    var wantSetter referenceSetter = &wantBox
    gotSetter.Set(item.replacement)
    wantSetter.Set(item.replacement)
    if gotBox.Get() != wantBox.Get() { t.Errorf("pointer method set = %q, Go = %q", gotBox.Get(), wantBox.Get()) }
    gotJSON, gotErr := generated.EncodeBox(item.value)
    wantJSON, wantErr := reference.EncodeBox(item.value)
    if !bytes.Equal(gotJSON, wantJSON) || (gotErr == nil) != (wantErr == nil) {
      t.Errorf("EncodeBox(%q) = (%s, %v), Go = (%s, %v)", item.value, gotJSON, gotErr, wantJSON, wantErr)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "definedstruct.test", generated, referenceSource, testSource)
}
