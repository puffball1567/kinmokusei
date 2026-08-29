package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalNativeStructReceiversMatchIndependentGo(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "receiver.otm")
	dependency := filepath.Join(temp, "meter.otm")
	input := `
import { Meter, meter } from "./meter";

struct Counter {
  public value: int;
  public function read(): int { return this.value; }
}

public function added(this: Counter, delta: int): Counter {
  this.value += delta;
  return this;
}

public function add(this: *Counter, delta: int): *Counter {
  this.value += delta;
  return this;
}

function localForms(): [5]int {
  let value = Counter { value: 2 };
  const copy = value.added(3);
  const readCopy = value.read;
  const addShared = value.add;
  value.value = 7;
  addShared(4).add(5);
  return [copy.value, readCopy(), value.value, value.read(), value.added(1).value];
}

function linkedForms(): [3]int {
  let value: Meter = meter(4);
  const before = value.read();
  const add = value.add;
  add(3);
  return [before, value.read(), value.added(5).read()];
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependency, []byte(`
struct Meter { public value: int; }
public function read(this: Meter): int { return this.value; }
public function added(this: Meter, delta: int): Meter { this.value += delta; return this; }
public function add(this: *Meter, delta: int): void { this.value += delta; }
function meter(value: int): Meter { return Meter { value: value }; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "externalreceiver")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, want := range []string{
		"func (this Counter) Added(delta int) Counter",
		"func (this *Counter) Add(delta int) *Counter",
		"func (this Meter) Read() int",
		"var addShared = value.Add",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference

type counter struct { value int }
func (value counter) read() int { return value.value }
func (value counter) added(delta int) counter { value.value += delta; return value }
func (value *counter) add(delta int) *counter { value.value += delta; return value }
func LocalForms() [5]int {
  value := counter{value: 2}
  copy := value.added(3)
  readCopy := value.read
  addShared := value.add
  value.value = 7
  addShared(4).add(5)
  return [5]int{copy.value, readCopy(), value.value, value.read(), value.added(1).value}
}

type meter struct { value int }
func (value meter) read() int { return value.value }
func (value meter) added(delta int) meter { value.value += delta; return value }
func (value *meter) add(delta int) { value.value += delta }
func LinkedForms() [3]int {
  value := meter{value: 4}
  before := value.read()
  add := value.add
  add(3)
  return [3]int{before, value.read(), value.added(5).read()}
}
`
	testSource := `package externalreceiver
import (
  "testing"
  reference "external-native-receiver.test/reference"
)
func TestExternalReceiverRuntimeContract(t *testing.T) {
  if got, want := localForms(), reference.LocalForms(); got != want { t.Errorf("local forms = %v, Go = %v", got, want) }
  if got, want := linkedForms(), reference.LinkedForms(); got != want { t.Errorf("linked forms = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "external-native-receiver.test", generated, referenceSource, testSource)
}

func TestExternalReceiverCannotExtendImportedStruct(t *testing.T) {
	temp := t.TempDir()
	dependency := filepath.Join(temp, "dependency.otm")
	source := filepath.Join(temp, "root.otm")
	if err := os.WriteFile(dependency, []byte(`struct Imported { public value: int; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(`
import { Imported } from "./dependency";
public function read(this: Imported): int { return this.value; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := EmitGo([]string{source}, "invalidreceiver")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnosticsText(diagnostics), "must be declared in the same module") {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}
