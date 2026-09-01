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
		"func __ontamaInitBox[T any](this *Box[T], value T)",
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
