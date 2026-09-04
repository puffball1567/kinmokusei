package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectionBuiltinsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "collections.km")
	input := `
import go net from "net";
import go http from "net/http";
import go atomic from "sync/atomic";

function sizes(text: string, values: int[], array: [3]int, pointer: *[3]int, lookup: Map<string, int>, channel: GoChannel<int>): int {
  return len(text) + len(values) + len(array) + len(pointer) + len(lookup) + len(channel) + cap(values) + cap(array) + cap(pointer) + cap(channel);
}
function appendWithinCapacity(): int {
  let values = makeSlice[int](1, 3);
  values[0] = 1;
  const grown = append(values, 2);
  grown[0] = 9;
  return len(grown) * 100 + cap(grown) * 10 + values[0];
}
function appendReallocation(): int {
  let values = makeSlice[int](1, 1);
  values[0] = 1;
  const grown = append(values, 2);
  grown[0] = 9;
  return values[0] * 10 + grown[0];
}
function appendSpread(values: int[], suffix: int[]): int[] { return append(values, suffix...); }
function appendText(values: byte[]): byte[] { return append(values, "abc"...); }
function copyOverlap(): int {
  let values = [1, 2, 3, 4];
  const count = copy(values[1:], values[:3]);
  return count * 10000 + values[0] * 1000 + values[1] * 100 + values[2] * 10 + values[3];
}
function copyText(): byte[] {
  const values = makeSlice[byte](5);
  copy(values, "abc");
  return values;
}
function mapOperations(): int {
  let lookup = makeMap[string, int](2);
  lookup["a"] = 1;
  lookup["b"] = 2;
  delete(lookup, "a");
  delete(lookup, "missing");
  return len(lookup) * 10 + lookup["b"];
}
function deleteNil(lookup: Map<string, int>): void { delete(lookup, "missing"); }
function nilSizes(values: int[], lookup: Map<string, int>): int { return len(values) + cap(values) + len(lookup); }
function extendIP(value: net.IP): net.IP { return append(value, byte(1), byte(2)); }
function namedSizes(value: net.IP, header: http.Header): int { return len(value) + cap(value) + len(header); }
function deleteHeader(header: http.Header): int { delete(header, "X-Test"); return len(header); }
function record(counter: *atomic.Int64, digit: int64, value: int): int {
  counter.Store(counter.Load() * 10 + digit);
  return value;
}
function observed(counter: *atomic.Int64, digit: int64, values: int[]): int[] {
  counter.Store(counter.Load() * 10 + digit);
  return values;
}
function makeEvaluationOrder(): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  const values = makeSlice[int](record(&counter, 1, 1), record(&counter, 2, 2));
  return counter.Load() * 10 + int64(cap(values));
}
function appendEvaluationOrder(): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  const values = append(observed(&counter, 1, [1]), record(&counter, 2, 2));
  return counter.Load() * 10 + int64(values[1]);
}
function dynamicNegative(length: int): int[] { return makeSlice[int](length); }
function dynamicCapacity(length: int, capacity: int): int[] { return makeSlice[int](length, capacity); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "collections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"kinmokuseiMakeLength := 1", "kinmokuseiMakeCapacity := 3", "make([]int, kinmokuseiMakeLength, kinmokuseiMakeCapacity)", "make(map[string]int, 2)", "append(values, suffix...)", `append(values, "abc"...)`, "copy(values[1:], values[:3])", `delete(lookup, "a")`, "len(value)", "cap(value)"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "net"
  "net/http"
  "sync/atomic"
)
func Sizes(text string, values []int, array [3]int, pointer *[3]int, lookup map[string]int, channel chan int) int {
  return len(text) + len(values) + len(array) + len(pointer) + len(lookup) + len(channel) + cap(values) + cap(array) + cap(pointer) + cap(channel)
}
func AppendWithinCapacity() int {
  values := make([]int, 1, 3)
  values[0] = 1
  grown := append(values, 2)
  grown[0] = 9
  return len(grown) * 100 + cap(grown) * 10 + values[0]
}
func AppendReallocation() int {
  values := make([]int, 1, 1)
  values[0] = 1
  grown := append(values, 2)
  grown[0] = 9
  return values[0] * 10 + grown[0]
}
func AppendSpread(values, suffix []int) []int { return append(values, suffix...) }
func AppendText(values []byte) []byte { return append(values, "abc"...) }
func CopyOverlap() int {
  values := []int{1, 2, 3, 4}
  count := copy(values[1:], values[:3])
  return count * 10000 + values[0] * 1000 + values[1] * 100 + values[2] * 10 + values[3]
}
func CopyText() []byte {
  values := make([]byte, 5)
  copy(values, "abc")
  return values
}
func MapOperations() int {
  lookup := make(map[string]int, 2)
  lookup["a"], lookup["b"] = 1, 2
  delete(lookup, "a")
  delete(lookup, "missing")
  return len(lookup) * 10 + lookup["b"]
}
func DeleteNil(lookup map[string]int) { delete(lookup, "missing") }
func NilSizes(values []int, lookup map[string]int) int { return len(values) + cap(values) + len(lookup) }
func ExtendIP(value net.IP) net.IP { return append(value, byte(1), byte(2)) }
func NamedSizes(value net.IP, header http.Header) int { return len(value) + cap(value) + len(header) }
func DeleteHeader(header http.Header) int { delete(header, "X-Test"); return len(header) }
func record(counter *atomic.Int64, digit int64, value int) int {
  counter.Store(counter.Load() * 10 + digit)
  return value
}
func observed(counter *atomic.Int64, digit int64, values []int) []int {
  counter.Store(counter.Load() * 10 + digit)
  return values
}
func MakeEvaluationOrder() int64 {
  var counter atomic.Int64
  length := record(&counter, 1, 1)
  capacity := record(&counter, 2, 2)
  values := make([]int, length, capacity)
  return counter.Load() * 10 + int64(cap(values))
}
func AppendEvaluationOrder() int64 {
  var counter atomic.Int64
  values := append(observed(&counter, 1, []int{1}), record(&counter, 2, 2))
  return counter.Load() * 10 + int64(values[1])
}
func DynamicNegative(length int) []int { return make([]int, length) }
func DynamicCapacity(length, capacity int) []int { return make([]int, length, capacity) }
`
	testSource := `package collections
import (
  "net"
  "net/http"
	"reflect"
	"slices"
  "testing"
	reference "collections.test/reference"
)
func didPanic(operation func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  operation()
  return false
}
func TestCollectionBuiltins(t *testing.T) {
  array := [3]int{1, 2, 3}
  lookup := map[string]int{"x": 1}
  channel := make(chan int, 2)
  goArray := array
  goChannel := make(chan int, 2)
  if got, want := sizes("abc", []int{1, 2}, array, &array, lookup, channel), reference.Sizes("abc", []int{1, 2}, goArray, &goArray, lookup, goChannel); got != want { t.Errorf("sizes = %d, Go = %d", got, want) }
  if got, want := appendWithinCapacity(), reference.AppendWithinCapacity(); got != want { t.Errorf("appendWithinCapacity = %d, Go = %d", got, want) }
  if got, want := appendReallocation(), reference.AppendReallocation(); got != want { t.Errorf("appendReallocation = %d, Go = %d", got, want) }
  if got, want := appendSpread([]int{1}, []int{2, 3}), reference.AppendSpread([]int{1}, []int{2, 3}); !slices.Equal(got, want) { t.Errorf("appendSpread = %v, Go = %v", got, want) }
  if got, want := appendText([]byte{'x'}), reference.AppendText([]byte{'x'}); !slices.Equal(got, want) { t.Errorf("appendText = %q, Go = %q", got, want) }
  if got, want := copyOverlap(), reference.CopyOverlap(); got != want { t.Errorf("copyOverlap = %d, Go = %d", got, want) }
  if got, want := copyText(), reference.CopyText(); !slices.Equal(got, want) { t.Errorf("copyText = %v, Go = %v", got, want) }
  if got, want := mapOperations(), reference.MapOperations(); got != want { t.Errorf("mapOperations = %d, Go = %d", got, want) }
  deleteNil(nil)
  reference.DeleteNil(nil)
  if got, want := nilSizes(nil, nil), reference.NilSizes(nil, nil); got != want { t.Errorf("nilSizes = %d, Go = %d", got, want) }
  gotIP, wantIP := net.IP{127, 0, 0, 1}, net.IP{127, 0, 0, 1}
  if got, want := extendIP(gotIP), reference.ExtendIP(wantIP); !slices.Equal(got, want) { t.Errorf("extendIP = %v, Go = %v", got, want) }
  gotHeader := http.Header{"X-Test": []string{"ok"}, "Other": []string{"value"}}
  wantHeader := http.Header{"X-Test": []string{"ok"}, "Other": []string{"value"}}
  if got, want := namedSizes(gotIP, gotHeader), reference.NamedSizes(wantIP, wantHeader); got != want { t.Errorf("namedSizes = %d, Go = %d", got, want) }
  if got, want := deleteHeader(gotHeader), reference.DeleteHeader(wantHeader); got != want || !reflect.DeepEqual(gotHeader, wantHeader) { t.Errorf("deleteHeader = (%d, %v), Go = (%d, %v)", got, gotHeader, want, wantHeader) }
  if got, want := makeEvaluationOrder(), reference.MakeEvaluationOrder(); got != want { t.Errorf("makeEvaluationOrder = %d, Go = %d", got, want) }
  if got, want := appendEvaluationOrder(), reference.AppendEvaluationOrder(); got != want { t.Errorf("appendEvaluationOrder = %d, Go = %d", got, want) }
  for name, operations := range map[string][2]func(){
    "negative length": {func() { dynamicNegative(-1) }, func() { reference.DynamicNegative(-1) }},
    "capacity below length": {func() { dynamicCapacity(2, 1) }, func() { reference.DynamicCapacity(2, 1) }},
  } {
    if got, want := didPanic(operations[0]), didPanic(operations[1]); got != want { t.Errorf("%s panic = %v, Go = %v", name, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "collections.test", generated, referenceSource, testSource)
}

func TestClearMinMaxBuiltinsMatchIndependentGo(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "ordered_clear.km")
	input := `
import go atomic from "sync/atomic";
import go http from "net/http";
import go math from "math";
import go net from "net";
import go time from "time";

function clearSlice(values: int[]): int {
  clear(values);
  return len(values) * 1000 + cap(values) * 100 + values[0] * 10 + values[1];
}
function clearMap(values: Map<string, int>): int { clear(values); return len(values); }
function clearNullable(values: int[] | null, lookup: Map<string, int> | null): int {
  clear(values);
  clear(lookup);
  return len(values) + len(lookup);
}
function clearNamed(ip: net.IP, header: http.Header): int {
  clear(ip);
  clear(header);
  return len(ip) * 100 + len(header);
}
function lowerInt(left: int, middle: int, right: int): int { return min(left, middle, right); }
function upperFloat(left: float, middle: float, right: float): float { return max(left, middle, right); }
function lowerString(left: string, middle: string, right: string): string { return min(left, middle, right); }
function upperDuration(left: time.Duration, right: time.Duration): time.Duration { return max(left, right, 0); }
function minIsNaN(value: float): boolean { return math.IsNaN(min(value, 1.0)); }
function maxIsNaN(value: float): boolean { return math.IsNaN(max(1.0, value)); }
function minSignbit(left: float, right: float): boolean { return math.Signbit(min(left, right)); }
function maxSignbit(left: float, right: float): boolean { return math.Signbit(max(left, right)); }
function observe(counter: *atomic.Int64, digit: int64, value: int): int {
  counter.Store(counter.Load() * 10 + digit);
  return value;
}
function minEvaluationOrder(): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  const value = min(observe(&counter, 1, 7), observe(&counter, 2, 3), observe(&counter, 3, 5));
  return counter.Load() * 10 + int64(value);
}
function maxEvaluationOrder(): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  const value = max(observe(&counter, 1, 7), observe(&counter, 2, 3), observe(&counter, 3, 5));
  return counter.Load() * 10 + int64(value);
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "orderedclear")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"clear(values)", "min(left, middle, right)", "max(left, right, 0)", "min(observe(&counter, 1, 7), observe(&counter, 2, 3), observe(&counter, 3, 5))"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference
import (
  "math"
  "net"
  "net/http"
  "sync/atomic"
  "time"
)
func ClearSlice(values []int) int {
  clear(values)
  return len(values) * 1000 + cap(values) * 100 + values[0] * 10 + values[1]
}
func ClearMap(values map[string]int) int { clear(values); return len(values) }
func ClearNullable(values []int, lookup map[string]int) int {
  clear(values)
  clear(lookup)
  return len(values) + len(lookup)
}
func ClearNamed(ip net.IP, header http.Header) int {
  clear(ip)
  clear(header)
  return len(ip) * 100 + len(header)
}
func LowerInt(left, middle, right int) int { return min(left, middle, right) }
func UpperFloat(left, middle, right float64) float64 { return max(left, middle, right) }
func LowerString(left, middle, right string) string { return min(left, middle, right) }
func UpperDuration(left, right time.Duration) time.Duration { return max(left, right, 0) }
func MinIsNaN(value float64) bool { return math.IsNaN(min(value, 1.0)) }
func MaxIsNaN(value float64) bool { return math.IsNaN(max(1.0, value)) }
func MinSignbit(left, right float64) bool { return math.Signbit(min(left, right)) }
func MaxSignbit(left, right float64) bool { return math.Signbit(max(left, right)) }
func observe(counter *atomic.Int64, digit int64, value int) int {
  counter.Store(counter.Load() * 10 + digit)
  return value
}
func MinEvaluationOrder() int64 {
  var counter atomic.Int64
  value := min(observe(&counter, 1, 7), observe(&counter, 2, 3), observe(&counter, 3, 5))
  return counter.Load() * 10 + int64(value)
}
func MaxEvaluationOrder() int64 {
  var counter atomic.Int64
  value := max(observe(&counter, 1, 7), observe(&counter, 2, 3), observe(&counter, 3, 5))
  return counter.Load() * 10 + int64(value)
}
`
	testSource := `package orderedclear
import (
  "math"
  "net"
  "net/http"
  "testing"
  "time"
  reference "ordered-clear.test/reference"
)
func TestOrderedAndClearDifferential(t *testing.T) {
  for _, input := range [][]int{{1, 2}, {0, -1}} {
    gotInput, wantInput := append([]int(nil), input...), append([]int(nil), input...)
    if got, want := clearSlice(gotInput), reference.ClearSlice(wantInput); got != want { t.Errorf("clearSlice(%v) = %d, Go = %d", input, got, want) }
  }
  gotMap, wantMap := map[string]int{"a": 1, "b": 2}, map[string]int{"a": 1, "b": 2}
  if got, want := clearMap(gotMap), reference.ClearMap(wantMap); got != want { t.Errorf("clearMap = %d, Go = %d", got, want) }
  if got, want := clearNullable(nil, nil), reference.ClearNullable(nil, nil); got != want { t.Errorf("clearNullable = %d, Go = %d", got, want) }
  gotIP, wantIP := net.IP{127, 0, 0, 1}, net.IP{127, 0, 0, 1}
  gotHeader, wantHeader := http.Header{"X": {"one"}}, http.Header{"X": {"one"}}
  if got, want := clearNamed(gotIP, gotHeader), reference.ClearNamed(wantIP, wantHeader); got != want { t.Errorf("clearNamed = %d, Go = %d", got, want) }
  for _, values := range [][3]int{{3, 1, 2}, {-1, 0, 1}, {7, 7, 7}} {
    if got, want := lowerInt(values[0], values[1], values[2]), reference.LowerInt(values[0], values[1], values[2]); got != want { t.Errorf("lowerInt(%v) = %d, Go = %d", values, got, want) }
  }
  for _, values := range [][3]float64{{3.5, 1.5, 2.5}, {-1, 0, 1}} {
    if got, want := upperFloat(values[0], values[1], values[2]), reference.UpperFloat(values[0], values[1], values[2]); got != want { t.Errorf("upperFloat(%v) = %v, Go = %v", values, got, want) }
  }
  for _, values := range [][3]string{{"c", "a", "b"}, {"温", "卵", "泉"}} {
    if got, want := lowerString(values[0], values[1], values[2]), reference.LowerString(values[0], values[1], values[2]); got != want { t.Errorf("lowerString(%v) = %q, Go = %q", values, got, want) }
  }
  if got, want := upperDuration(-time.Second, time.Second), reference.UpperDuration(-time.Second, time.Second); got != want { t.Errorf("upperDuration = %v, Go = %v", got, want) }
  nan := math.NaN()
  if got, want := minIsNaN(nan), reference.MinIsNaN(nan); got != want { t.Errorf("minIsNaN = %v, Go = %v", got, want) }
  if got, want := maxIsNaN(nan), reference.MaxIsNaN(nan); got != want { t.Errorf("maxIsNaN = %v, Go = %v", got, want) }
  negativeZero := math.Copysign(0, -1)
  for _, pair := range [][2]float64{{0, negativeZero}, {negativeZero, 0}} {
    if got, want := minSignbit(pair[0], pair[1]), reference.MinSignbit(pair[0], pair[1]); got != want { t.Errorf("minSignbit(%v) = %v, Go = %v", pair, got, want) }
    if got, want := maxSignbit(pair[0], pair[1]), reference.MaxSignbit(pair[0], pair[1]); got != want { t.Errorf("maxSignbit(%v) = %v, Go = %v", pair, got, want) }
  }
  if got, want := minEvaluationOrder(), reference.MinEvaluationOrder(); got != want { t.Errorf("minEvaluationOrder = %d, Go = %d", got, want) }
  if got, want := maxEvaluationOrder(), reference.MaxEvaluationOrder(); got != want { t.Errorf("maxEvaluationOrder = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "ordered-clear.test", generated, referenceSource, testSource)
}

func TestCollectionBuiltinNamesCanBeShadowed(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "shadow.km")
	input := `
function len(value: int): int { return value + 1; }
function cap(value: int): int { return value + 2; }
function append(value: int): int { return value + 3; }
function copy(value: int): int { return value + 4; }
function delete(value: int): int { return value + 5; }
function clear(value: int): int { return value + 6; }
function min(value: int): int { return value + 7; }
function max(value: int): int { return value + 8; }
function makeSlice(value: int): int { return value + 9; }
function makeMap(value: int): int { return value + 10; }
function copyArray(value: int): int { return value + 11; }
function viewArray(value: int): int { return value + 12; }
function use(): int { return len(1) + cap(1) + append(1) + copy(1) + delete(1) + clear(1) + min(1) + max(1) + makeSlice(1) + makeMap(1) + copyArray(1) + viewArray(1); }
function localShadow(): int { const len = (value: int) => value + 8; return len(1); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "shadow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"func len_(value int) int", "func cap_(value int) int", "func append_(value int) int", "func copy_(value int) int", "func delete_(value int) int", "func clear_(value int) int", "func min_(value int) int", "func max_(value int) int", "func makeSlice(value int) int", "func makeMap(value int) int", "func copyArray(value int) int", "func viewArray(value int) int", "var len_ = func"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("shadowed names were not stably mangled; missing %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
func Len(value int) int { return value+1 }
func Cap(value int) int { return value+2 }
func Append(value int) int { return value+3 }
func Copy(value int) int { return value+4 }
func Delete(value int) int { return value+5 }
func Clear(value int) int { return value+6 }
func Min(value int) int { return value+7 }
func Max(value int) int { return value+8 }
func MakeSlice(value int) int { return value+9 }
func MakeMap(value int) int { return value+10 }
func CopyArray(value int) int { return value+11 }
func ViewArray(value int) int { return value+12 }
func Use() int { return Len(1)+Cap(1)+Append(1)+Copy(1)+Delete(1)+Clear(1)+Min(1)+Max(1)+MakeSlice(1)+MakeMap(1)+CopyArray(1)+ViewArray(1) }
func LocalShadow() int { length := func(value int) int { return value+8 }; return length(1) }
`
	testSource := `package shadow
import (
  "testing"
  reference "shadow.test/reference"
)
func TestShadowed(t *testing.T) {
  if got, want := use(), reference.Use(); got != want { t.Errorf("use = %d, Go = %d", got, want) }
  if got, want := localShadow(), reference.LocalShadow(); got != want { t.Errorf("localShadow = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "shadow.test", generated, referenceSource, testSource)
}
