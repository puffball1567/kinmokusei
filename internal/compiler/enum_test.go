package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnumMatchesIndependentGo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "enum.km")
	source := `
enum Status { Pending, Running = 4, Complete, Failed = -2, Retrying, }
enum Tiny: int8 { Minimum = -128, Zero = 0, Maximum = 127, }
enum WireCode: uint16 { Empty, Ready = 41, Complete, Maximum = 65535, }

function statusValue(value: Status): int { return int(value); }
function tinyValue(value: Tiny): int8 { return int8(value); }
function wireValue(value: WireCode): uint16 { return uint16(value); }
function fromInt(value: int): Status { return Status(value); }
function classify(value: Status): string {
  switch (value) {
    case Status.Pending { return "pending"; }
    case Status.Running, Status.Retrying { return "active"; }
    case Status.Complete { return "complete"; }
    default { return "failed"; }
  }
}
function lookup(values: Map<Status, string>, key: Status): string {
  const [value, present] = values[key];
  if (present) { return value; }
  return "missing";
}
function genericIdentity<T>(value: T): T { return value; }
function generic(value: Status): Status { return genericIdentity<Status>(value); }
function ordered(left: WireCode, right: WireCode): boolean { return left < right; }
public function active(this: Status): boolean { return this === Status.Running || this === Status.Retrying; }
public function advance(this: *Status): Status {
  if (*this === Status.Pending) { *this = Status.Running; }
  else if (*this === Status.Running) { *this = Status.Complete; }
  return *this;
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "enummatrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"type Status int", "StatusPending", "StatusRunning", "StatusComplete", "StatusFailed", "StatusRetrying", "type Tiny int8", "type WireCode uint16", "genericIdentity[Status](value)"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go missing %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
type Status int
const (
  StatusPending Status = 0
  StatusRunning Status = 4
  StatusComplete Status = 5
  StatusFailed Status = -2
  StatusRetrying Status = -1
)
type Tiny int8
const ( TinyMinimum Tiny = -128; TinyZero Tiny = 0; TinyMaximum Tiny = 127 )
type WireCode uint16
const ( WireCodeEmpty WireCode = 0; WireCodeReady WireCode = 41; WireCodeComplete WireCode = 42; WireCodeMaximum WireCode = 65535 )
func StatusValue(value Status) int { return int(value) }
func TinyValue(value Tiny) int8 { return int8(value) }
func WireValue(value WireCode) uint16 { return uint16(value) }
func FromInt(value int) Status { return Status(value) }
func Classify(value Status) string { switch value { case StatusPending: return "pending"; case StatusRunning, StatusRetrying: return "active"; case StatusComplete: return "complete"; default: return "failed" } }
func Lookup(values map[Status]string, key Status) string { value, present := values[key]; if present { return value }; return "missing" }
func genericIdentity[T any](value T) T { return value }
func Generic(value Status) Status { return genericIdentity[Status](value) }
func Ordered(left, right WireCode) bool { return left < right }
func (value Status) Active() bool { return value == StatusRunning || value == StatusRetrying }
func (value *Status) Advance() Status { if *value == StatusPending { *value = StatusRunning } else if *value == StatusRunning { *value = StatusComplete }; return *value }
`
	testSource := `package enummatrix
import (
  "testing"
  reference "enum.test/reference"
)
func TestEnumMatrix(t *testing.T) {
  statuses := []struct { got Status; want reference.Status }{
    {StatusPending, reference.StatusPending}, {StatusRunning, reference.StatusRunning}, {StatusComplete, reference.StatusComplete}, {StatusFailed, reference.StatusFailed}, {StatusRetrying, reference.StatusRetrying},
  }
  for _, item := range statuses {
    if got, want := statusValue(item.got), reference.StatusValue(item.want); got != want { t.Errorf("statusValue(%d)=%d Go=%d", item.got, got, want) }
    if got, want := classify(item.got), reference.Classify(item.want); got != want { t.Errorf("classify(%d)=%q Go=%q", item.got, got, want) }
    if got, want := generic(item.got), Status(reference.Generic(item.want)); got != want { t.Errorf("generic(%d)=%d Go=%d", item.got, got, want) }
    if got, want := item.got.Active(), item.want.Active(); got != want { t.Errorf("Active(%d)=%v Go=%v", item.got, got, want) }
  }
  for _, item := range []struct { got Tiny; want reference.Tiny }{{TinyMinimum, reference.TinyMinimum}, {TinyZero, reference.TinyZero}, {TinyMaximum, reference.TinyMaximum}} {
    if got, want := tinyValue(item.got), reference.TinyValue(item.want); got != want { t.Errorf("tinyValue=%d Go=%d", got, want) }
  }
  wires := []struct { got WireCode; want reference.WireCode }{{WireCodeEmpty, reference.WireCodeEmpty}, {WireCodeReady, reference.WireCodeReady}, {WireCodeComplete, reference.WireCodeComplete}, {WireCodeMaximum, reference.WireCodeMaximum}}
  for _, item := range wires {
    if got, want := wireValue(item.got), reference.WireValue(item.want); got != want { t.Errorf("wireValue=%d Go=%d", got, want) }
  }
  for left := range wires { for right := range wires {
    if got, want := ordered(wires[left].got, wires[right].got), reference.Ordered(wires[left].want, wires[right].want); got != want { t.Errorf("ordered(%d,%d)=%v Go=%v", left, right, got, want) }
  }}
  for _, value := range []int{-100, -2, -1, 0, 4, 5, 99} {
    if got, want := fromInt(value), Status(reference.FromInt(value)); got != want { t.Errorf("fromInt(%d)=%d Go=%d", value, got, want) }
  }
  gotAdvanced, wantAdvanced := StatusPending, reference.StatusPending
  for step := 0; step < 4; step++ {
    if got, want := gotAdvanced.Advance(), Status(wantAdvanced.Advance()); got != want || gotAdvanced != Status(wantAdvanced) { t.Errorf("Advance step %d result/state=%d/%d Go=%d", step, got, gotAdvanced, want) }
  }
  gotMap := map[Status]string{StatusPending: "zero", StatusRunning: "four", StatusFailed: "negative"}
  wantMap := map[reference.Status]string{reference.StatusPending: "zero", reference.StatusRunning: "four", reference.StatusFailed: "negative"}
  for _, item := range statuses {
    if got, want := lookup(gotMap, item.got), reference.Lookup(wantMap, item.want); got != want { t.Errorf("lookup(%d)=%q Go=%q", item.got, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "enum.test", generated, referenceSource, testSource)
}

func TestEnumRelativeModuleAndPublicGoAPI(t *testing.T) {
	root := t.TempDir()
	dependency := filepath.Join(root, "status.km")
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(dependency, []byte(`enum Status: int32 { Pending = 10, Complete, } function local(): Status { return Status.Pending; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { Status } from "./status.km"; function selected(): Status { return Status.Complete; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "enumapi")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{"type Status int32", "StatusPending", "StatusComplete", "func selected() Status"} {
		if !strings.Contains(text, want) {
			t.Errorf("linked generated Go missing %q:\n%s", want, generated)
		}
	}
}
