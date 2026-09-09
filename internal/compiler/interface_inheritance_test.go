package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkedInterfaceInheritanceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"root.km": `interface Reader<T> { function read(): T; } interface Writer<T> { function write(value: T): void; } interface Named { function name(): string; }`,
		"store.km": `
import { Reader, Writer, Named } from "./root";
interface Store<T> extends Left<T>, Right<T>, Writer<T>, Named { function all(...values: T[]): T[]; function load(): Result<T>; }
interface Left<T> extends Reader<T> {}
interface Right<T> extends Reader<T> {}
class Box<T> implements Store<T> {
  constructor(private value: T) {}
  public virtual function read(): T { return this.value; }
  public function write(value: T): void { this.value = value; }
  public function name(): string { return "box"; }
  public function all(...values: T[]): T[] { return values; }
  public function load(): Result<T> { return ok(this.read()); }
}
interface Transform<A, B> { function apply(value: A): B; }
interface Reordered<T, U> extends Transform<U, T> {}
`,
		"entry.km": `
import { Reader } from "./root";
import { Store, Box, Reordered } from "./store";
class Doubled extends Box<int> {
  constructor(value: int) { super(value); }
  public override function read(): int { return super.read() * 2; }
}
class Length implements Reordered<int, string> { public function apply(value: string): int { return len(value); } }
interface Marker<T, U> {}
interface Multi extends Marker<int, string>, Marker<string, int> {}
class Both implements Multi {}
function Pick<T, U>(marker: Marker<T, U>): int { return 3; }
function Explicit(): int { return Pick<string>(new Both()) + Pick<int, string>(new Both()); }
function Read<T>(value: Reader<T>): T { return value.read(); }
function Upcast(value: Store<int>): Reader<int> { return value; }
function Nullable(value: Store<int> | null): Reader<int> | null { return value; }
function Make(value: int): Store<int> { return new Doubled(value); }
function Use(value: int): int {
  const box = new Doubled(value);
  const store: Store<int> = box;
  const parent: Reader<int> = box;
  const bound = store.read;
  store.write(value + 1);
  return Read(store) + Read(box) + parent.read() + bound() + len(store.all(1, 2, 3)) + len(store.name());
}
function Load(value: Store<int>): Result<int> { return value.load(); }
function Reorder(value: string): int { const transform: Reordered<int, string> = new Length(); return transform.apply(value); }
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "interfaceinheritance")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "interface {") {
		t.Fatalf("missing Go interfaces:\n%s", generated)
	}
	reference := `package reference
type Reader[T any] interface { Read() T }
type Writer[T any] interface { Write(T) }
type Named interface { Name() string }
type Left[T any] interface { Reader[T] }
type Right[T any] interface { Reader[T] }
type Store[T any] interface { Left[T]; Right[T]; Writer[T]; Named; All(...T) []T; Load() (T, error) }
type Doubled struct { value int }
func (b *Doubled) Read() int { return b.value*2 }
func (b *Doubled) Write(value int) { b.value = value }
func (b *Doubled) Name() string { return "box" }
func (b *Doubled) All(values ...int) []int { return values }
func (b *Doubled) Load() (int, error) { return b.Read(), nil }
func Read[T any](r Reader[T]) T { return r.Read() }
func Upcast(s Store[int]) Reader[int] { return s }
func Nullable(s Store[int]) Reader[int] { return s }
func Make(value int) Store[int] { return &Doubled{value} }
func Use(value int) int { box := &Doubled{value}; var store Store[int] = box; var parent Reader[int] = box; bound := store.Read; store.Write(value+1); return Read(store)+Read(box)+parent.Read()+bound()+len(store.All(1,2,3))+len(store.Name()) }
func Load(value Store[int]) (int, error) { return value.Load() }
func Reorder(value string) int { return len(value) }
type Marker[T, U any] interface{}
type Both struct{}
func Pick[T, U any](marker Marker[T, U]) int { return 3 }
func Explicit() int { return Pick[string, int](&Both{}) + Pick[int, string](&Both{}) }
`
	comparison := `package interfaceinheritance_test
import (
 "testing"
 generated "interface-inheritance-linked.test"
 reference "interface-inheritance-linked.test/reference"
)
func TestInterfaces(t *testing.T) {
 if generated.Explicit() != reference.Explicit() { t.Error("explicit and partial contract selection") }
 for _, n := range []int{-10,0,1,20} {
  if got, want := generated.Use(n), reference.Use(n); got != want { t.Errorf("Use(%d)=%d want %d", n, got, want) }
  g, r := generated.Make(n), reference.Make(n)
  if generated.Upcast(g).Read() != reference.Upcast(r).Read() { t.Error("upcast dispatch") }
  if generated.Upcast(g) != generated.Upcast(g) { t.Error("upcast identity") }
  gv, ge := generated.Load(g); rv, re := reference.Load(r); if gv != rv || (ge == nil) != (re == nil) { t.Error("Result contract") }
 }
 if generated.Nullable(nil) != nil || reference.Nullable(nil) != nil { t.Error("nullable interface upcast") }
 for _, value := range []string{"", "hello", "温泉"} { if generated.Reorder(value) != reference.Reorder(value) { t.Error("reordered parameters") } }
}
`
	runGeneratedGoDifferentialTest(t, root, "interface-inheritance-linked.test", generated, reference, comparison)
}
