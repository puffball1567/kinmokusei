package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericStructsMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_struct.otm")
	if err := os.WriteFile(source, []byte(`
type UserID = distinct string;

class User {
  public name: string;
  constructor(name: string) { this.name = name; }
}

struct Box<T> {
  public value: T;
  public function get(): T { return this.value; }
  public pointer function set(value: T): void { this.value = value; }
}

struct Pair<T, U> {
  public first: T;
  public second: U;
}

struct SliceBox<T> { public values: T[]; }
struct Nested<T> { public inner: Box<T>; }
struct Node<T> { public value: T; public next: *Node<T>; }
struct ExternalBox<T> { public value: T; }
struct ExternalPair<T, U> { public first: T; public second: U; }

public function externalGet<U>(this: ExternalBox<U>): U { return this.value; }
public function externalSet<U>(this: *ExternalBox<U>, value: U): void { this.value = value; }
public function externalSecond<A, B>(this: ExternalPair<A, B>): B { return this.second; }

function NewBox<T>(value: T): Box<T> { return Box<T> { value: value }; }
function ReadBox<T>(box: Box<T>): T { return box.get(); }
function Replace<T>(box: *Box<T>, value: T): T {
  const previous = box.get();
  box.set(value);
  return previous;
}
function CopyBehavior(box: Box<int>, value: int): int {
  let copied = box;
  copied.set(value);
  return box.value * 100 + copied.value;
}
function MethodValueBehavior(box: Box<int>, value: int): int {
  const get = box.get;
  const set = box.set;
  set(value);
  return get() * 100 + box.get();
}
function SliceBehavior(box: SliceBox<int>): int {
  let copied = box;
  copied.values[0] += 1;
  copied.values = append(copied.values, 9);
  return box.values[0] * 100 + len(box.values) * 10 + len(copied.values);
}
function PairBehavior(pair: Pair<string, int>): int { return len(pair.first) * 100 + pair.second; }
function NestedBehavior(value: int): int {
  const nested: Nested<int> = Nested<int> { inner: NewBox(value) };
  return nested.inner.get();
}
function NodeBehavior(value: int): int {
  let tail: Node<int> = Node<int> { value: value + 1, next: nil };
  const head: Node<int> = Node<int> { value: value, next: &tail };
  return head.value * 10 + head.next.value;
}
function UserIDBehavior(value: string): string {
  const box: Box<UserID> = NewBox(UserID(value));
  return string(box.value);
}
function ClassBehavior(value: string): string {
  const box: Box<User> = NewBox(new User(value));
  return box.value.name;
}
function Lookup(values: Map<Box<int>, string>, key: Box<int>): string { return values[key]; }
function Present<T>(value: T): Result<Box<T>> { return ok(NewBox(value)); }
function ExternalReceiverBehavior(value: int, replacement: int): int {
  let box = ExternalBox<int> { value: value };
  const get = box.externalGet;
  box.externalSet(replacement);
  return get() * 100 + box.externalGet();
}
function ExternalPairBehavior(label: string, value: int): int {
  const pair = ExternalPair<string, int> { first: label, second: value };
  return len(pair.first) * 100 + pair.externalSecond();
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericstruct")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"type Box[T any] struct",
		"func (this Box[T]) Get() T",
		"func (this *Box[T]) Set(value T)",
		"type Pair[T any, U any] struct",
		"type Node[T any] struct",
		"Next  *Node[T]",
		"func (this ExternalBox[U]) ExternalGet() U",
		"func (this *ExternalBox[U]) ExternalSet(value U)",
		"func (this ExternalPair[A, B]) ExternalSecond() B",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type UserID string
type User struct { Name string }
func NewUser(name string) *User { return &User{Name: name} }
type Box[T any] struct { Value T }
func (box Box[T]) Get() T { return box.Value }
func (box *Box[T]) Set(value T) { box.Value = value }
type Pair[T, U any] struct { First T; Second U }
type SliceBox[T any] struct { Values []T }
type Nested[T any] struct { Inner Box[T] }
type Node[T any] struct { Value T; Next *Node[T] }
type ExternalBox[T any] struct { Value T }
type ExternalPair[T, U any] struct { First T; Second U }
func (box ExternalBox[U]) ExternalGet() U { return box.Value }
func (box *ExternalBox[U]) ExternalSet(value U) { box.Value = value }
func (pair ExternalPair[A, B]) ExternalSecond() B { return pair.Second }

func NewBox[T any](value T) Box[T] { return Box[T]{Value: value} }
func ReadBox[T any](box Box[T]) T { return box.Get() }
func Replace[T any](box *Box[T], value T) T { previous := box.Get(); box.Set(value); return previous }
func CopyBehavior(box Box[int], value int) int { copied := box; copied.Set(value); return box.Value*100 + copied.Value }
func MethodValueBehavior(box Box[int], value int) int { get := box.Get; set := box.Set; set(value); return get()*100 + box.Get() }
func SliceBehavior(box SliceBox[int]) int { copied := box; copied.Values[0]++; copied.Values = append(copied.Values, 9); return box.Values[0]*100 + len(box.Values)*10 + len(copied.Values) }
func PairBehavior(pair Pair[string, int]) int { return len(pair.First)*100 + pair.Second }
func NestedBehavior(value int) int { nested := Nested[int]{Inner: NewBox(value)}; return nested.Inner.Get() }
func NodeBehavior(value int) int { tail := Node[int]{Value: value+1}; head := Node[int]{Value: value, Next: &tail}; return head.Value*10 + head.Next.Value }
func UserIDBehavior(value string) string { box := NewBox(UserID(value)); return string(box.Value) }
func ClassBehavior(value string) string { box := NewBox(NewUser(value)); return box.Value.Name }
func Lookup(values map[Box[int]]string, key Box[int]) string { return values[key] }
func Present[T any](value T) (Box[T], error) { return NewBox(value), nil }
func ExternalReceiverBehavior(value, replacement int) int { box := ExternalBox[int]{Value: value}; get := box.ExternalGet; box.ExternalSet(replacement); return get()*100 + box.ExternalGet() }
func ExternalPairBehavior(label string, value int) int { pair := ExternalPair[string, int]{First: label, Second: value}; return len(pair.First)*100 + pair.ExternalSecond() }
`
	testSource := `package genericstruct_test

import (
  "testing"
  generated "genericstruct.test"
  reference "genericstruct.test/reference"
)

func TestGenericStructBehavior(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    gotBox, wantBox := generated.NewBox(value), reference.NewBox(value)
    if got, want := generated.ReadBox(gotBox), reference.ReadBox(wantBox); got != want { t.Errorf("ReadBox(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.Replace(&gotBox, value+"!"), reference.Replace(&wantBox, value+"!"); got != want || gotBox.Value != wantBox.Value { t.Errorf("Replace(%q) = (%q, %q), Go = (%q, %q)", value, got, gotBox.Value, want, wantBox.Value) }
    if got, want := generated.UserIDBehavior(value), reference.UserIDBehavior(value); got != want { t.Errorf("UserIDBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.ClassBehavior(value), reference.ClassBehavior(value); got != want { t.Errorf("ClassBehavior(%q) = %q, Go = %q", value, got, want) }
    gotPresent, gotErr := generated.Present(value)
    wantPresent, wantErr := reference.Present(value)
    if gotPresent.Value != wantPresent.Value || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present(%q) = (%q, %v), Go = (%q, %v)", value, gotPresent.Value, gotErr, wantPresent.Value, wantErr) }
  }
  for _, values := range [][2]int{{0, 7}, {-2, 4}, {9, 3}} {
    if got, want := generated.CopyBehavior(generated.NewBox(values[0]), values[1]), reference.CopyBehavior(reference.NewBox(values[0]), values[1]); got != want { t.Errorf("CopyBehavior(%v) = %d, Go = %d", values, got, want) }
    if got, want := generated.MethodValueBehavior(generated.NewBox(values[0]), values[1]), reference.MethodValueBehavior(reference.NewBox(values[0]), values[1]); got != want { t.Errorf("MethodValueBehavior(%v) = %d, Go = %d", values, got, want) }
    if got, want := generated.NestedBehavior(values[0]), reference.NestedBehavior(values[0]); got != want { t.Errorf("NestedBehavior(%d) = %d, Go = %d", values[0], got, want) }
    if got, want := generated.NodeBehavior(values[0]), reference.NodeBehavior(values[0]); got != want { t.Errorf("NodeBehavior(%d) = %d, Go = %d", values[0], got, want) }
    if got, want := generated.ExternalReceiverBehavior(values[0], values[1]), reference.ExternalReceiverBehavior(values[0], values[1]); got != want { t.Errorf("ExternalReceiverBehavior(%v) = %d, Go = %d", values, got, want) }
  }
  gotValues, wantValues := []int{4}, []int{4}
  if got, want := generated.SliceBehavior(generated.SliceBox[int]{Values: gotValues}), reference.SliceBehavior(reference.SliceBox[int]{Values: wantValues}); got != want || gotValues[0] != wantValues[0] { t.Errorf("SliceBehavior = (%d, %v), Go = (%d, %v)", got, gotValues, want, wantValues) }
  if got, want := generated.PairBehavior(generated.Pair[string, int]{First: "x", Second: 65}), reference.PairBehavior(reference.Pair[string, int]{First: "x", Second: 65}); got != want { t.Errorf("PairBehavior = %d, Go = %d", got, want) }
  if got, want := generated.ExternalPairBehavior("温泉", 65), reference.ExternalPairBehavior("温泉", 65); got != want { t.Errorf("ExternalPairBehavior = %d, Go = %d", got, want) }
  gotKey, wantKey := generated.NewBox(7), reference.NewBox(7)
  if got, want := generated.Lookup(map[generated.Box[int]]string{gotKey: "value"}, gotKey), reference.Lookup(map[reference.Box[int]]string{wantKey: "value"}, wantKey); got != want { t.Errorf("Lookup = %q, Go = %q", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericstruct.test", generated, referenceSource, testSource)
}

func TestLinkedGenericStructMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "box.otm")
	entry := filepath.Join(temporary, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`struct Box<T> { public value: T; public function get(): T { return this.value; } } function makeBox<T>(value: T): Box<T> { return Box<T> { value: value }; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Box, makeBox } from "./box"; function linked(value: string): string { const box: Box<string> = makeBox(value); return box.get(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "genericstructlinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
type Box[T any] struct { Value T }
func (box Box[T]) Get() T { return box.Value }
func makeBox[T any](value T) Box[T] { return Box[T]{Value: value} }
func Linked(value string) string { return makeBox(value).Get() }
`
	testSource := `package genericstructlinked
import (
  "testing"
  reference "genericstruct-linked.test/reference"
)
func TestLinked(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := linked(value), reference.Linked(value); got != want { t.Errorf("linked(%q) = %q, Go = %q", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericstruct-linked.test", generated, referenceSource, testSource)
}
