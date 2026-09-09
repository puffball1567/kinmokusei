package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassFieldInitializersMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"model.km": `
let trace = 0;
const seed = 7;
function mark(value: int): int { trace = trace * 10 + value; return value; }
function reset(): void { trace = 0; }
function log(): int { return trace; }
class Base {
  public first: int = mark(1);
  public named: int = seed;
  constructor(seed: string) { mark(2); mark(this.kind()); }
  public virtual function kind(): int { return 5; }
}
class Derived extends Base {
  public second: int = mark(3);
  constructor(seed: string) { super(seed); mark(4); mark(this.kind()); }
  public override function kind(): int { return 6; }
}
class Grand extends Derived {
  public third: int = mark(7);
  constructor() { super("ignored"); mark(8); }
}
class Leaf { constructor(public value: int) {} }
class Bucket<T> {
  public items: T[] = [];
  public leaf: Leaf = new Leaf(9);
  public callback: (value: int) => int = (value: int): int => { return value + seed; };
}
class Nested<T> extends Bucket<T[]> {}
class Small extends Leaf { constructor() { super(11); } }
class Owner { public leaf: Leaf = new Small(); }
constraint Number = ~int8 | ~int64;
class Counter<T extends Number> { public value: T = T(2); }
class PrivateDefaults {
  private value: int = PrivateDefaults.initial();
  private static function initial(): int { return seed; }
  public function read(): int { return this.value; }
}
function fail(): int { mark(2); throw new Exception("initialization failed"); }
class Broken {
  public before: int = mark(1);
  public failed: int = fail();
  public after: int = mark(3);
  constructor() { mark(4); }
}
class BrokenBase { constructor() { fail(); } }
class Unreached extends BrokenBase { public value: int = mark(3); }
function crash(): int { mark(2); let values: int[] = []; return values[0]; }
class Raw { public before: int = mark(1); public value: int = crash(); public after: int = mark(3); }
`,
		"entry.km": `
import { Base, Derived, Grand, Leaf, Bucket, Nested, Small, Owner, Counter, PrivateDefaults, Broken, Unreached, Raw, reset, log } from "./model";
const seed = 99;
function booleanInt(value: boolean): int { if (value) { return 1; } return 0; }
function Order(): int[] {
  reset(); const value = new Grand(); const base: Base = value;
  const [again, ok] = base as? Grand;
  return [log(), value.first, value.named, value.second, value.third, base.kind(), booleanInt(ok), booleanInt(again === value)];
}
function Independent(value: int): int[] {
  const first = new Bucket<int>(); const second = new Bucket<int>();
  first.items = append(first.items, value); first.leaf.value = value;
  const nested = new Nested<int>(); nested.items = append(nested.items, [value]);
  const owner = new Owner(); const [small, ok] = owner.leaf as? Small;
  return [len(first.items), len(second.items), first.leaf.value, second.leaf.value, first.callback(value), nested.items[0][0], booleanInt(ok), small.value, int(new Counter<int8>().value), new PrivateDefaults().read()];
}
function PanicTrace(base: boolean): int { reset(); try { if (base) { const value = new Unreached(); } else { const value = new Broken(); } } catch (failure: Exception) {} return log(); }
function RawPanic(): void { reset(); const value = new Raw(); }
function CurrentTrace(): int { return log(); }
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "fieldinitializer")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "__kinmokuseiFieldsBucket[T any](this *Bucket[T])") {
		t.Fatalf("missing isolated generic field initializer:\n%s", generated)
	}
	reference := `package reference
var trace int
const seed = 7
func mark(value int) int { trace = trace*10 + value; return value }
type Base struct { First, Named int }
type Derived struct { Base; Second int }
type Grand struct { Derived; Third int }
func (*Base) Kind() int { return 5 }
func (*Derived) Kind() int { return 6 }
func initBase(value *Base) { value.First = mark(1); value.Named = seed; mark(2); mark(value.Kind()) }
func initDerived(value *Derived) { initBase(&value.Base); value.Second = mark(3); mark(4); mark(value.Kind()) }
func Order() []int { trace = 0; value := &Grand{}; initDerived(&value.Derived); value.Third = mark(7); mark(8); var base interface{Kind() int} = value; again, ok := base.(*Grand); return []int{trace,value.First,value.Named,value.Second,value.Third,base.Kind(),boolean(ok),boolean(again == value)} }
func boolean(value bool) int { if value { return 1 }; return 0 }
type Leaf struct { Value int }
func (leaf *Leaf) read() int { return leaf.Value }
type Small struct { Leaf }
type Owner struct { Leaf interface{read() int} }
func owner() *Owner { return &Owner{&Small{Leaf{11}}} }
type Counter[T ~int8 | ~int64] struct { Value T }
func counter[T ~int8 | ~int64]() *Counter[T] { return &Counter[T]{T(2)} }
type PrivateDefaults struct { value int }
func privateInitial() int { return seed }
func privateDefaults() *PrivateDefaults { return &PrivateDefaults{privateInitial()} }
type Bucket[T any] struct { Items []T; Leaf *Leaf; Callback func(int) int }
func bucket[T any]() *Bucket[T] { return &Bucket[T]{Items:[]T{},Leaf:&Leaf{9},Callback:func(value int) int { return value+seed }} }
func Independent(value int) []int { first,second := bucket[int](),bucket[int](); first.Items = append(first.Items,value); first.Leaf.Value = value; nested := bucket[[]int](); nested.Items = append(nested.Items,[]int{value}); owned := owner(); small,ok := owned.Leaf.(*Small); return []int{len(first.Items),len(second.Items),first.Leaf.Value,second.Leaf.Value,first.Callback(value),nested.Items[0][0],boolean(ok),small.Value,int(counter[int8]().Value),privateDefaults().value} }
func fail() int { mark(2); panic("initialization failed") }
func PanicTrace(base bool) (result int) { trace = 0; defer func() { recover(); result = trace }(); if base { fail(); mark(3) } else { mark(1); fail(); mark(3); mark(4) }; return trace }
func RawPanic() { trace = 0; mark(1); mark(2); values := []int{}; _ = values[0]; mark(3) }
func CurrentTrace() int { return trace }
`
	comparison := `package fieldinitializer_test
import (
 "reflect"
 "testing"
 generated "class-field-initializer.test"
 reference "class-field-initializer.test/reference"
)
func panicOutcome(call func(), read func() int) (trace int, panicked bool) { defer func() { panicked = recover() != nil; trace = read() }(); call(); return }
func TestInitializers(t *testing.T) {
 for i:=0;i<3;i++ { if got,want := generated.Order(),reference.Order(); !reflect.DeepEqual(got,want) { t.Errorf("order=%v want=%v",got,want) } }
 for _,value := range []int{-1,0,1,99} { if got,want := generated.Independent(value),reference.Independent(value); !reflect.DeepEqual(got,want) { t.Errorf("independent=%v want=%v",got,want) } }
 for _,base := range []bool{false,true} { if got,want := generated.PanicTrace(base),reference.PanicTrace(base); got!=want { t.Errorf("panic trace=%v want=%v",got,want) } }
 gotTrace,gotPanic := panicOutcome(generated.RawPanic,generated.CurrentTrace); wantTrace,wantPanic := panicOutcome(reference.RawPanic,reference.CurrentTrace)
 if gotTrace!=wantTrace || gotPanic!=wantPanic { t.Errorf("raw panic trace=(%d,%t) want=(%d,%t)",gotTrace,gotPanic,wantTrace,wantPanic) }
 first,second := generated.NewBucket[string](),generated.NewBucket[string](); first.Items = append(first.Items,"hello"); first.Leaf.Value = 17
 wantFirst,wantSecond := reference.Independent(17),reference.Independent(9)
 if len(first.Items)!=wantFirst[0] || len(second.Items)!=wantSecond[1] || first.Leaf.Value!=wantFirst[2] || second.Leaf.Value!=wantSecond[3] { t.Error("external Go constructor initialization") }
}
`
	runGeneratedGoDifferentialTest(t, root, "class-field-initializer.test", generated, reference, comparison)
}
