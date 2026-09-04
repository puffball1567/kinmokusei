package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericClassesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_class.km")
	if err := os.WriteFile(source, []byte(`
interface Reader<T> { function read(): T; }

class Box<T> {
  constructor(public value: T) {}
  public function get(): T { return this.value; }
  public function set(value: T): void { this.value = value; }
}

class Pair<K extends comparable, V> {
  constructor(public key: K, public value: V) {}
}

class Value<T> implements Reader<T> {
  constructor(private value: T) {}
  public function read(): T { return this.value; }
}

class SliceBox<T> { constructor(public values: T[]) {} }
class Nested<T> { constructor(public inner: Box<T>) {} }
class Node<T> { constructor(public value: T, public next: Node<T> | null) {} }

function BoxBehavior(value: int, replacement: int): int {
  const box = new Box<int>(value);
  const get = box.get;
  box.set(replacement);
  return get() * 100 + box.value;
}
function Identity(value: string): boolean {
  const box = new Box<string>(value);
  const alias = box;
  alias.set(value + "!");
  return box === alias && box.get() === value + "!";
}
function PairBehavior(label: string, value: int): int {
  const pair = new Pair<string, int>(label, value);
  return len(pair.key) * 100 + pair.value;
}
function InterfaceBehavior(value: string): string {
  const reader: Reader<string> = new Value<string>(value);
  return reader.read();
}
function SliceBehavior(values: int[]): int {
  const box = new SliceBox<int>(values);
  box.values[0] += 1;
  box.values = append(box.values, 9);
  return values[0] * 100 + len(values) * 10 + len(box.values);
}
function NestedBehavior(value: int): int {
  const nested = new Nested<int>(new Box<int>(value));
  nested.inner.set(value + 1);
  return nested.inner.get();
}
function DeepBehavior(value: int): int {
  const nested = new Box<Box<int>>(new Box<int>(value));
  nested.value.set(value + 2);
  return nested.get().get();
}
function ChainBehavior(value: int): int {
  const tail = new Node<int>(value + 1, null);
  const head = new Node<int>(value, tail);
  const next = head.next;
  if (next === null) { return -1; }
  return head.value * 10 + next.value;
}
function Present<T>(value: T): Result<Box<T>> { return ok(new Box<T>(value)); }
function Lookup(values: Map<Box<int>, string>, key: Box<int>): string { return values[key]; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericclass")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"type Box[T any] struct",
		"func NewBox[T any](value T) *Box[T]",
		"func (this *Box[T]) Get() T",
		"type Pair[K comparable, V any] struct",
		"func NewPair[K comparable, V any](key K, value V) *Pair[K, V]",
		"func NewValue[T any](value T) *Value[T]",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Reader[T any] interface { Read() T }
type Box[T any] struct { Value T }
func NewBox[T any](value T) *Box[T] { return &Box[T]{Value: value} }
func (box *Box[T]) Get() T { return box.Value }
func (box *Box[T]) Set(value T) { box.Value = value }
type Pair[K comparable, V any] struct { Key K; Value V }
func NewPair[K comparable, V any](key K, value V) *Pair[K, V] { return &Pair[K, V]{Key: key, Value: value} }
type Value[T any] struct { value T }
func NewValue[T any](value T) *Value[T] { return &Value[T]{value: value} }
func (value *Value[T]) Read() T { return value.value }
type SliceBox[T any] struct { Values []T }
func NewSliceBox[T any](values []T) *SliceBox[T] { return &SliceBox[T]{Values: values} }
type Nested[T any] struct { Inner *Box[T] }
func NewNested[T any](inner *Box[T]) *Nested[T] { return &Nested[T]{Inner: inner} }
type Node[T any] struct { Value T; Next *Node[T] }
func NewNode[T any](value T, next *Node[T]) *Node[T] { return &Node[T]{Value: value, Next: next} }

func BoxBehavior(value, replacement int) int { box := NewBox(value); get := box.Get; box.Set(replacement); return get()*100 + box.Value }
func Identity(value string) bool { box := NewBox(value); alias := box; alias.Set(value+"!"); return box == alias && box.Get() == value+"!" }
func PairBehavior(label string, value int) int { pair := NewPair(label, value); return len(pair.Key)*100 + pair.Value }
func InterfaceBehavior(value string) string { var reader Reader[string] = NewValue(value); return reader.Read() }
func SliceBehavior(values []int) int { box := NewSliceBox(values); box.Values[0]++; box.Values = append(box.Values, 9); return values[0]*100 + len(values)*10 + len(box.Values) }
func NestedBehavior(value int) int { nested := NewNested(NewBox(value)); nested.Inner.Set(value+1); return nested.Inner.Get() }
func DeepBehavior(value int) int { nested := NewBox(NewBox(value)); nested.Value.Set(value+2); return nested.Get().Get() }
func ChainBehavior(value int) int { tail := NewNode(value+1, (*Node[int])(nil)); head := NewNode(value, tail); next := head.Next; if next == nil { return -1 }; return head.Value*10 + next.Value }
func Present[T any](value T) (*Box[T], error) { return NewBox(value), nil }
func Lookup(values map[*Box[int]]string, key *Box[int]) string { return values[key] }
`
	testSource := `package genericclass_test

import (
  "testing"
  generated "genericclass.test"
  reference "genericclass.test/reference"
)

func TestGenericClassBehavior(t *testing.T) {
  for _, values := range [][2]int{{0, 7}, {-2, 4}, {9, 3}} {
    if got, want := generated.BoxBehavior(values[0], values[1]), reference.BoxBehavior(values[0], values[1]); got != want { t.Errorf("BoxBehavior(%v) = %d, Go = %d", values, got, want) }
    if got, want := generated.NestedBehavior(values[0]), reference.NestedBehavior(values[0]); got != want { t.Errorf("NestedBehavior(%d) = %d, Go = %d", values[0], got, want) }
    if got, want := generated.DeepBehavior(values[0]), reference.DeepBehavior(values[0]); got != want { t.Errorf("DeepBehavior(%d) = %d, Go = %d", values[0], got, want) }
    if got, want := generated.ChainBehavior(values[0]), reference.ChainBehavior(values[0]); got != want { t.Errorf("ChainBehavior(%d) = %d, Go = %d", values[0], got, want) }
  }
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := generated.Identity(value), reference.Identity(value); got != want { t.Errorf("Identity(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.InterfaceBehavior(value), reference.InterfaceBehavior(value); got != want { t.Errorf("InterfaceBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.PairBehavior(value, 65), reference.PairBehavior(value, 65); got != want { t.Errorf("PairBehavior(%q) = %d, Go = %d", value, got, want) }
    gotPresent, gotErr := generated.Present(value)
    wantPresent, wantErr := reference.Present(value)
    if gotPresent.Value != wantPresent.Value || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present(%q) = (%q, %v), Go = (%q, %v)", value, gotPresent.Value, gotErr, wantPresent.Value, wantErr) }
  }
  gotValues, wantValues := []int{4}, []int{4}
  if got, want := generated.SliceBehavior(gotValues), reference.SliceBehavior(wantValues); got != want || gotValues[0] != wantValues[0] { t.Errorf("SliceBehavior = (%d, %v), Go = (%d, %v)", got, gotValues, want, wantValues) }
  gotBox, wantBox := generated.NewBox("public"), reference.NewBox("public")
  gotBox.Set("api"); wantBox.Set("api")
  if got, want := gotBox.Get(), wantBox.Get(); got != want { t.Errorf("public generic Go API = %q, Go = %q", got, want) }
  var gotReader generated.Reader[string] = generated.NewValue("reader")
  var wantReader reference.Reader[string] = reference.NewValue("reader")
  if got, want := gotReader.Read(), wantReader.Read(); got != want { t.Errorf("public generic interface = %q, Go = %q", got, want) }
  gotKey, wantKey := generated.NewBox(7), reference.NewBox(7)
  if got, want := generated.Lookup(map[*generated.Box[int]]string{gotKey: "value"}, gotKey), reference.Lookup(map[*reference.Box[int]]string{wantKey: "value"}, wantKey); got != want { t.Errorf("Lookup = %q, Go = %q", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericclass.test", generated, referenceSource, testSource)
}

func TestLinkedGenericClassMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "box.km")
	entry := filepath.Join(temporary, "entry.km")
	if err := os.WriteFile(dependency, []byte(`
interface Reader<T> { function read(): T; }
class Box<T> implements Reader<T> {
  constructor(public value: T) {}
  public function read(): T { return this.value; }
  public static function make(value: T): Box<T> { return new Box<T>(value); }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import { Box, Reader } from "./box";
function linked(value: string): string {
  const box: Box<string> = Box.make(value);
  const reader: Reader<string> = box;
  return reader.read();
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "genericclasslinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
type Reader[T any] interface { Read() T }
type Box[T any] struct { Value T }
func NewBox[T any](value T) *Box[T] { return &Box[T]{Value: value} }
func (box *Box[T]) Read() T { return box.Value }
func BoxMake[T any](value T) *Box[T] { return NewBox(value) }
func Linked(value string) string { var reader Reader[string] = BoxMake(value); return reader.Read() }
`
	testSource := `package genericclasslinked
import (
  "testing"
  reference "genericclass-linked.test/reference"
)
func TestLinked(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := linked(value), reference.Linked(value); got != want { t.Errorf("linked(%q) = %q, Go = %q", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericclass-linked.test", generated, referenceSource, testSource)
}
