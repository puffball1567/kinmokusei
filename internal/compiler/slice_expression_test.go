package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSliceExpressionsAndNamedCollectionsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "slices.km")
	dependency := filepath.Join(temp, "bounds.km")
	fixture := filepath.Join(temp, "fixture")
	input := `
import go net from "net";
import go http from "net/http";
import go template from "html/template";
import go atomic from "sync/atomic";
import go collections from "example.com/collections";
import { upper } from "./bounds";

function aliasesSliceStorage(): int {
  let values = [1, 2, 3, 4];
  const view = values[1:3];
  view[0] = 9;
  return values[1];
}
function aliasesFixedArrayStorage(): int {
  let values: [4]int = [1, 2, 3, 4];
  const view: int[] = values[1:3];
  view[0] = 8;
  return values[1];
}
function pointerView(values: *[4]int): int[] { return values[:2]; }
function allForms(values: int[]): int {
  const all = values[:];
  const prefix = values[:2];
  const suffix = values[3:];
  const middle = values[1:4];
  const full = values[1:3:4];
  return all[0] + prefix[1] + suffix[0] + middle[0] + full[1];
}
function stringBytes(value: string): string { return value[1:4]; }
function ipTail(value: net.IP): byte[] { return value[12:]; }
function headerFirst(value: http.Header): string { return value["X-Test"][0]; }
function htmlMiddle(value: template.HTML): template.HTML { return value[1:4]; }
function namedArrayAlias(): int {
  let value: collections.Triple = collections.NewTriple();
  const view = value[1:];
  view[0] = 9;
  return value[1];
}
function namedArrayPointer(): int {
  const pointer = collections.NewTriplePointer();
  const view = pointer[:2];
  view[0] = 7;
  return pointer[0];
}
function importedBound(values: int[]): int[] { return values[:upper()]; }
function record(counter: *atomic.Int64, digit: int64, value: int): int {
  counter.Store(counter.Load() * 10 + digit);
  return value;
}
function observed(counter: *atomic.Int64, values: int[]): int[] {
  counter.Store(counter.Load() * 10 + 1);
  return values;
}
function evaluationOrder(): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  const values = [10, 20, 30, 40, 50];
  const view = observed(&counter, values)[record(&counter, 2, 1):record(&counter, 3, 3):record(&counter, 4, 4)];
  return counter.Load() * 100 + int64(view[0]);
}
function dynamicPanic(values: int[], high: int): int[] { return values[:high]; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependency, []byte(`function upper(): int { return 2; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "go.mod"), []byte("module example.com/collections\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureSource := `package collections
type Triple [3]int
func NewTriple() Triple { return Triple{1, 2, 3} }
func NewTriplePointer() *Triple { value := NewTriple(); return &value }
`
	if err := os.WriteFile(filepath.Join(fixture, "collections.go"), []byte(fixtureSource), 0o644); err != nil {
		t.Fatal(err)
	}
	module := "module slicing.test\n\ngo 1.23\n\nrequire example.com/collections v0.0.0\n\nreplace example.com/collections => ./fixture\n"
	if err := os.WriteFile(filepath.Join(temp, "go.mod"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "slicing")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"values[:]", "values[:2]", "values[3:]", "values[1:4]", "values[1:3:4]", "value[\"X-Test\"][0]", "value[1:]", "pointer[:2]", "values[:upper()]"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "html/template"
  "net"
  "net/http"
  "sync/atomic"
  collections "example.com/collections"
)
func AliasesSliceStorage() int {
  values := []int{1, 2, 3, 4}
  view := values[1:3]
  view[0] = 9
  return values[1]
}
func AliasesFixedArrayStorage() int {
  values := [4]int{1, 2, 3, 4}
  view := values[1:3]
  view[0] = 8
  return values[1]
}
func PointerView(values *[4]int) []int { return values[:2] }
func AllForms(values []int) int {
  all := values[:]
  prefix := values[:2]
  suffix := values[3:]
  middle := values[1:4]
  full := values[1:3:4]
  return all[0] + prefix[1] + suffix[0] + middle[0] + full[1]
}
func StringBytes(value string) string { return value[1:4] }
func IPTail(value net.IP) []byte { return value[12:] }
func HeaderFirst(value http.Header) string { return value["X-Test"][0] }
func HTMLMiddle(value template.HTML) template.HTML { return value[1:4] }
func NamedArrayAlias() int {
  value := collections.NewTriple()
  view := value[1:]
  view[0] = 9
  return value[1]
}
func NamedArrayPointer() int {
  pointer := collections.NewTriplePointer()
  view := pointer[:2]
  view[0] = 7
  return pointer[0]
}
func ImportedBound(values []int) []int { return values[:2] }
func record(counter *atomic.Int64, digit int64, value int) int {
  counter.Store(counter.Load() * 10 + digit)
  return value
}
func observed(counter *atomic.Int64, values []int) []int {
  counter.Store(counter.Load() * 10 + 1)
  return values
}
func EvaluationOrder() int64 {
  var counter atomic.Int64
  values := []int{10, 20, 30, 40, 50}
  view := observed(&counter, values)[record(&counter, 2, 1):record(&counter, 3, 3):record(&counter, 4, 4)]
  return counter.Load() * 100 + int64(view[0])
}
func DynamicPanic(values []int, high int) []int { return values[:high] }
`
	testSource := `package slicing
import (
  "html/template"
  "net"
  "net/http"
  "slices"
  "testing"
  reference "slicing.test/reference"
)
func didPanic(operation func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  operation()
  return false
}
func TestSliceExpressions(t *testing.T) {
  if got, want := aliasesSliceStorage(), reference.AliasesSliceStorage(); got != want { t.Errorf("aliasesSliceStorage = %d, Go = %d", got, want) }
  if got, want := aliasesFixedArrayStorage(), reference.AliasesFixedArrayStorage(); got != want { t.Errorf("aliasesFixedArrayStorage = %d, Go = %d", got, want) }
  gotArray, wantArray := [4]int{1, 2, 3, 4}, [4]int{1, 2, 3, 4}
  gotView, wantView := pointerView(&gotArray), reference.PointerView(&wantArray)
  gotView[0], wantView[0] = 9, 9
  if !slices.Equal(gotView, wantView) || gotArray != wantArray { t.Errorf("pointerView = (%v, %v), Go = (%v, %v)", gotView, gotArray, wantView, wantArray) }
  for _, values := range [][]int{{1, 2, 3, 4, 5}, {-1, 0, 2, 4, 8}} {
    if got, want := allForms(values), reference.AllForms(values); got != want { t.Errorf("allForms(%v) = %d, Go = %d", values, got, want) }
  }
  for _, value := range []string{"hello", "a世b"} {
    if got, want := stringBytes(value), reference.StringBytes(value); got != want { t.Errorf("stringBytes(%q) = %q, Go = %q", value, got, want) }
  }
  for _, ip := range []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")} {
    if got, want := ipTail(ip), reference.IPTail(ip); !slices.Equal(got, want) { t.Errorf("ipTail(%v) = %v, Go = %v", ip, got, want) }
  }
  for _, header := range []http.Header{{"X-Test": []string{"ok"}}, {"X-Test": []string{"first", "second"}}} {
    if got, want := headerFirst(header), reference.HeaderFirst(header); got != want { t.Errorf("headerFirst = %q, Go = %q", got, want) }
  }
  for _, value := range []template.HTML{"hello", "a世b"} {
    if got, want := htmlMiddle(value), reference.HTMLMiddle(value); got != want { t.Errorf("htmlMiddle(%q) = %q, Go = %q", value, got, want) }
  }
  if got, want := namedArrayAlias(), reference.NamedArrayAlias(); got != want { t.Errorf("namedArrayAlias = %d, Go = %d", got, want) }
  if got, want := namedArrayPointer(), reference.NamedArrayPointer(); got != want { t.Errorf("namedArrayPointer = %d, Go = %d", got, want) }
  gotValues, wantValues := []int{4, 2, 1}, []int{4, 2, 1}
  gotBound, wantBound := importedBound(gotValues), reference.ImportedBound(wantValues)
  gotBound[0], wantBound[0] = 9, 9
  if !slices.Equal(gotBound, wantBound) || !slices.Equal(gotValues, wantValues) { t.Errorf("importedBound alias = (%v, %v), Go = (%v, %v)", gotBound, gotValues, wantBound, wantValues) }
  if got, want := evaluationOrder(), reference.EvaluationOrder(); got != want { t.Errorf("evaluationOrder = %d, Go = %d", got, want) }
  for _, test := range []struct { values []int; high int }{{[]int{1, 2}, 3}, {nil, 1}, {[]int{}, 1}} {
    gotPanic := didPanic(func() { dynamicPanic(test.values, test.high) })
    wantPanic := didPanic(func() { reference.DynamicPanic(test.values, test.high) })
    if gotPanic != wantPanic { t.Errorf("dynamicPanic(%v, %d) panic = %v, Go = %v", test.values, test.high, gotPanic, wantPanic) }
  }
}
`
	runGeneratedGoDifferentialTestWithModule(t, temp, "slicing.test", module, generated, referenceSource, testSource)
}
