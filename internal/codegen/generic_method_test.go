package codegen

import (
	"strings"
	"testing"
)

func TestGenericMethodsLowerToTypedGoHelpers(t *testing.T) {
	generated := string(generateCheckedSource(t, `
constraint Integer = ~int | ~int8;
class Box<T> {
  constructor(public value: T) {}
  public function keep<U>(marker: U): T { return this.value; }
  public static function pair<U>(left: U, right: T): {left: U, right: T} {
    return {left: left, right: right};
  }
}
struct Holder<T> {
  public value: T;
  public function echo<U>(value: U): U { return value; }
  public pointer function update<U extends Integer>(value: T, marker: U): U {
    this.value = value;
    return marker;
  }
}
function use(box: Box<string>, holder: Holder<string>): string {
  const kept = box.keep<int>(1);
  const pair = Box.pair<int>(2, kept);
  holder.update<int8>(pair.right, int8(3));
  return holder.echo(pair.right);
}
`))
	for _, expected := range []string{
		"func BoxKeep[U any, T any](this *Box[T], marker U) T",
		"func BoxPair[U any, T any](left U, right T) struct",
		"func HolderEcho[U any, T any](this Holder[T], value U) U",
		"func HolderUpdate[U Integer, T any](this *Holder[T], value T, marker U) U",
		"var kept = BoxKeep[int](box, 1)",
		"var pair = BoxPair[int](2, kept)",
		"HolderUpdate[int8](&holder, pair.Right, int8(3))",
		"return HolderEcho(holder, pair.Right)",
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}
}

func TestInheritedGenericMethodCallsUseSafeOwnerConversions(t *testing.T) {
	generated := string(generateCheckedSource(t, `
class Base<T> {
  constructor(protected value: T) {}
  public function echo<U>(value: U): U { return value; }
}
class Middle<V> extends Base<V> {
  constructor(value: V) { super(value); }
}
class Child<W> extends Middle<W> {
  constructor(value: W) { super(value); }
  public function relay<U>(value: U): U { return super.echo(value); }
}
function use(child: Child<string>): string { return child.echo("ok"); }
`))
	for _, expected := range []string{
		"func BaseEcho[U any, T any](this *Base[T], value U) U",
		"return BaseEcho(__kinmokuseiUpcastMiddleToBase[W](&this.Middle), value)",
		`return BaseEcho(__kinmokuseiUpcastChildToBase[string](child), "ok")`,
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}
}
