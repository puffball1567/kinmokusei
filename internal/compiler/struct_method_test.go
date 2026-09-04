package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeStructReceiverMethodsMatchIndependentGo(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "methods.km")
	dependency := filepath.Join(temp, "meter.km")
	input := `
import { Meter, meter } from "./meter";

struct Counter {
  public value: int;
  public function addToCopy(delta: int): int {
    this.value += delta;
    return this.value;
  }
  public function read(): int { return this.value; }
  public pointer function add(delta: int): void { this.value += delta; }
  public pointer function nilOrValue(): int {
    if (this === nil) { return -1; }
    return this.value;
  }
  private function doubled(): int { return this.value * 2; }
}

function valueReceiverCopy(): [2]int {
  let value = Counter { value: 3 };
  const changed = value.addToCopy(4);
  return [changed, value.value];
}
function pointerReceiverForms(): [4]int {
  let value = Counter { value: 1 };
  value.add(2);
  const pointer = &value;
  pointer.add(3);
  pointer.value += 4;
  return [value.value, pointer.read(), value.doubled(), pointer.nilOrValue()];
}
function methodValues(): [4]int {
  let value = Counter { value: 5 };
  const copiedRead = value.read;
  const add = value.add;
  value.value = 8;
  add(2);
  return [copiedRead(), value.value, value.read(), value.doubled()];
}
function receiverStep(order: *int): int {
  (*order) = (*order) * 10 + 1;
  return 0;
}
function argumentStep(order: *int): int {
  (*order) = (*order) * 10 + 2;
  return 3;
}
function receiverAndArgumentOrder(): int {
  let order = 0;
  let values = [Counter { value: 4 }];
  values[receiverStep(&order)].add(argumentStep(&order));
  return order * 10 + values[0].value;
}
function nilPointerReceiver(): int {
  let value: *Counter = nil;
  return value.nilOrValue();
}
function linkedReceiverMethods(): [2]int {
  let value: Meter = meter(6);
  const before = value.read();
  value.add(5);
  return [before, value.read()];
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependency, []byte(`
struct Meter {
  public value: int;
  public function read(): int { return this.value; }
  public pointer function add(delta: int): void { this.value += delta; }
}
function meter(value: int): Meter { return Meter { value: value }; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "structmethods")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, want := range []string{
		"func (this Counter) AddToCopy(delta int) int",
		"func (this *Counter) Add(delta int)",
		"func (this Counter) doubled() int",
		"pointer.Add(3)",
		"pointer.Value += 4",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference

type counter struct { value int }
func (value counter) addToCopy(delta int) int { value.value += delta; return value.value }
func (value counter) read() int { return value.value }
func (value *counter) add(delta int) { value.value += delta }
func (value *counter) nilOrValue() int { if value == nil { return -1 }; return value.value }
func (value counter) doubled() int { return value.value * 2 }

func ValueReceiverCopy() [2]int {
  value := counter{value: 3}
  changed := value.addToCopy(4)
  return [2]int{changed, value.value}
}
func PointerReceiverForms() [4]int {
  value := counter{value: 1}
  value.add(2)
  pointer := &value
  pointer.add(3)
  pointer.value += 4
  return [4]int{value.value, pointer.read(), value.doubled(), pointer.nilOrValue()}
}
func MethodValues() [4]int {
  value := counter{value: 5}
  copiedRead := value.read
  add := value.add
  value.value = 8
  add(2)
  return [4]int{copiedRead(), value.value, value.read(), value.doubled()}
}
func receiverStep(order *int) int { *order = *order * 10 + 1; return 0 }
func argumentStep(order *int) int { *order = *order * 10 + 2; return 3 }
func ReceiverAndArgumentOrder() int {
  order := 0
  values := []counter{{value: 4}}
  values[receiverStep(&order)].add(argumentStep(&order))
  return order * 10 + values[0].value
}
func NilPointerReceiver() int { var value *counter; return value.nilOrValue() }

type meter struct { value int }
func (value meter) read() int { return value.value }
func (value *meter) add(delta int) { value.value += delta }
func LinkedReceiverMethods() [2]int {
  value := meter{value: 6}
  before := value.read()
  value.add(5)
  return [2]int{before, value.read()}
}
`
	testSource := `package structmethods
import (
  "testing"
  reference "native-struct-method.test/reference"
)
func TestNativeStructReceiverRuntimeContract(t *testing.T) {
  if got, want := valueReceiverCopy(), reference.ValueReceiverCopy(); got != want { t.Errorf("value receiver = %v, Go = %v", got, want) }
  if got, want := pointerReceiverForms(), reference.PointerReceiverForms(); got != want { t.Errorf("pointer forms = %v, Go = %v", got, want) }
  if got, want := methodValues(), reference.MethodValues(); got != want { t.Errorf("method values = %v, Go = %v", got, want) }
  if got, want := receiverAndArgumentOrder(), reference.ReceiverAndArgumentOrder(); got != want { t.Errorf("evaluation order = %d, Go = %d", got, want) }
  if got, want := nilPointerReceiver(), reference.NilPointerReceiver(); got != want { t.Errorf("nil receiver = %d, Go = %d", got, want) }
  if got, want := linkedReceiverMethods(), reference.LinkedReceiverMethods(); got != want { t.Errorf("linked methods = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "native-struct-method.test", generated, referenceSource, testSource)
}
