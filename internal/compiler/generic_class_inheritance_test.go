package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericClassInheritanceMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_class_inheritance.otm")
	input := `
interface Reader<T> { function read(): T; }

class Base<T> implements Reader<T> {
  constructor(protected value: T) {}
  public function read(): T { return this.value; }
  public function get(): T { return this.value; }
}

class Child<U> extends Base<U> {
  constructor(value: U, public label: string) { super(value); }
  public function pair(): string { return this.label; }
}

class Concrete extends Base<int> {
  constructor(value: int) { super(value); }
}

class Middle<A, B> extends Base<B> {
  constructor(value: B) { super(value); }
}

class Leaf<X> extends Middle<int, X> {
  constructor(value: X) { super(value); }
}

class Fixed extends Base<string> {
  constructor(value: string) { super(value); }
}

class GenericLeaf<Marker> extends Fixed {
  constructor(value: string, public marker: Marker) { super(value); }
}

function Inherited(value: string): string {
  return new Child<string>(value, "child").get();
}

function InterfaceValue(value: string): string {
  const reader: Reader<string> = new Child<string>(value, "reader");
  return reader.read();
}

function ConcreteValue(value: int): int {
  return new Concrete(value).get();
}

function MultiLevel(value: string): string {
  return new Leaf<string>(value).get();
}

function RoundTrip(value: string): boolean {
  const child = new Child<string>(value, "round");
  const base: Base<string> = child;
  const [restored, present] = base as? Child<string>;
  return present && restored === child && restored.get() === value;
}

function ConcreteRoundTrip(value: int): boolean {
  const concrete = new Concrete(value);
  const base: Base<int> = concrete;
  const [restored, present] = base as? Concrete;
  return present && restored === concrete && restored.get() === value;
}

function LeafRoundTrip(value: string): boolean {
  const leaf = new Leaf<string>(value);
  const base: Base<string> = leaf;
  const [restored, present] = base as? Leaf<string>;
  return present && restored === leaf && restored.get() === value;
}

function IntermediateRoundTrip(value: string): boolean {
  const leaf = new Leaf<string>(value);
  const expected: Middle<int, string> = leaf;
  const base: Base<string> = leaf;
  const [restored, present] = base as? Middle<int, string>;
  return present && restored === expected && restored.get() === value;
}

function WrongIntermediateArgumentsFail(value: string): boolean {
  const base: Base<string> = new Leaf<string>(value);
  const [_, present] = base as? Middle<boolean, string>;
  return !present;
}

function GenericDescendantToFixed(value: string): boolean {
  const leaf = new GenericLeaf<int>(value, 7);
  const expected: Fixed = leaf;
  const base: Base<string> = leaf;
  const [restored, present] = base as? Fixed;
  return present && restored === expected && restored.get() === value;
}

function NullableRoundTrip(value: Child<string> | null): boolean {
  const base: Base<string> | null = value;
  if (base === null) { return true; }
  const [restored, present] = base as? Child<string>;
  return present && restored === value;
}

function CheckedFailure(value: string): boolean {
  const base = new Base<string>(value);
  const [_, present] = base as? Child<string>;
  return !present;
}

function ForcedValue(value: string): string {
  const base: Base<string> = new Child<string>(value, "forced");
  return (base as! Child<string>).get();
}

function ForcedFailure(value: string): string {
  const base = new Base<string>(value);
  return (base as! Child<string>).get();
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericinheritance")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"type Child[U any] struct",
		"Base[U]",
		"type Middle[A any, B any] struct",
		"Base[B]",
		"type Leaf[X any] struct",
		"Middle[int, X]",
		"func UpcastChildToBase[U any](value *Child[U]) *Base[U]",
		"func DowncastBaseToChild[U any](value *Base[U]) (*Child[U], bool)",
		"func UpcastLeafToBase[X any](value *Leaf[X]) *Base[X]",
		"func DowncastBaseToLeaf[X any](value *Base[X]) (*Leaf[X], bool)",
		"func DowncastBaseToConcrete(value *Base[int]) (*Concrete, bool)",
		"type __ontamaMiddleProjection[A any, B any] interface",
		"__ontamaAsMiddle() *Middle[A, B]",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Reader[T any] interface { Read() T }
type Base[T any] struct { root any; value T }
func NewBase[T any](value T) *Base[T] { result := &Base[T]{value: value}; result.root = result; return result }
func (value *Base[T]) Read() T { return value.value }
func (value *Base[T]) Get() T { return value.value }

type Child[U any] struct { Base[U]; Label string }
func NewChild[U any](value U, label string) *Child[U] { result := &Child[U]{Base: Base[U]{value: value}, Label: label}; result.root = result; return result }
func UpcastChildToBase[U any](value *Child[U]) *Base[U] { if value == nil { return nil }; return &value.Base }
func DowncastBaseToChild[U any](value *Base[U]) (*Child[U], bool) { if value == nil { return nil, false }; result, ok := value.root.(*Child[U]); return result, ok }
func MustDowncastBaseToChild[U any](value *Base[U]) *Child[U] { result, ok := DowncastBaseToChild(value); if !ok { panic("cannot downcast Base to Child") }; return result }

type Concrete struct { Base[int] }
func NewConcrete(value int) *Concrete { result := &Concrete{Base: Base[int]{value: value}}; result.root = result; return result }

type Middle[A, B any] struct { Base[B] }
func NewMiddle[A, B any](value B) *Middle[A, B] { result := &Middle[A, B]{Base: Base[B]{value: value}}; result.root = result; return result }
type Leaf[X any] struct { Middle[int, X] }
func NewLeaf[X any](value X) *Leaf[X] { result := &Leaf[X]{Middle: Middle[int, X]{Base: Base[X]{value: value}}}; result.root = result; return result }

type Fixed struct { Base[string] }
type GenericLeaf[Marker any] struct { Fixed; Marker Marker }
func NewGenericLeaf[Marker any](value string, marker Marker) *GenericLeaf[Marker] { result := &GenericLeaf[Marker]{Fixed: Fixed{Base: Base[string]{value: value}}, Marker: marker}; result.root = result; return result }

func Inherited(value string) string { return NewChild(value, "child").Get() }
func InterfaceValue(value string) string { var reader Reader[string] = NewChild(value, "reader"); return reader.Read() }
func ConcreteValue(value int) int { return NewConcrete(value).Get() }
func MultiLevel(value string) string { return NewLeaf(value).Get() }
func RoundTrip(value string) bool { child := NewChild(value, "round"); base := UpcastChildToBase(child); restored, present := DowncastBaseToChild(base); return present && restored == child && restored.Get() == value }
func ConcreteRoundTrip(value int) bool { concrete := NewConcrete(value); base := &concrete.Base; restored, present := base.root.(*Concrete); return present && restored == concrete && restored.Get() == value }
func LeafRoundTrip(value string) bool { leaf := NewLeaf(value); base := &leaf.Middle.Base; restored, present := base.root.(*Leaf[string]); return present && restored == leaf && restored.Get() == value }
func IntermediateRoundTrip(value string) bool { leaf := NewLeaf(value); expected := &leaf.Middle; base := &leaf.Middle.Base; root, present := base.root.(*Leaf[string]); if !present { return false }; restored := &root.Middle; return restored == expected && restored.Get() == value }
func WrongIntermediateArgumentsFail(value string) bool { leaf := NewLeaf(value); base := &leaf.Middle.Base; _, present := base.root.(*Middle[bool, string]); return !present }
func GenericDescendantToFixed(value string) bool { leaf := NewGenericLeaf(value, 7); expected := &leaf.Fixed; base := &leaf.Fixed.Base; root, present := base.root.(*GenericLeaf[int]); if !present { return false }; restored := &root.Fixed; return restored == expected && restored.Get() == value }
func NullableRoundTrip(value *Child[string]) bool { var base *Base[string]; if value != nil { base = &value.Base }; if base == nil { return true }; restored, present := base.root.(*Child[string]); return present && restored == value }
func CheckedFailure(value string) bool { base := NewBase(value); child, present := DowncastBaseToChild(base); return !present && child == nil }
func ForcedValue(value string) string { return MustDowncastBaseToChild(UpcastChildToBase(NewChild(value, "forced"))).Get() }
func ForcedFailure(value string) string { return MustDowncastBaseToChild(NewBase(value)).Get() }
`
	testSource := `package genericinheritance_test

import (
  "fmt"
  "testing"
  generated "genericinheritance.test"
  reference "genericinheritance.test/reference"
)

func panicText(call func()) (result string) {
  defer func() { if value := recover(); value != nil { result = fmt.Sprint(value) } }()
  call()
  return ""
}

func TestGenericInheritanceBehavior(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := generated.Inherited(value), reference.Inherited(value); got != want { t.Errorf("Inherited(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.InterfaceValue(value), reference.InterfaceValue(value); got != want { t.Errorf("InterfaceValue(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.MultiLevel(value), reference.MultiLevel(value); got != want { t.Errorf("MultiLevel(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.RoundTrip(value), reference.RoundTrip(value); got != want { t.Errorf("RoundTrip(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.LeafRoundTrip(value), reference.LeafRoundTrip(value); got != want { t.Errorf("LeafRoundTrip(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.IntermediateRoundTrip(value), reference.IntermediateRoundTrip(value); got != want { t.Errorf("IntermediateRoundTrip(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.WrongIntermediateArgumentsFail(value), reference.WrongIntermediateArgumentsFail(value); got != want { t.Errorf("WrongIntermediateArgumentsFail(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.GenericDescendantToFixed(value), reference.GenericDescendantToFixed(value); got != want { t.Errorf("GenericDescendantToFixed(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.NullableRoundTrip(generated.NewChild(value, "nullable")), reference.NullableRoundTrip(reference.NewChild(value, "nullable")); got != want { t.Errorf("NullableRoundTrip(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.CheckedFailure(value), reference.CheckedFailure(value); got != want { t.Errorf("CheckedFailure(%q) = %v, Go = %v", value, got, want) }
    if got, want := generated.ForcedValue(value), reference.ForcedValue(value); got != want { t.Errorf("ForcedValue(%q) = %q, Go = %q", value, got, want) }
    gotPanic := panicText(func() { generated.ForcedFailure(value) })
    wantPanic := panicText(func() { reference.ForcedFailure(value) })
    if gotPanic != wantPanic { t.Errorf("ForcedFailure(%q) panic = %q, Go = %q", value, gotPanic, wantPanic) }
  }
  for _, value := range []int{-9, 0, 42} {
    if got, want := generated.ConcreteValue(value), reference.ConcreteValue(value); got != want { t.Errorf("ConcreteValue(%d) = %d, Go = %d", value, got, want) }
    if got, want := generated.ConcreteRoundTrip(value), reference.ConcreteRoundTrip(value); got != want { t.Errorf("ConcreteRoundTrip(%d) = %v, Go = %v", value, got, want) }
  }
  if got, want := generated.NullableRoundTrip(nil), reference.NullableRoundTrip(nil); got != want { t.Errorf("NullableRoundTrip(nil) = %v, Go = %v", got, want) }
  gotChild, wantChild := generated.NewChild("api", "label"), reference.NewChild("api", "label")
  gotBase, wantBase := generated.UpcastChildToBase(gotChild), reference.UpcastChildToBase(wantChild)
  gotRestored, gotOK := generated.DowncastBaseToChild(gotBase)
  wantRestored, wantOK := reference.DowncastBaseToChild(wantBase)
  if !gotOK || !wantOK || gotRestored != gotChild || wantRestored != wantChild || gotRestored.Get() != wantRestored.Get() { t.Errorf("public conversion API did not preserve identity") }
  if got, ok := generated.DowncastBaseToChild[string](nil); ok || got != nil { t.Errorf("public checked nil downcast = (%v, %v)", got, ok) }
  if got := generated.UpcastChildToBase[string](nil); got != nil { t.Errorf("public nil upcast = %v", got) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericinheritance.test", generated, referenceSource, testSource)
}

func TestLinkedGenericClassInheritanceMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	baseSource := filepath.Join(temporary, "base.otm")
	entrySource := filepath.Join(temporary, "entry.otm")
	if err := os.WriteFile(baseSource, []byte(`
class Base<T> {
  constructor(protected value: T) {}
  public function get(): T { return this.value; }
}
class Child<U> extends Base<U> {
  constructor(value: U) { super(value); }
}
function makeChild(value: string): Child<string> { return new Child<string>(value); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entrySource, []byte(`
import { Base, Child, makeChild } from "./base";
function linked(value: string): string {
  const child = makeChild(value);
  const base: Base<string> = child;
  const [restored, present] = base as? Child<string>;
  if (!present) { return "missing"; }
  return restored.get();
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entrySource}, "genericinheritancelinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	referenceSource := `package reference
type Base[T any] struct { root any; value T }
func (value *Base[T]) Get() T { return value.value }
type Child[U any] struct { Base[U] }
func NewChild[U any](value U) *Child[U] { result := &Child[U]{Base: Base[U]{value: value}}; result.root = result; return result }
func makeChild(value string) *Child[string] { return NewChild(value) }
func Linked(value string) string { child := makeChild(value); base := &child.Base; restored, present := base.root.(*Child[string]); if !present { return "missing" }; return restored.Get() }
`
	testSource := `package genericinheritancelinked
import (
  "testing"
  reference "genericinheritance-linked.test/reference"
)
func TestLinkedGenericInheritance(t *testing.T) {
  for _, value := range []string{"", "linked", "温泉卵"} {
    if got, want := linked(value), reference.Linked(value); got != want { t.Errorf("linked(%q) = %q, Go = %q", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericinheritance-linked.test", generated, referenceSource, testSource)
}
