package codegen

import (
	"strings"
	"testing"
)

func TestGenericClassGeneratesGoTypeParametersAndInstantiations(t *testing.T) {
	generated := string(generateCheckedSource(t, `
interface Reader<T> { function read(): T; }
class Box<T> {
  constructor(public value: T) {}
  public function get(): T { return this.value; }
  public function set(value: T): void { this.value = value; }
  public static function make(value: T): Box<T> { return new Box<T>(value); }
}
class Key<K extends comparable> { constructor(public value: K) {} }
class Value<T> implements Reader<T> {
  constructor(private value: T) {}
  public function read(): T { return this.value; }
}
function use(): string {
  const box = Box.make("first");
  box.set("second");
  const key = new Key<int>(7);
  const reader: Reader<string> = new Value<string>(box.get());
  return reader.read();
}
	`))
	for _, expected := range []string{
		"type Box[T any] struct",
		`Value T ` + "`json:\"value\"`",
		"func __kinmokuseiInitBox[T any](this *Box[T], value T)",
		"func NewBox[T any](value T) *Box[T]",
		"func (this *Box[T]) Get() T",
		"func (this *Box[T]) Set(value T)",
		"func BoxMake[T any](value T) *Box[T]",
		"type Key[K comparable] struct",
		`var box = BoxMake("first")`,
		"var key = NewKey[int](7)",
		`var reader Reader[string] = NewValue[string](box.Get())`,
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}
}

func TestGenericClassInheritanceGeneratesInstantiatedBaseAndConversions(t *testing.T) {
	generated := string(generateCheckedSource(t, `
class Base<T> {
  constructor(protected value: T) {}
  public function get(): T { return this.value; }
}
class Child<U> extends Base<U> {
  constructor(value: U) { super(value); }
  public function read(): U { return super.get(); }
}
class Concrete extends Base<int> {
  constructor(value: int) { super(value); }
}
function use(value: Child<string>): Base<string> { return value; }
`))
	for _, expected := range []string{
		"type Child[U any] struct",
		"Base[U]",
		"func __kinmokuseiUpcastChildToBase[U any](value *Child[U]) *Base[U]",
		"func __kinmokuseiDowncastBaseToChild[U any](value *Base[U]) (*Child[U], bool)",
		"func UpcastChildToBase[U any](value *Child[U]) *Base[U]",
		"func __kinmokuseiInitChild[U any](this *Child[U], value U)",
		"__kinmokuseiInitBase(&this.Base, value)",
		"func (this *Child[U]) Read() U",
		"return this.Base.Get()",
		"type Concrete struct",
		"Base[int]",
		"return __kinmokuseiUpcastChildToBase[string](value)",
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}
}

func TestGenericClassVirtualDispatchGeneratesParameterizedInterface(t *testing.T) {
	generated := string(generateCheckedSource(t, `
class Base<T> {
  constructor(protected value: T) {}
  public virtual function read(): T { return this.value; }
  public virtual function choose(value: T): T { return value; }
}
class Child<U> extends Base<U> {
  constructor(value: U) { super(value); }
  public override function read(): U { return super.read(); }
}
class Concrete extends Base<string> {
  constructor(value: string) { super(value); }
  public override function read(): string { return "concrete"; }
}
`))
	for _, expected := range []string{
		"type __kinmokuseiBaseVirtual[T any] interface",
		"__kinmokuseiBaseRead() T",
		"__kinmokuseiBaseChoose(value T) T",
		"__kinmokuseiBaseSelf __kinmokuseiBaseVirtual[T]",
		"func (this *Base[T]) Read() T",
		"func (this *Base[T]) __kinmokuseiBaseRead() T",
		"func (this *Child[U]) __kinmokuseiBaseRead() U",
		"this.__kinmokuseiBaseSelf = this",
		"func (this *Concrete) __kinmokuseiBaseRead() string",
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}
}
