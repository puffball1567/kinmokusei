package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinedTypeReceiverMethodsMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "defined_type_method.otm")
	if err := os.WriteFile(source, []byte(`
type Counter = distinct int;
type Label = distinct string;
type Values = distinct int[];

public function plus(this: Counter, delta: Counter): Counter { return this + delta; }
public function add(this: *Counter, delta: Counter): void { *this += delta; }
public function isNil(this: *Counter): boolean { return this === nil; }
public function text(this: Label): string { return string(this); }
public function size(this: Label): int { return len(this); }
public function push(this: *Values, value: int): void { *this = append(*this, value); }
public function sum(this: Values): int { let total: int = 0; for (const value of this) { total += value; } return total; }
public function present(this: Counter): Result<Counter> { return ok(this); }

function ValueBehavior(value: Counter, delta: Counter): int { return int(value.plus(delta)); }
function PointerBehavior(value: Counter, first: Counter, second: Counter): int {
  let copy = value;
  copy.add(first);
  const add = copy.add;
  add(second);
  return int(copy);
}
function ValueMethodCapture(value: Counter, delta: Counter): int {
  const plus = value.plus;
  value += 10;
  return int(plus(delta));
}
function NilBehavior(): boolean { let value: *Counter = nil; return value.isNil(); }
function LabelBehavior(value: Label): string { return value.text(); }
function LabelSize(value: Label): int { return value.size(); }
function ValuesBehavior(values: int[], tail: int): int {
  let typed = Values(values);
  typed.push(tail);
  return typed.sum() * 100 + len(typed);
}
function PresentBehavior(value: Counter): Result<Counter> { return value.present(); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "definedtypemethod")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"func (this Counter) Plus(delta Counter) Counter",
		"func (this *Counter) Add(delta Counter)",
		"func (this *Counter) IsNil() bool",
		"func (this *Values) Push(value int)",
		"func (this Values) Sum() int",
		"func (this Counter) Present() (Counter, error)",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Counter int
type Label string
type Values []int

func (value Counter) Plus(delta Counter) Counter { return value + delta }
func (value *Counter) Add(delta Counter) { *value += delta }
func (value *Counter) IsNil() bool { return value == nil }
func (value Label) Text() string { return string(value) }
func (value Label) Size() int { return len(value) }
func (values *Values) Push(value int) { *values = append(*values, value) }
func (values Values) Sum() int { total := 0; for _, value := range values { total += value }; return total }
func (value Counter) Present() (Counter, error) { return value, nil }

func ValueBehavior(value, delta Counter) int { return int(value.Plus(delta)) }
func PointerBehavior(value, first, second Counter) int { copy := value; copy.Add(first); add := copy.Add; add(second); return int(copy) }
func ValueMethodCapture(value, delta Counter) int { plus := value.Plus; value += 10; return int(plus(delta)) }
func NilBehavior() bool { var value *Counter; return value.IsNil() }
func LabelBehavior(value Label) string { return value.Text() }
func LabelSize(value Label) int { return value.Size() }
func ValuesBehavior(values []int, tail int) int { typed := Values(values); typed.Push(tail); return typed.Sum()*100 + len(typed) }
func PresentBehavior(value Counter) (Counter, error) { return value.Present() }
`
	testSource := `package definedtypemethod_test

import (
  "reflect"
  "testing"
  generated "definedtypemethod.test"
  reference "definedtypemethod.test/reference"
)

type generatedText interface { Text() string }
type referenceText interface { Text() string }
type generatedAdder interface { Add(generated.Counter) }
type referenceAdder interface { Add(reference.Counter) }

func TestDefinedTypeReceiverMethodBehavior(t *testing.T) {
  for _, item := range [][3]int{{0, 0, 0}, {-5, 2, 8}, {11, -4, 9}} {
    gotValue, wantValue := generated.Counter(item[0]), reference.Counter(item[0])
    gotFirst, wantFirst := generated.Counter(item[1]), reference.Counter(item[1])
    gotSecond, wantSecond := generated.Counter(item[2]), reference.Counter(item[2])
    if got, want := generated.ValueBehavior(gotValue, gotFirst), reference.ValueBehavior(wantValue, wantFirst); got != want { t.Errorf("ValueBehavior(%v) = %d, Go = %d", item, got, want) }
    if got, want := generated.PointerBehavior(gotValue, gotFirst, gotSecond), reference.PointerBehavior(wantValue, wantFirst, wantSecond); got != want { t.Errorf("PointerBehavior(%v) = %d, Go = %d", item, got, want) }
    if got, want := generated.ValueMethodCapture(gotValue, gotFirst), reference.ValueMethodCapture(wantValue, wantFirst); got != want { t.Errorf("ValueMethodCapture(%v) = %d, Go = %d", item, got, want) }
    gotResult, gotErr := generated.PresentBehavior(gotValue)
    wantResult, wantErr := reference.PresentBehavior(wantValue)
    if int(gotResult) != int(wantResult) || (gotErr == nil) != (wantErr == nil) { t.Errorf("PresentBehavior(%d) = (%d, %v), Go = (%d, %v)", item[0], gotResult, gotErr, wantResult, wantErr) }

    var gotAdder generatedAdder = &gotValue
    var wantAdder referenceAdder = &wantValue
    gotAdder.Add(gotFirst)
    wantAdder.Add(wantFirst)
    if int(gotValue) != int(wantValue) { t.Errorf("external pointer interface = %d, Go = %d", gotValue, wantValue) }
  }
  if got, want := generated.NilBehavior(), reference.NilBehavior(); got != want { t.Errorf("NilBehavior = %v, Go = %v", got, want) }
  for _, value := range []string{"", "onsen", "温泉卵"} {
    gotLabel, wantLabel := generated.Label(value), reference.Label(value)
    if got, want := generated.LabelBehavior(gotLabel), reference.LabelBehavior(wantLabel); got != want { t.Errorf("LabelBehavior(%q) = %q, Go = %q", value, got, want) }
    if got, want := generated.LabelSize(gotLabel), reference.LabelSize(wantLabel); got != want { t.Errorf("LabelSize(%q) = %d, Go = %d", value, got, want) }
    var gotText generatedText = gotLabel
    var wantText referenceText = wantLabel
    if got, want := gotText.Text(), wantText.Text(); got != want { t.Errorf("external value interface = %q, Go = %q", got, want) }
  }
  for _, values := range [][]int{{}, {1}, {-2, 4, 9}} {
    gotValues, wantValues := append([]int(nil), values...), append([]int(nil), values...)
    if got, want := generated.ValuesBehavior(gotValues, 7), reference.ValuesBehavior(wantValues, 7); got != want || !reflect.DeepEqual(gotValues, wantValues) { t.Errorf("ValuesBehavior(%v) = (%d, %v), Go = (%d, %v)", values, got, gotValues, want, wantValues) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "definedtypemethod.test", generated, referenceSource, testSource)
}

func TestLinkedDefinedTypeReceiverMethodMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "counter.otm")
	entry := filepath.Join(temporary, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`type Counter = distinct int; public function add(this: *Counter, delta: Counter): void { *this += delta; } public function doubled(this: Counter): Counter { return this + this; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Counter } from "./counter"; function linked(value: int, delta: int): int { let counter = Counter(value); counter.add(Counter(delta)); return int(counter.doubled()); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "definedtypemethodlinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
type Counter int
func (value *Counter) Add(delta Counter) { *value += delta }
func (value Counter) Doubled() Counter { return value + value }
func Linked(value, delta int) int { counter := Counter(value); counter.Add(Counter(delta)); return int(counter.Doubled()) }
`
	testSource := `package definedtypemethodlinked
import (
  "testing"
  reference "definedtypemethod-linked.test/reference"
)
func TestLinked(t *testing.T) {
  for _, item := range [][2]int{{0, 0}, {-4, 9}, {11, -3}} {
    if got, want := linked(item[0], item[1]), reference.Linked(item[0], item[1]); got != want { t.Errorf("linked(%v) = %d, Go = %d", item, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "definedtypemethod-linked.test", generated, referenceSource, testSource)
}

func TestLinkedDefinedTypePrivateMethodIsRejected(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "counter.otm")
	entry := filepath.Join(temporary, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`type Counter = distinct int; function hidden(this: Counter): int { return int(this); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Counter } from "./counter"; function bad(value: Counter): int { return value.hidden(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := EmitGo([]string{entry}, "definedtypemethodprivate")
	if err != nil {
		t.Fatal(err)
	}
	messages := make([]string, len(diagnostics))
	for index, item := range diagnostics {
		messages[index] = item.Message
	}
	if !strings.Contains(strings.Join(messages, "\n"), `method "hidden" is private on defined type`) {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}
