package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompoundAssignmentAndIncrementCompileAndRunMatrix(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "updates.otm")
	input := `
import go os from "os";
import go time from "time";

class Counter {
  public count: int;
  constructor() { this.count = 0; }
  public function bump(): int { this.count++; this.count += 2; return this.count; }
}

function integerOperators(): int {
  let value = 240;
  value += 16;
  value -= 6;
  value *= 2;
  value /= 5;
  value %= 64;
  value |= 8;
  value ^= 3;
  value &= 31;
  value &^= 3;
  value <<= 2;
  value >>= 3;
  value++;
  value--;
  return value;
}
function concat(): string { let value = "on"; value += "tama"; return value; }
function numericUpdates(): int { let decimal = 1.5; let small: byte = 1; decimal++; decimal--; small++; small--; return int(decimal * 10.0) + int(small); }
function indexed(): int {
  const values = [1];
  const table = makeMap[string, int]();
  values[0] += 2;
  values[0]++;
  table["x"] = 1;
  table["x"] += 2;
  table["x"]++;
  return values[0] * 10 + table["x"];
}
function pointerUpdate(): int { let value = 3; const pointer = &value; (*pointer) *= 4; (*pointer)--; return value; }
function nextIndex(calls: *int): int { (*calls)++; return 0; }
function targetEvaluatedOnce(): int {
  let calls = 0;
  const values = [10];
  values[nextIndex(&calls)] += 5;
  return calls * 100 + values[0];
}
function forPost(limit: int): int { let index = 0; for (; index < limit; index++) {} return index; }
function classUpdate(): int { return new Counter().bump(); }
function namedGoTypes(): int {
  let duration: time.Duration = time.Second;
  duration += time.Second;
  duration >>= 1;
  let mode: os.FileMode = os.ModePerm;
  mode &^= os.ModePerm;
  mode |= os.ModeDir;
  return int(duration / time.Second) + int((mode & os.ModeDir) >> 31);
}
function dynamicDivide(value: int, divisor: int): int { value /= divisor; return value; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "updatematrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{"value += 16", "value &^= 3", "value <<= 2", "value++", "value--", `value += "tama"`, `table["x"]++`, "*pointer *= 4", "values[nextIndex(&calls)] += 5", "for ; index < limit; index++", "duration += time.Second", "mode |= os.ModeDir"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "os"
  "time"
)
type Counter struct { count int }
func NewCounter() *Counter { return &Counter{} }
func (counter *Counter) Bump() int { counter.count++; counter.count += 2; return counter.count }
func IntegerOperators() int {
  value := 240
  value += 16
  value -= 6
  value *= 2
  value /= 5
  value %= 64
  value |= 8
  value ^= 3
  value &= 31
  value &^= 3
  value <<= 2
  value >>= 3
  value++
  value--
  return value
}
func Concat() string { value := "on"; value += "tama"; return value }
func NumericUpdates() int { decimal := 1.5; var small byte = 1; decimal++; decimal--; small++; small--; return int(decimal * 10.0) + int(small) }
func Indexed() int {
  values := []int{1}
  table := make(map[string]int)
  values[0] += 2
  values[0]++
  table["x"] = 1
  table["x"] += 2
  table["x"]++
  return values[0]*10 + table["x"]
}
func PointerUpdate() int { value := 3; pointer := &value; (*pointer) *= 4; (*pointer)--; return value }
func nextIndex(calls *int) int { (*calls)++; return 0 }
func TargetEvaluatedOnce() int { calls := 0; values := []int{10}; values[nextIndex(&calls)] += 5; return calls*100 + values[0] }
func ForPost(limit int) int { index := 0; for ; index < limit; index++ {} ; return index }
func ClassUpdate() int { return NewCounter().Bump() }
func NamedGoTypes() int {
  duration := time.Duration(time.Second)
  duration += time.Second
  duration >>= 1
  mode := os.FileMode(os.ModePerm)
  mode &^= os.ModePerm
  mode |= os.ModeDir
  return int(duration/time.Second) + int((mode&os.ModeDir)>>31)
}
func DynamicDivide(value, divisor int) int { value /= divisor; return value }
`
	testSource := `package updatematrix
import (
  "testing"
  reference "updates.test/reference"
)
func didPanic(call func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  call()
  return false
}
func TestUpdateMatrix(t *testing.T) {
  if got, want := integerOperators(), reference.IntegerOperators(); got != want { t.Errorf("integerOperators = %d, Go = %d", got, want) }
  if got, want := concat(), reference.Concat(); got != want { t.Errorf("concat = %q, Go = %q", got, want) }
  if got, want := numericUpdates(), reference.NumericUpdates(); got != want { t.Errorf("numericUpdates = %d, Go = %d", got, want) }
  if got, want := indexed(), reference.Indexed(); got != want { t.Errorf("indexed = %d, Go = %d", got, want) }
  if got, want := pointerUpdate(), reference.PointerUpdate(); got != want { t.Errorf("pointerUpdate = %d, Go = %d", got, want) }
  if got, want := targetEvaluatedOnce(), reference.TargetEvaluatedOnce(); got != want { t.Errorf("targetEvaluatedOnce = %d, Go = %d", got, want) }
  for _, limit := range []int{-3, 0, 1, 4, 9} {
    if got, want := forPost(limit), reference.ForPost(limit); got != want { t.Errorf("forPost(%d) = %d, Go = %d", limit, got, want) }
  }
  if got, want := classUpdate(), reference.ClassUpdate(); got != want { t.Errorf("classUpdate = %d, Go = %d", got, want) }
  if got, want := namedGoTypes(), reference.NamedGoTypes(); got != want { t.Errorf("namedGoTypes = %d, Go = %d", got, want) }
  for _, item := range [][2]int{{12, 3}, {-12, 3}, {13, -3}, {0, 7}, {1, 2}} {
    if got, want := dynamicDivide(item[0], item[1]), reference.DynamicDivide(item[0], item[1]); got != want { t.Errorf("dynamicDivide(%d, %d) = %d, Go = %d", item[0], item[1], got, want) }
  }
}
func TestDynamicDividePanicCompatibility(t *testing.T) {
  got := didPanic(func() { dynamicDivide(12, 0) })
  want := didPanic(func() { reference.DynamicDivide(12, 0) })
  if got != want || !got { t.Errorf("zero divisor panic = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "updates.test", generated, referenceSource, testSource)
}
