package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericClassVirtualDispatchMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_class_virtual.km")
	input := `
interface Reader<T> { function read(): T; }

class Base<T> implements Reader<T> {
  public constructorSeen: T;
  constructor(protected value: T) {
    this.constructorSeen = this.read();
  }
  public virtual function read(): T { return this.value; }
  public virtual function choose(value: T): T { return value; }
  public function indirect(): T { return this.read(); }
}

class Child<U> extends Base<U> {
  constructor(value: U, public replacement: U) { super(value); }
  public override function read(): U { return this.replacement; }
  public override function choose(value: U): U { return this.replacement; }
  public virtual function label(value: U): U { return value; }
  public function baseRead(): U { return super.read(); }
}

class GrandChild<V> extends Child<V> {
  constructor(value: V, replacement: V) { super(value, replacement); }
  public override function label(value: V): V { return this.replacement; }
}

class Concrete extends Base<string> {
  constructor(value: string) { super(value); }
  public override function read(): string { return "concrete"; }
}

class Middle<A, B> extends Base<B> {
  constructor(value: B) { super(value); }
  public override function read(): B { return super.read(); }
}

class Leaf<X> extends Middle<int, X> {
  constructor(value: X, public replacement: X) { super(value); }
  public final override function read(): X { return this.replacement; }
}

class ComparableBase<K extends comparable> {
  public virtual function equal(left: K, right: K): boolean { return left === right; }
}

class ComparableChild<K extends comparable> extends ComparableBase<K> {
  public override function equal(left: K, right: K): boolean { return super.equal(left, right); }
}

function GenericBehavior(value: string): string {
  const child = new Child<string>(value, value + "-child");
  const base: Base<string> = child;
  return base.constructorSeen + "/" + base.indirect() + "/" + base.choose(value + "-ignored") + "/" + child.baseRead();
}

function InterfaceBehavior(value: string): string {
  const reader: Reader<string> = new Child<string>(value, value + "-interface");
  return reader.read();
}

function ChildOwnedBehavior(value: string): string {
  const child: Child<string> = new GrandChild<string>(value, value + "-grand");
  return child.label(value + "-argument");
}

function MethodValueBehavior(value: string): string {
  const base: Base<string> = new Child<string>(value, value + "-method");
  const read = base.read;
  return read();
}

function ConcreteBehavior(value: string): string {
  const base: Base<string> = new Concrete(value);
  return base.constructorSeen + "/" + base.read();
}

function MultiLevelBehavior(value: string): string {
  const leaf = new Leaf<string>(value, value + "-leaf");
  const base: Base<string> = leaf;
  return base.constructorSeen + "/" + base.indirect();
}

function ComparableBehavior(left: string, right: string): boolean {
  const base: ComparableBase<string> = new ComparableChild<string>();
  return base.equal(left, right);
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericvirtual")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"type __kinmokuseiBaseVirtual[T any] interface",
		"__kinmokuseiBaseSelf __kinmokuseiBaseVirtual[T]",
		"__kinmokuseiBaseChoose(value T) T",
		"func (this *Base[T]) Read() T",
		"func (this *Child[U]) __kinmokuseiBaseRead() U",
		"type __kinmokuseiChildVirtual[U any] interface",
		"func (this *GrandChild[V]) __kinmokuseiChildLabel(value V) V",
		"func (this *Concrete) __kinmokuseiBaseRead() string",
		"func (this *Leaf[X]) __kinmokuseiBaseRead() X",
		"type __kinmokuseiComparableBaseVirtual[K comparable] interface",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Reader[T any] interface { Read() T }
type baseDispatch[T any] interface { read() T; choose(T) T }
type Base[T any] struct { self baseDispatch[T]; value T; ConstructorSeen T }
func initBase[T any](target *Base[T], value T) { target.self = target; target.value = value; target.ConstructorSeen = target.self.read() }
func NewBase[T any](value T) *Base[T] { target := &Base[T]{}; initBase(target, value); return target }
func (target *Base[T]) read() T { return target.value }
func (target *Base[T]) choose(value T) T { return value }
func (target *Base[T]) Read() T { if target == nil || target.self == nil { return target.read() }; return target.self.read() }
func (target *Base[T]) Choose(value T) T { if target == nil || target.self == nil { return target.choose(value) }; return target.self.choose(value) }
func (target *Base[T]) Indirect() T { return target.Read() }

type childDispatch[U any] interface { label(U) U }
type Child[U any] struct { Base[U]; Replacement U; labelSelf childDispatch[U] }
func NewChild[U any](value, replacement U) *Child[U] { target := &Child[U]{}; initBase(&target.Base, value); target.Replacement = replacement; target.self = target; target.labelSelf = target; return target }
func (target *Child[U]) read() U { return target.Replacement }
func (target *Child[U]) choose(value U) U { return target.Replacement }
func (target *Child[U]) label(value U) U { return value }
func (target *Child[U]) Label(value U) U { return target.labelSelf.label(value) }
func (target *Child[U]) BaseRead() U { return target.Base.read() }

type GrandChild[V any] struct { Child[V] }
func NewGrandChild[V any](value, replacement V) *GrandChild[V] { target := &GrandChild[V]{}; initBase(&target.Base, value); target.Replacement = replacement; target.self = target; target.labelSelf = target; return target }
func (target *GrandChild[V]) label(value V) V { return target.Replacement }

type Concrete struct { Base[string] }
func NewConcrete(value string) *Concrete { target := &Concrete{}; initBase(&target.Base, value); target.self = target; return target }
func (target *Concrete) read() string { return "concrete" }

type Middle[A, B any] struct { Base[B] }
func initMiddle[A, B any](target *Middle[A, B], value B) { initBase(&target.Base, value); target.self = target }
func (target *Middle[A, B]) read() B { return target.Base.read() }
type Leaf[X any] struct { Middle[int, X]; Replacement X }
func NewLeaf[X any](value, replacement X) *Leaf[X] { target := &Leaf[X]{}; initMiddle(&target.Middle, value); target.Replacement = replacement; target.self = target; return target }
func (target *Leaf[X]) read() X { return target.Replacement }

type comparableDispatch[K comparable] interface { equal(K, K) bool }
type ComparableBase[K comparable] struct { self comparableDispatch[K] }
func (target *ComparableBase[K]) equal(left, right K) bool { return left == right }
func (target *ComparableBase[K]) Equal(left, right K) bool { return target.self.equal(left, right) }
type ComparableChild[K comparable] struct { ComparableBase[K] }
func NewComparableChild[K comparable]() *ComparableChild[K] { target := &ComparableChild[K]{}; target.self = target; return target }
func (target *ComparableChild[K]) equal(left, right K) bool { return target.ComparableBase.equal(left, right) }

func GenericBehavior(value string) string { child := NewChild(value, value+"-child"); base := &child.Base; return base.ConstructorSeen + "/" + base.Indirect() + "/" + base.Choose(value+"-ignored") + "/" + child.BaseRead() }
func InterfaceBehavior(value string) string { var reader Reader[string] = NewChild(value, value+"-interface"); return reader.Read() }
func ChildOwnedBehavior(value string) string { grand := NewGrandChild(value, value+"-grand"); return grand.Child.Label(value+"-argument") }
func MethodValueBehavior(value string) string { child := NewChild(value, value+"-method"); read := child.Base.Read; return read() }
func ConcreteBehavior(value string) string { concrete := NewConcrete(value); base := &concrete.Base; return base.ConstructorSeen + "/" + base.Read() }
func MultiLevelBehavior(value string) string { leaf := NewLeaf(value, value+"-leaf"); base := &leaf.Middle.Base; return base.ConstructorSeen + "/" + base.Indirect() }
func ComparableBehavior(left, right string) bool { child := NewComparableChild[string](); return child.ComparableBase.Equal(left, right) }
`
	testSource := `package genericvirtual_test
import (
  "testing"
  generated "genericvirtual.test"
  reference "genericvirtual.test/reference"
)
func TestGenericVirtualBehavior(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := generated.GenericBehavior(value), reference.GenericBehavior(value); got != want { t.Errorf("GenericBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.InterfaceBehavior(value), reference.InterfaceBehavior(value); got != want { t.Errorf("InterfaceBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.ChildOwnedBehavior(value), reference.ChildOwnedBehavior(value); got != want { t.Errorf("ChildOwnedBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.MethodValueBehavior(value), reference.MethodValueBehavior(value); got != want { t.Errorf("MethodValueBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.ConcreteBehavior(value), reference.ConcreteBehavior(value); got != want { t.Errorf("ConcreteBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.MultiLevelBehavior(value), reference.MultiLevelBehavior(value); got != want { t.Errorf("MultiLevelBehavior(%q) = %q, Go = %q", value, got, want) }
  }
  for _, values := range [][2]string{{"", ""}, {"same", "same"}, {"left", "right"}, {"温泉", "温泉"}} {
    if got, want := generated.ComparableBehavior(values[0], values[1]), reference.ComparableBehavior(values[0], values[1]); got != want { t.Errorf("ComparableBehavior(%q, %q) = %v, Go = %v", values[0], values[1], got, want) }
  }
  child := generated.NewChild("api", "api-child")
  base := generated.UpcastChildToBase(child)
  if got := base.Read(); got != "api-child" { t.Errorf("external Go virtual dispatch = %q", got) }
  var zero generated.Base[string]
  if got := zero.Read(); got != "" { t.Errorf("zero-value direct fallback = %q", got) }
  if got := zero.Choose("fallback"); got != "fallback" { t.Errorf("zero-value argument fallback = %q", got) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericvirtual.test", generated, referenceSource, testSource)
}

func TestLinkedGenericClassVirtualDispatchMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	baseSource := filepath.Join(temporary, "base.km")
	entrySource := filepath.Join(temporary, "entry.km")
	if err := os.WriteFile(baseSource, []byte(`
class Base<T> {
  constructor(protected value: T) {}
  public virtual function read(): T { return this.value; }
}
class Child<U> extends Base<U> {
  constructor(value: U, public replacement: U) { super(value); }
  public override function read(): U { return this.replacement; }
}
function makeChild(value: string): Child<string> {
  return new Child<string>(value, value + "-linked");
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entrySource, []byte(`
import { Base, Child, makeChild } from "./base";
function linked(value: string): string {
  const child: Child<string> = makeChild(value);
  const base: Base<string> = child;
  return base.read();
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entrySource}, "genericvirtuallinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	referenceSource := `package reference
type dispatch[T any] interface { read() T }
type Base[T any] struct { self dispatch[T]; value T }
func (target *Base[T]) read() T { return target.value }
func (target *Base[T]) Read() T { return target.self.read() }
type Child[U any] struct { Base[U]; replacement U }
func (target *Child[U]) read() U { return target.replacement }
func NewChild[U any](value, replacement U) *Child[U] { target := &Child[U]{Base: Base[U]{value: value}, replacement: replacement}; target.self = target; return target }
func Linked(value string) string { child := NewChild(value, value+"-linked"); return child.Base.Read() }
`
	testSource := `package genericvirtuallinked
import (
  "testing"
  reference "genericvirtual-linked.test/reference"
)
func TestLinkedGenericVirtual(t *testing.T) {
  for _, value := range []string{"", "linked", "温泉卵"} {
    if got, want := linked(value), reference.Linked(value); got != want { t.Errorf("linked(%q) = %q, Go = %q", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericvirtual-linked.test", generated, referenceSource, testSource)
}
