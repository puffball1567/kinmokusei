package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValueSwitchCompilesAndRuns(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "value_switch.otm")
	input := `
import go atomic from "sync/atomic";

class Item {
  constructor(public value: int) {}
}

function classify(value: int): int {
  switch (value) {
    case 0 { return 10; }
    case 1, 2 + 1 { return 20; }
    default { return 30; }
  }
}
function observe(counter: *atomic.Int64, value: int): int {
  counter.Add(1);
  return value;
}
function evaluationOrder(value: int): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  let selected = 0;
  switch (observe(&counter, value)) {
    case observe(&counter, 1) { selected = 1; }
    case observe(&counter, 2), observe(&counter, 3) { selected = 2; }
    default { selected = 9; }
  }
  return counter.Load() * 10 + int64(selected);
}
function nilPointer(value: *int): boolean {
  switch (value) {
    case nil { return true; }
    default { return false; }
  }
}
function fixedArray(value: [2]int): int {
  switch (value) {
    case [1, 2] { return 1; }
    case [3, 4], [5, 6] { return 2; }
    default { return 0; }
  }
}
function sameReference(actual: Item, expected: Item): boolean {
  switch (actual) {
    case expected { return true; }
    default { return false; }
  }
}
function breakSwitch(value: int): int {
  let result = 0;
  switch (value) {
    case 1 { result = 7; break; }
    default { result = 8; }
  }
  return result;
}
function fallthroughChain(value: int): int {
  let result = 0;
  switch (value) {
    case 0 { result = result * 10 + 1; fallthrough; }
    case 1 { result = result * 10 + 2; fallthrough; }
    default { result = result * 10 + 3; }
  }
  return result;
}
function fallthroughDefaultMiddle(value: int): int {
  let result = 0;
  switch (value) {
    case 0 { result = result * 10 + 1; fallthrough; }
    default { result = result * 10 + 2; fallthrough; }
    case 2 { result = result * 10 + 3; }
  }
  return result;
}
function fallthroughReturns(value: int): int {
  switch (value) {
    case 0 { fallthrough; }
    case 1 { return 10; }
    default { return 20; }
  }
}
function fallthroughEvaluation(value: int): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  let result = 0;
  switch (observe(&counter, value)) {
    case observe(&counter, 0) { result = 1; fallthrough; }
    case observe(&counter, 1) { result = result * 10 + 2; }
    default { result = 9; }
  }
  return counter.Load() * 100 + int64(result);
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "valueswitch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{
		"switch value {",
		"case 1, 2 + 1:",
		"switch observe(&counter, value) {",
		"case observe(&counter, 2), observe(&counter, 3):",
		"case [2]int{1, 2}:",
		"switch actual {",
		"fallthrough",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import "sync/atomic"
func Classify(value int) int {
  switch value {
  case 0:
    return 10
  case 1, 2 + 1:
    return 20
  default:
    return 30
  }
}
func observe(counter *atomic.Int64, value int) int {
  counter.Add(1)
  return value
}
func EvaluationOrder(value int) int64 {
  var counter atomic.Int64
  selected := 0
  switch observe(&counter, value) {
  case observe(&counter, 1):
    selected = 1
  case observe(&counter, 2), observe(&counter, 3):
    selected = 2
  default:
    selected = 9
  }
  return counter.Load() * 10 + int64(selected)
}
func NilPointer(value *int) bool {
  switch value {
  case nil:
    return true
  default:
    return false
  }
}
func FixedArray(value [2]int) int {
  switch value {
  case [2]int{1, 2}:
    return 1
  case [2]int{3, 4}, [2]int{5, 6}:
    return 2
  default:
    return 0
  }
}
type item struct { value int }
func ReferenceIdentity() (same, distinct bool) {
  first, second := &item{value: 1}, &item{value: 1}
  switch first {
  case first:
    same = true
  }
  switch first {
  case second:
    distinct = true
  }
  return same, distinct
}
func BreakSwitch(value int) int {
  result := 0
  switch value {
  case 1:
    result = 7
    break
  default:
    result = 8
  }
  return result
}
func FallthroughChain(value int) int {
  result := 0
  switch value {
  case 0:
    result = result*10 + 1
    fallthrough
  case 1:
    result = result*10 + 2
    fallthrough
  default:
    result = result*10 + 3
  }
  return result
}
func FallthroughDefaultMiddle(value int) int {
  result := 0
  switch value {
  case 0:
    result = result*10 + 1
    fallthrough
  default:
    result = result*10 + 2
    fallthrough
  case 2:
    result = result*10 + 3
  }
  return result
}
func FallthroughReturns(value int) int {
  switch value {
  case 0:
    fallthrough
  case 1:
    return 10
  default:
    return 20
  }
}
func FallthroughEvaluation(value int) int64 {
  var counter atomic.Int64
  result := 0
  switch observe(&counter, value) {
  case observe(&counter, 0):
    result = 1
    fallthrough
  case observe(&counter, 1):
    result = result*10 + 2
  default:
    result = 9
  }
  return counter.Load()*100 + int64(result)
}
`
	testSource := `package valueswitch
import (
  "testing"
  reference "valueswitch.test/reference"
)
func TestValueSwitchRuntimeMatrix(t *testing.T) {
  for _, input := range []int{0, 1, 2, 3, -1} {
    if got, want := classify(input), reference.Classify(input); got != want { t.Errorf("classify(%d) = %d, Go = %d", input, got, want) }
  }
  for _, input := range []int{1, 2, 3, 4, -1} {
    if got, want := evaluationOrder(input), reference.EvaluationOrder(input); got != want { t.Errorf("evaluationOrder(%d) = %d, Go = %d", input, got, want) }
  }
  if got, want := nilPointer(nil), reference.NilPointer(nil); got != want { t.Errorf("nilPointer(nil) = %v, Go = %v", got, want) }
  value := 1
  if got, want := nilPointer(&value), reference.NilPointer(&value); got != want { t.Errorf("nilPointer(non-nil) = %v, Go = %v", got, want) }
  for _, input := range [][2]int{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {0, 0}} {
    if got, want := fixedArray(input), reference.FixedArray(input); got != want { t.Errorf("fixedArray(%v) = %d, Go = %d", input, got, want) }
  }
  first, second := NewItem(1), NewItem(1)
  gotSame, gotDistinct := sameReference(first, first), sameReference(first, second)
  wantSame, wantDistinct := reference.ReferenceIdentity()
  if gotSame != wantSame || gotDistinct != wantDistinct { t.Errorf("reference identity = (%v, %v), Go = (%v, %v)", gotSame, gotDistinct, wantSame, wantDistinct) }
  for _, input := range []int{1, 2, 0, -1} {
    if got, want := breakSwitch(input), reference.BreakSwitch(input); got != want { t.Errorf("breakSwitch(%d) = %d, Go = %d", input, got, want) }
  }
  for _, input := range []int{-1, 0, 1, 2, 9} {
    if got, want := fallthroughChain(input), reference.FallthroughChain(input); got != want { t.Errorf("fallthroughChain(%d) = %d, Go = %d", input, got, want) }
    if got, want := fallthroughDefaultMiddle(input), reference.FallthroughDefaultMiddle(input); got != want { t.Errorf("fallthroughDefaultMiddle(%d) = %d, Go = %d", input, got, want) }
    if got, want := fallthroughReturns(input), reference.FallthroughReturns(input); got != want { t.Errorf("fallthroughReturns(%d) = %d, Go = %d", input, got, want) }
    if got, want := fallthroughEvaluation(input), reference.FallthroughEvaluation(input); got != want { t.Errorf("fallthroughEvaluation(%d) = %d, Go = %d", input, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "valueswitch.test", generated, referenceSource, testSource)
}
