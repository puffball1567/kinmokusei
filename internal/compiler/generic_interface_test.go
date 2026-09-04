package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericInterfacesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "generic_interface.km")
	if err := os.WriteFile(source, []byte(`
interface Transformer<T, U> {
  function transform(value: T): U;
}

interface Source<T> {
  function read(): T;
}

interface Marker<T> {
  function kind(): string;
}

interface Loader<T> {
  function load(): Result<T>;
}

class Length implements Transformer<string, int> {
  public bias: int;
  constructor(bias: int) { this.bias = bias; }
  public function transform(value: string): int { return len(value) + this.bias; }
}

class Offset implements Transformer<int, int> {
  public delta: int;
  constructor(delta: int) { this.delta = delta; }
  public function transform(value: int): int { return value + this.delta; }
}

class IntSource implements Source<int> {
  public value: int;
  constructor(value: int) { this.value = value; }
  public function read(): int { return this.value; }
}

class Both implements Marker<int>, Marker<string> {
  public function kind(): string { return "both"; }
}

class IntLoader implements Loader<int> {
  public value: int;
  constructor(value: int) { this.value = value; }
  public function load(): Result<int> { return ok(this.value); }
}

function Apply<T, U>(transformer: Transformer<T, U>, value: T): U {
  return transformer.transform(value);
}

function ApplyLengths(values: string[], transformer: Transformer<string, int>): int[] {
  let result: int[] = [];
  for (const value of values) { result = append(result, transformer.transform(value)); }
  return result;
}

function Read(source: Source<int>): int { return source.read(); }
function Select(flag: boolean, left: Source<int>, right: Source<int>): Source<int> {
  if (flag) { return left; }
  return right;
}
function Same(left: Source<int>, right: Source<int>): boolean { return left === right; }
function Lookup(values: Map<Source<int>, string>, key: Source<int>): string { return values[key]; }
function Present(value: int): Result<Source<int>> { return ok(new IntSource(value)); }
function KindInt(value: Marker<int>): string { return value.kind(); }
function KindString(value: Marker<string>): string { return value.kind(); }
function Load(value: Loader<int>): Result<int> { return value.load(); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "genericinterface")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"type Transformer[T any, U any] interface",
		"Transform(value T) U",
		"type Source[T any] interface",
		"type Loader[T any] interface",
		"Load() (T, error)",
		"func Apply[T any, U any](transformer Transformer[T, U], value T) U",
		"func Present(value int) (Source[int], error)",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Transformer[T, U any] interface { Transform(T) U }
type Source[T any] interface { Read() T }
type Marker[T any] interface { Kind() string }
type Loader[T any] interface { Load() (T, error) }

type Length struct { Bias int }
func NewLength(bias int) *Length { return &Length{Bias: bias} }
func (value *Length) Transform(input string) int { return len(input) + value.Bias }
type Offset struct { Delta int }
func NewOffset(delta int) *Offset { return &Offset{Delta: delta} }
func (value *Offset) Transform(input int) int { return input + value.Delta }
type IntSource struct { Value int }
func NewIntSource(value int) *IntSource { return &IntSource{Value: value} }
func (value *IntSource) Read() int { return value.Value }
type Both struct{}
func NewBoth() *Both { return &Both{} }
func (*Both) Kind() string { return "both" }
type IntLoader struct { Value int }
func NewIntLoader(value int) *IntLoader { return &IntLoader{Value: value} }
func (value *IntLoader) Load() (int, error) { return value.Value, nil }

func Apply[T, U any](transformer Transformer[T, U], value T) U { return transformer.Transform(value) }
func ApplyLengths(values []string, transformer Transformer[string, int]) []int { result := []int{}; for _, value := range values { result = append(result, transformer.Transform(value)) }; return result }
func Read(source Source[int]) int { return source.Read() }
func Select(flag bool, left, right Source[int]) Source[int] { if flag { return left }; return right }
func Same(left, right Source[int]) bool { return left == right }
func Lookup(values map[Source[int]]string, key Source[int]) string { return values[key] }
func Present(value int) (Source[int], error) { return NewIntSource(value), nil }
func KindInt(value Marker[int]) string { return value.Kind() }
func KindString(value Marker[string]) string { return value.Kind() }
func Load(value Loader[int]) (int, error) { return value.Load() }
`
	testSource := `package genericinterface_test

import (
  "reflect"
  "testing"
  generated "genericinterface.test"
  reference "genericinterface.test/reference"
)

func TestGenericInterfaceBehavior(t *testing.T) {
  for _, item := range []struct { text string; bias int }{{"", 0}, {"onsen", -1}, {"温泉卵", 2}} {
    if got, want := generated.Apply(generated.NewLength(item.bias), item.text), reference.Apply(reference.NewLength(item.bias), item.text); got != want { t.Errorf("Apply(%q, %d) = %d, Go = %d", item.text, item.bias, got, want) }
  }
  for _, item := range [][2]int{{0, 0}, {-4, 7}, {9, -3}} {
    if got, want := generated.Apply(generated.NewOffset(item[1]), item[0]), reference.Apply(reference.NewOffset(item[1]), item[0]); got != want { t.Errorf("Apply offset %v = %d, Go = %d", item, got, want) }
  }
  gotLengths := generated.ApplyLengths([]string{"", "a", "温泉"}, generated.NewLength(1))
  wantLengths := reference.ApplyLengths([]string{"", "a", "温泉"}, reference.NewLength(1))
  if !reflect.DeepEqual(gotLengths, wantLengths) { t.Errorf("ApplyLengths = %v, Go = %v", gotLengths, wantLengths) }

  gotLeft, gotRight := generated.NewIntSource(3), generated.NewIntSource(8)
  wantLeft, wantRight := reference.NewIntSource(3), reference.NewIntSource(8)
  for _, flag := range []bool{false, true} {
    if got, want := generated.Read(generated.Select(flag, gotLeft, gotRight)), reference.Read(reference.Select(flag, wantLeft, wantRight)); got != want { t.Errorf("Select(%v) = %d, Go = %d", flag, got, want) }
  }
  if got, want := generated.Same(gotLeft, gotLeft), reference.Same(wantLeft, wantLeft); got != want { t.Errorf("Same identity = %v, Go = %v", got, want) }
  if got, want := generated.Same(gotLeft, gotRight), reference.Same(wantLeft, wantRight); got != want { t.Errorf("Same distinct = %v, Go = %v", got, want) }
  if got, want := generated.Lookup(map[generated.Source[int]]string{gotLeft: "left"}, gotLeft), reference.Lookup(map[reference.Source[int]]string{wantLeft: "left"}, wantLeft); got != want { t.Errorf("Lookup = %q, Go = %q", got, want) }

  gotPresent, gotErr := generated.Present(-7)
  wantPresent, wantErr := reference.Present(-7)
  if generated.Read(gotPresent) != reference.Read(wantPresent) || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present = (%d, %v), Go = (%d, %v)", generated.Read(gotPresent), gotErr, reference.Read(wantPresent), wantErr) }

  gotBoth, wantBoth := generated.NewBoth(), reference.NewBoth()
  if got, want := generated.KindInt(gotBoth)+generated.KindString(gotBoth), reference.KindInt(wantBoth)+reference.KindString(wantBoth); got != want { t.Errorf("multi-instantiation marker = %q, Go = %q", got, want) }

  gotLoad, gotLoadErr := generated.Load(generated.NewIntLoader(-11))
  wantLoad, wantLoadErr := reference.Load(reference.NewIntLoader(-11))
  if gotLoad != wantLoad || (gotLoadErr == nil) != (wantLoadErr == nil) { t.Errorf("generic Result method = (%d, %v), Go = (%d, %v)", gotLoad, gotLoadErr, wantLoad, wantLoadErr) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericinterface.test", generated, referenceSource, testSource)
}

func TestLinkedGenericInterfaceMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "contract.km")
	entry := filepath.Join(temporary, "entry.km")
	if err := os.WriteFile(dependency, []byte(`interface Reader<T> { function read(): T; } function consume<T>(reader: Reader<T>): T { return reader.read(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Reader, consume } from "./contract"; class Value implements Reader<string> { public text: string; constructor(text: string) { this.text = text; } public function read(): string { return this.text; } } function linked(text: string): string { return consume<string>(new Value(text)); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "genericinterfacelinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
type Reader[T any] interface { Read() T }
type Value struct { Text string }
func (value *Value) Read() string { return value.Text }
func consume[T any](reader Reader[T]) T { return reader.Read() }
func Linked(text string) string { return consume[string](&Value{Text: text}) }
`
	testSource := `package genericinterfacelinked
import (
  "testing"
  reference "genericinterface-linked.test/reference"
)
func TestLinked(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := linked(value), reference.Linked(value); got != want { t.Errorf("linked(%q) = %q, Go = %q", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "genericinterface-linked.test", generated, referenceSource, testSource)
}
