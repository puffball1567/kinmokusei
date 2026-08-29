package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoStandardLibraryInteropCompilesAndRuns(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "interop.otm")
	input := `
import go strings from "strings";
import go strconv from "strconv";
import go math from "math";
import go fmt from "fmt";
import go path from "path";
import go runtime from "runtime";
const circleConstant: float = math.Pi;
function normalize(value: string): string { return strings.ToUpper(strings.TrimSpace(value)); }
function secondWord(value: string): string { return strings.Fields(value)[1]; }
function digits(value: int): string { return strconv.Itoa(value); }
function circleRatio(): float { return circleConstant; }
function joinNone(): string { return path.Join(); }
function joinMany(): string { return path.Join("a", "b", "c"); }
function formatItem(): string { return fmt.Sprintf("%s:%d", "item", 2); }
function collect(): void { runtime.GC(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "interop")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{`strings "strings"`, `strconv "strconv"`, `math "math"`, `fmt "fmt"`, `path "path"`, `runtime "runtime"`, "const circleConstant float64 = math.Pi", "strings.ToUpper", "strconv.Itoa", `path.Join("a", "b", "c")`, `fmt.Sprintf("%s:%d", "item", 2)`, "runtime.GC"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "fmt"
  "math"
  "path"
  "runtime"
  "strconv"
  "strings"
)
const circleConstant float64 = math.Pi
func Normalize(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }
func SecondWord(value string) string { return strings.Fields(value)[1] }
func Digits(value int) string { return strconv.Itoa(value) }
func CircleRatio() float64 { return circleConstant }
func JoinNone() string { return path.Join() }
func JoinMany() string { return path.Join("a", "b", "c") }
func FormatItem() string { return fmt.Sprintf("%s:%d", "item", 2) }
func Collect() { runtime.GC() }
`
	testSource := `package interop
import (
  "testing"
  reference "interop.test/reference"
)
func TestInterop(t *testing.T) {
  collect()
  reference.Collect()
  for _, value := range []string{"  hello  ", "温 泉", "", "MiXeD"} {
    if got, want := normalize(value), reference.Normalize(value); got != want { t.Errorf("normalize(%q) = %q, Go = %q", value, got, want) }
  }
  for _, value := range []string{"hello world", "a  b c", "温 泉"} {
    if got, want := secondWord(value), reference.SecondWord(value); got != want { t.Errorf("secondWord(%q) = %q, Go = %q", value, got, want) }
  }
  for _, value := range []int{-42, 0, 42} {
    if got, want := digits(value), reference.Digits(value); got != want { t.Errorf("digits(%d) = %q, Go = %q", value, got, want) }
  }
  if got, want := circleRatio(), reference.CircleRatio(); got != want { t.Errorf("circleRatio = %v, Go = %v", got, want) }
  if got, want := joinNone(), reference.JoinNone(); got != want { t.Errorf("joinNone = %q, Go = %q", got, want) }
  if got, want := joinMany(), reference.JoinMany(); got != want { t.Errorf("joinMany = %q, Go = %q", got, want) }
  if got, want := formatItem(), reference.FormatItem(); got != want { t.Errorf("formatItem = %q, Go = %q", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "interop.test", generated, referenceSource, testSource)
}

func TestGoCallbacksClosuresAndFunctionValuesCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "callbacks.otm")
	input := `
import go sort from "sort";
import go strings from "strings";
function identityRune(value: int32): int32 { return value; }
function sortedDigits(): int {
  const items = [3, 1, 2];
  sort.Slice(items, (left: int, right: int): boolean => items[left] < items[right]);
  return items[0] * 100 + items[1] * 10 + items[2];
}
function mapped(): string { return strings.Map((value: int32): int32 => value + 1, "ab"); }
function namedMapped(): string { return strings.Map(identityRune, "ab"); }
function packageFunctionValue(): string { const upper = strings.ToUpper; return upper("ab"); }
function boundMethodValue(): string { const replace = strings.NewReplacer("a", "b").Replace; return replace("a-cat"); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "callbacks")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"sort.Slice(items, func(left int, right int) bool", "strings.Map(identityRune", "var upper = strings.ToUpper", ".Replace"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "sort"
  "strings"
)
func identityRune(value rune) rune { return value }
func SortedDigits() int { items := []int{3, 1, 2}; sort.Slice(items, func(left, right int) bool { return items[left] < items[right] }); return items[0]*100+items[1]*10+items[2] }
func Mapped() string { return strings.Map(func(value rune) rune { return value+1 }, "ab") }
func NamedMapped() string { return strings.Map(identityRune, "ab") }
func PackageFunctionValue() string { upper := strings.ToUpper; return upper("ab") }
func BoundMethodValue() string { replace := strings.NewReplacer("a", "b").Replace; return replace("a-cat") }
`
	testSource := `package callbacks
import (
  "testing"
  reference "callbacks.test/reference"
)
func TestCallbacks(t *testing.T) {
	if got, want := sortedDigits(), reference.SortedDigits(); got != want { t.Errorf("sortedDigits = %v, Go = %v", got, want) }
	if got, want := mapped(), reference.Mapped(); got != want { t.Errorf("mapped = %q, Go = %q", got, want) }
	if got, want := namedMapped(), reference.NamedMapped(); got != want { t.Errorf("namedMapped = %q, Go = %q", got, want) }
	if got, want := packageFunctionValue(), reference.PackageFunctionValue(); got != want { t.Errorf("packageFunctionValue = %q, Go = %q", got, want) }
	if got, want := boundMethodValue(), reference.BoundMethodValue(); got != want { t.Errorf("boundMethodValue = %q, Go = %q", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "callbacks.test", generated, referenceSource, testSource)
}

func TestGoInterfaceValuesAndExplicitClassImplementationCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "interfaces.otm")
	input := `
import go io from "io";
import go sort from "sort";
import go strings from "strings";
class Sequence implements sort.Interface {
  constructor(private count: int) {}
  public function len(): int { return this.count; }
  public function less(left: int, right: int): boolean { return false; }
  public function swap(left: int, right: int): void {}
}
function copiedBytes(): int64 {
  const [count, err] = io.Copy(io.Discard, strings.NewReader("abc"));
  return count;
}
function closeReader(): error { return io.NopCloser(strings.NewReader("abc")).Close(); }
function nilReader(): boolean { let reader: io.Reader = nil; return reader == nil; }
function sortSequence(): void { sort.Sort(new Sequence(1)); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "interfaces")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"var _ sort.Interface = &Sequence{}", "io.Copy(io.Discard, strings.NewReader", "var reader io.Reader = nil", "sort.Sort(NewSequence(1))"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "io"
  "sort"
  "strings"
)
type Sequence struct { count int }
func NewSequence(count int) *Sequence { return &Sequence{count: count} }
func (sequence *Sequence) Len() int { return sequence.count }
func (*Sequence) Less(left, right int) bool { return false }
func (*Sequence) Swap(left, right int) {}
func CopiedBytes() int64 { count, _ := io.Copy(io.Discard, strings.NewReader("abc")); return count }
func CloseReader() error { return io.NopCloser(strings.NewReader("abc")).Close() }
func NilReader() bool { var reader io.Reader; return reader == nil }
func SortSequence() { sort.Sort(NewSequence(1)) }
`
	testSource := `package interfaces
import (
  "testing"
  reference "interfaces.test/reference"
)
func TestInterfaces(t *testing.T) {
	if got, want := copiedBytes(), reference.CopiedBytes(); got != want { t.Errorf("copiedBytes = %v, Go = %v", got, want) }
	gotErr, wantErr := closeReader(), reference.CloseReader()
	if (gotErr == nil) != (wantErr == nil) { t.Errorf("closeReader = %v, Go = %v", gotErr, wantErr) }
	if got, want := nilReader(), reference.NilReader(); got != want { t.Errorf("nilReader = %v, Go = %v", got, want) }
	sortSequence()
	reference.SortSequence()
}
`
	runGeneratedGoDifferentialTest(t, temp, "interfaces.test", generated, referenceSource, testSource)
}

func TestGoGenericInferenceAndExplicitArgumentsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "generics.otm")
	input := `
import go maps from "maps";
import go slices from "slices";
function contains(items: int[]): boolean { return slices.Contains(items, 2); }
function cloned(items: int[]): int[] { return slices.Clone(items); }
function indexed(items: int[]): int { return slices.IndexFunc(items, (item: int): boolean => item > 2); }
function clonedMap(items: Map<string, int>): Map<string, int> { return maps.Clone(items); }
function concatenated(items: int[][]): int[] { return slices.Concat(items...); }
function explicitClone(items: int[]): int[] { return slices.Clone[int[]](items); }
function explicitEmptyConcat(): int[] { return slices.Concat[int[]](); }
function explicitMap(items: Map<string, int>): Map<string, int> { return maps.Clone[Map<string, int>, string, int](items); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "generics")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"slices.Contains(items, 2)", "slices.Clone(items)", "slices.IndexFunc", "maps.Clone(items)", "slices.Concat(items...)", "slices.Clone[[]int](items)", "slices.Concat[[]int]()", "maps.Clone[map[string]int, string, int](items)"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "maps"
  "slices"
)
func Contains(items []int) bool { return slices.Contains(items, 2) }
func Cloned(items []int) []int { return slices.Clone(items) }
func Indexed(items []int) int { return slices.IndexFunc(items, func(item int) bool { return item>2 }) }
func ClonedMap(items map[string]int) map[string]int { return maps.Clone(items) }
func Concatenated(items [][]int) []int { return slices.Concat(items...) }
func ExplicitClone(items []int) []int { return slices.Clone[[]int](items) }
func ExplicitEmptyConcat() []int { return slices.Concat[[]int]() }
func ExplicitMap(items map[string]int) map[string]int { return maps.Clone[map[string]int, string, int](items) }
`
	testSource := `package generics
import (
  "maps"
  "slices"
  "testing"
  reference "generics.test/reference"
)
func TestGenerics(t *testing.T) {
	for _, items := range [][]int{nil, {}, {1}, {1, 2, 3, 4}, {3, -1, 2}} {
	  if got, want := contains(items), reference.Contains(items); got != want { t.Errorf("contains(%v) = %v, Go = %v", items, got, want) }
	  if got, want := indexed(items), reference.Indexed(items); got != want { t.Errorf("indexed(%v) = %v, Go = %v", items, got, want) }
	  got, want := cloned(items), reference.Cloned(items)
	  if !slices.Equal(got, want) || (got == nil) != (want == nil) { t.Errorf("cloned(%v) = %v, Go = %v", items, got, want) }
	  gotExplicit, wantExplicit := explicitClone(items), reference.ExplicitClone(items)
	  if !slices.Equal(gotExplicit, wantExplicit) || (gotExplicit == nil) != (wantExplicit == nil) { t.Errorf("explicitClone(%v) = %v, Go = %v", items, gotExplicit, wantExplicit) }
	}
	for _, items := range []map[string]int{nil, {}, {"x": 7}, {"x": -1, "y": 8}} {
	  got, want := clonedMap(items), reference.ClonedMap(items)
	  if !maps.Equal(got, want) || (got == nil) != (want == nil) { t.Errorf("clonedMap(%v) = %v, Go = %v", items, got, want) }
	  gotExplicit, wantExplicit := explicitMap(items), reference.ExplicitMap(items)
	  if !maps.Equal(gotExplicit, wantExplicit) || (gotExplicit == nil) != (wantExplicit == nil) { t.Errorf("explicitMap(%v) = %v, Go = %v", items, gotExplicit, wantExplicit) }
	}
	for _, items := range [][][]int{nil, {}, {{1, 2}, {3}}, {{}, {-1}}, {nil, {4}}} {
	  if got, want := concatenated(items), reference.Concatenated(items); !slices.Equal(got, want) || (got == nil) != (want == nil) { t.Errorf("concatenated(%v) = %v, Go = %v", items, got, want) }
	}
	if got, want := explicitEmptyConcat(), reference.ExplicitEmptyConcat(); !slices.Equal(got, want) || (got == nil) != (want == nil) { t.Errorf("explicitEmptyConcat = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "generics.test", generated, referenceSource, testSource)
}

func TestGoGenericNamedTypesAndMethodsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "generic_types.otm")
	input := `
import go atomic from "sync/atomic";
import go time from "time";
import go unique from "unique";
function roundTrip(value: string): string {
  let pointer: atomic.Pointer<string> = atomic.Pointer<string>{};
  pointer.Store(&value);
  return *pointer.Load();
}
function previous(first: string, second: string): string {
  let pointer: atomic.Pointer<string> = atomic.Pointer<string>{};
  pointer.Store(&first);
  return *pointer.Swap(&second);
}
function canonical(value: string): string { const handle = unique.Make(value); return handle.Value(); }
function explicitCanonical(value: string): string { const handle: unique.Handle<string> = unique.Make(value); return handle.Value(); }
function handles(value: string): unique.Handle<string>[] { return [unique.Make(value)]; }
function durationPointer(): atomic.Pointer<time.Duration> { return atomic.Pointer<time.Duration>{}; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "generictypes")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"atomic.Pointer[string]", "pointer.Store(&value)", "pointer.Load()", "pointer.Swap(&second)", "unique.Make(value)", "[]unique.Handle[string]", "atomic.Pointer[time.Duration]"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "sync/atomic"
  "time"
  "unique"
)
func RoundTrip(value string) string { var pointer atomic.Pointer[string]; pointer.Store(&value); return *pointer.Load() }
func Previous(first, second string) string { var pointer atomic.Pointer[string]; pointer.Store(&first); return *pointer.Swap(&second) }
func Canonical(value string) string { return unique.Make(value).Value() }
func ExplicitCanonical(value string) string { var handle unique.Handle[string] = unique.Make(value); return handle.Value() }
func Handles(value string) []unique.Handle[string] { return []unique.Handle[string]{unique.Make(value)} }
func DurationPointer() atomic.Pointer[time.Duration] { return atomic.Pointer[time.Duration]{} }
`
	testSource := `package generictypes
import (
  "testing"
  reference "generictypes.test/reference"
)
func TestGenericTypes(t *testing.T) {
	for _, value := range []string{"", "first", "温泉"} {
	  if got, want := roundTrip(value), reference.RoundTrip(value); got != want { t.Errorf("roundTrip(%q) = %q, Go = %q", value, got, want) }
	  if got, want := canonical(value), reference.Canonical(value); got != want { t.Errorf("canonical(%q) = %q, Go = %q", value, got, want) }
	  if got, want := explicitCanonical(value), reference.ExplicitCanonical(value); got != want { t.Errorf("explicitCanonical(%q) = %q, Go = %q", value, got, want) }
	  got, want := handles(value), reference.Handles(value)
	  if len(got) != len(want) || got[0].Value() != want[0].Value() { t.Errorf("handles(%q) = %v, Go = %v", value, got, want) }
	}
	for _, pair := range [][2]string{{"first", "second"}, {"", "x"}, {"温", "泉"}} {
	  if got, want := previous(pair[0], pair[1]), reference.Previous(pair[0], pair[1]); got != want { t.Errorf("previous(%q, %q) = %q, Go = %q", pair[0], pair[1], got, want) }
	}
	gotPointer, wantPointer := durationPointer(), reference.DurationPointer()
	if got, want := gotPointer.Load() == nil, wantPointer.Load() == nil; got != want { t.Errorf("durationPointer nil = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "generictypes.test", generated, referenceSource, testSource)
}

func TestGoCheckedAndUncheckedTypeAssertionsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "assertions.otm")
	input := `
import go io from "io";
import go strings from "strings";
function forceReader(reader: io.Reader): *strings.Reader { return reader as! *strings.Reader; }
function isReader(reader: io.Reader): boolean { const [typed, ok] = reader as? *strings.Reader; return ok; }
function isWriter(reader: io.Reader): boolean { const [writer, ok] = reader as? io.Writer; return ok; }
function forceWriter(reader: io.Reader): io.Writer { return reader as! io.Writer; }
function nilReader(): boolean { let reader: io.Reader = nil; const [typed, ok] = reader as? *strings.Reader; return ok; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "assertions")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"return reader.(*strings.Reader)", "var typed, ok = reader.(*strings.Reader)", "var writer, ok = reader.(io.Writer)", "return reader.(io.Writer)"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "io"
  "strings"
)
func ForceReader(reader io.Reader) *strings.Reader { return reader.(*strings.Reader) }
func IsReader(reader io.Reader) bool { _, ok := reader.(*strings.Reader); return ok }
func IsWriter(reader io.Reader) bool { _, ok := reader.(io.Writer); return ok }
func ForceWriter(reader io.Reader) io.Writer { return reader.(io.Writer) }
func NilReader() bool { var reader io.Reader; _, ok := reader.(*strings.Reader); return ok }
`
	testSource := `package assertions
import (
	"bytes"
	"strings"
	"testing"
	reference "assertions.test/reference"
)
func didPanic(call func()) (panicked bool) { defer func() { panicked = recover() != nil }(); call(); return false }
func TestAssertions(t *testing.T) {
	reader := strings.NewReader("abc")
	if got, want := forceReader(reader).Len(), reference.ForceReader(strings.NewReader("abc")).Len(); got != want { t.Errorf("forceReader len = %d, Go = %d", got, want) }
	if got, want := isReader(reader), reference.IsReader(strings.NewReader("abc")); got != want { t.Errorf("isReader = %v, Go = %v", got, want) }
	if got, want := isWriter(reader), reference.IsWriter(strings.NewReader("abc")); got != want { t.Errorf("isWriter = %v, Go = %v", got, want) }
	buffer := bytes.NewBufferString("abc")
	if got, want := isWriter(buffer), reference.IsWriter(bytes.NewBufferString("abc")); got != want { t.Errorf("buffer isWriter = %v, Go = %v", got, want) }
	if got, want := nilReader(), reference.NilReader(); got != want { t.Errorf("nilReader = %v, Go = %v", got, want) }
	if got, want := didPanic(func() { forceWriter(reader) }), didPanic(func() { reference.ForceWriter(strings.NewReader("abc")) }); got != want || !got { t.Errorf("forceWriter panic = %v, Go = %v", got, want) }
	if got, want := didPanic(func() { forceReader(buffer) }), didPanic(func() { reference.ForceReader(bytes.NewBufferString("abc")) }); got != want || !got { t.Errorf("forceReader panic = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "assertions.test", generated, referenceSource, testSource)
}

func TestGoNamedTypesPointersFieldsMethodsAndVariablesCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "level_one.otm")
	input := `
import go bytes from "bytes";
import go http from "net/http";
import go os from "os";
import go strings from "strings";
import go time from "time";
function pointerRoundTrip(value: time.Duration): time.Duration {
  let copy: time.Duration = value;
  const pointer: *time.Duration = &copy;
  return *pointer;
}
function timeout(): time.Duration {
  const client: http.Client = http.Client{ Timeout: time.Second * 2 };
  return client.Timeout;
}
function nilClient(): boolean { let client: *http.Client = nil; return client == nil; }
function currentUnix(): int64 { return time.Now().Unix(); }
function outputName(): string { return os.Stdout.Name(); }
function replaceOutput(output: *os.File): void { os.Stdout = output; }
function readerLength(): int { return strings.NewReader("abc").Len(); }
function resetBuffer(): string { let buffer: bytes.Buffer = bytes.Buffer{}; buffer.Reset(); return buffer.String(); }
function duration(value: int64): time.Duration { return time.Duration(value); }
function inferredCallback(): time.Duration { const readTimeout = () => http.DefaultClient.Timeout; return readTimeout(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "levelone")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{"value time.Duration", "*time.Duration", "&copy", "http.Client{Timeout: time.Second * 2}", "os.Stdout.Name()", "strings.NewReader", "buffer.Reset()", "time.Duration(value)", "func() time.Duration"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "bytes"
  "net/http"
  "os"
  "strings"
  "time"
)
func PointerRoundTrip(value time.Duration) time.Duration { copy := value; pointer := &copy; return *pointer }
func Timeout() time.Duration { client := http.Client{Timeout: time.Second*2}; return client.Timeout }
func NilClient() bool { var client *http.Client; return client == nil }
func CurrentUnix() int64 { return time.Now().Unix() }
func OutputName() string { return os.Stdout.Name() }
func ReplaceOutput(output *os.File) { os.Stdout = output }
func ReaderLength() int { return strings.NewReader("abc").Len() }
func ResetBuffer() string { var buffer bytes.Buffer; buffer.Reset(); return buffer.String() }
func Duration(value int64) time.Duration { return time.Duration(value) }
func InferredCallback() time.Duration { readTimeout := func() time.Duration { return http.DefaultClient.Timeout }; return readTimeout() }
`
	testSource := `package levelone
import (
  "os"
  "testing"
  "time"
  reference "levelone.test/reference"
)
func TestLevelOne(t *testing.T) {
  for _, value := range []time.Duration{-time.Second, 0, 7, 2*time.Second} {
    if got, want := pointerRoundTrip(value), reference.PointerRoundTrip(value); got != want { t.Errorf("pointerRoundTrip(%v) = %v, Go = %v", value, got, want) }
  }
  if got, want := timeout(), reference.Timeout(); got != want { t.Errorf("timeout = %v, Go = %v", got, want) }
  if got, want := nilClient(), reference.NilClient(); got != want { t.Errorf("nilClient = %v, Go = %v", got, want) }
  gotUnix, wantUnix := currentUnix(), reference.CurrentUnix()
  if delta := gotUnix-wantUnix; delta < -1 || delta > 1 { t.Errorf("currentUnix = %d, Go = %d", gotUnix, wantUnix) }
  if got, want := outputName(), reference.OutputName(); got != want { t.Errorf("outputName = %q, Go = %q", got, want) }
  if got, want := readerLength(), reference.ReaderLength(); got != want { t.Errorf("readerLength = %v, Go = %v", got, want) }
  if got, want := resetBuffer(), reference.ResetBuffer(); got != want { t.Errorf("resetBuffer = %q, Go = %q", got, want) }
  for _, value := range []int64{-5, 0, 5, 1<<40} {
    if got, want := duration(value), reference.Duration(value); got != want { t.Errorf("duration(%d) = %v, Go = %v", value, got, want) }
  }
  if got, want := inferredCallback(), reference.InferredCallback(); got != want { t.Errorf("inferredCallback = %v, Go = %v", got, want) }
  original := os.Stdout
  output, err := os.CreateTemp(t.TempDir(), "output")
  if err != nil { t.Fatal(err) }
  defer output.Close()
  replaceOutput(output)
  gotReplaced := os.Stdout == output
  os.Stdout = original
  reference.ReplaceOutput(output)
  wantReplaced := os.Stdout == output
  os.Stdout = original
  if gotReplaced != wantReplaced || !gotReplaced { t.Errorf("replaceOutput = %v, Go = %v", gotReplaced, wantReplaced) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "levelone.test", generated, referenceSource, testSource)
}

func TestGoMultipleResultsAndErrorCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "level_two.otm")
	input := `
import go errors from "errors";
import go strconv from "strconv";
function parse(value: string): int {
  const [parsed, err] = strconv.Atoi(value);
  if (err != nil) { return -1; }
  return parsed;
}

function reparse(value: string): int {
  let parsed: int = 0;
  let err: error = nil;
  [parsed, err] = strconv.Atoi(value);
  if (err != nil) { return -1; }
  return parsed;
}
function message(value: string): string {
  const [_, err] = strconv.Atoi(value);
  if (err == nil) { return ""; }
  return err.Error();
}
function discard(value: string): void { const [_, _] = strconv.Atoi(value); }
function makeError(): error { return errors.New("boom"); }
function loopParse(value: string): int {
  for (const [parsed, err] = strconv.Atoi(value); err == nil; ) { return parsed; }
  return -1;
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "leveltwo")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{"var parsed, err = strconv.Atoi(value)", "var err error = nil", "parsed, err = strconv.Atoi(value)", "var _, _ = strconv.Atoi(value)", "err.Error()", "func makeError() error", "for parsed, err := strconv.Atoi(value); err == nil;"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "errors"
  "strconv"
)
func Parse(value string) int { parsed, err := strconv.Atoi(value); if err != nil { return -1 }; return parsed }
func Reparse(value string) int { parsed := 0; var err error; parsed, err = strconv.Atoi(value); if err != nil { return -1 }; return parsed }
func Message(value string) string { _, err := strconv.Atoi(value); if err == nil { return "" }; return err.Error() }
func Discard(value string) { _, _ = strconv.Atoi(value) }
func MakeError() error { return errors.New("boom") }
func LoopParse(value string) int { for parsed, err := strconv.Atoi(value); err == nil; { return parsed }; return -1 }
`
	testSource := `package leveltwo
import (
  "testing"
  reference "leveltwo.test/reference"
)
func TestLevelTwo(t *testing.T) {
  for _, value := range []string{"42", "-7", "0", "nope", "", " 1"} {
    if got, want := parse(value), reference.Parse(value); got != want { t.Errorf("parse(%q) = %v, Go = %v", value, got, want) }
    if got, want := reparse(value), reference.Reparse(value); got != want { t.Errorf("reparse(%q) = %v, Go = %v", value, got, want) }
    if got, want := message(value), reference.Message(value); got != want { t.Errorf("message(%q) = %q, Go = %q", value, got, want) }
    if got, want := loopParse(value), reference.LoopParse(value); got != want { t.Errorf("loopParse(%q) = %v, Go = %v", value, got, want) }
    discard(value)
    reference.Discard(value)
  }
  gotErr, wantErr := makeError(), reference.MakeError()
  if gotErr == nil || wantErr == nil || gotErr.Error() != wantErr.Error() { t.Errorf("makeError = %v, Go = %v", gotErr, wantErr) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "leveltwo.test", generated, referenceSource, testSource)
}

func TestExternalGoModuleViaLocalReplaceCompilesAndRuns(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	mainModule := `module example.com/application

go 1.23

require example.com/library v0.0.0

replace example.com/library => ./library
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mainModule), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	librarySource := `package library

import (
	"strconv"
	"time"
)

const Version = "v1"
type ID int64
type Config struct { Prefix string }
func Parse(text string) (ID, error) {
	value, err := strconv.ParseInt(text, 10, 64)
	return ID(value), err
}

func (config Config) Render(value ID) string { return config.Prefix + strconv.FormatInt(int64(value), 10) }
func Delay() time.Duration { return 2 * time.Second }
func Sum(prefix int, values ...int) int {
	total := prefix
	for _, value := range values { total += value }
	return total
}
func Eight(value int8) int8 { return value }
func Sixteen(value int16) int16 { return value }
func hidden() int { return 1 }
`
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte(librarySource), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main.otm")
	ontamaSource := `
import go library from "example.com/library";
import go time from "time";
function render(text: string): string {
  const [identifier, err] = library.Parse(text);
  if (err != nil) { return err.Error(); }
  const config: library.Config = library.Config{ Prefix: "id:" };
  return config.Render(identifier);
}
function dependencyVersion(): string { return library.Version; }
function dependencyDelay(): time.Duration { return library.Delay(); }
function dependencySum(): int { return library.Sum(10, 1, 2, 3); }
function dependencySpread(values: int[]): int { return library.Sum(10, values...); }
function narrowIntegers(): int { return int(library.Eight(8)) + int(library.Sixteen(16)); }
`
	if err := os.WriteFile(source, []byte(ontamaSource), 0o644); err != nil {
		t.Fatal(err)
	}
	directory, diagnostics, err := WriteGeneratedModule([]string{source}, "externalmodule")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("directory=%q err=%v diagnostics=%v", directory, err, diagnostics)
	}
	generated, err := os.ReadFile(filepath.Join(directory, "generated.go"))
	if err != nil || !strings.Contains(string(generated), "library.Sum(10, values...)") {
		t.Fatalf("generated spread call missing: err=%v\n%s", err, generated)
	}
	if _, err := os.Stat(filepath.Join(directory, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("generated nested go.mod should not exist, err=%v", err)
	}
	moduleAfter, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(moduleAfter) != mainModule {
		t.Fatalf("main go.mod changed: err=%v\n%s", err, moduleAfter)
	}
	referenceSource := `package reference
import (
  library "example.com/library"
  "time"
)
func Render(text string) string {
  identifier, err := library.Parse(text)
  if err != nil { return err.Error() }
  config := library.Config{Prefix: "id:"}
  return config.Render(identifier)
}
func DependencyVersion() string { return library.Version }
func DependencyDelay() time.Duration { return library.Delay() }
func DependencySum() int { return library.Sum(10, 1, 2, 3) }
func DependencySpread(values []int) int { return library.Sum(10, values...) }
func NarrowIntegers() int { return int(library.Eight(8))+int(library.Sixteen(16)) }
`
	testSource := `package externalmodule
import (
  "testing"
  reference "externalmodule.test/reference"
)
func TestExternalModule(t *testing.T) {
	for _, value := range []string{"42", "-7", "0", "bad", ""} {
	  if got, want := render(value), reference.Render(value); got != want { t.Errorf("render(%q) = %q, Go = %q", value, got, want) }
	}
	if got, want := dependencyVersion(), reference.DependencyVersion(); got != want { t.Errorf("version = %q, Go = %q", got, want) }
	if got, want := dependencyDelay(), reference.DependencyDelay(); got != want { t.Errorf("delay = %v, Go = %v", got, want) }
	if got, want := dependencySum(), reference.DependencySum(); got != want { t.Errorf("sum = %v, Go = %v", got, want) }
	for _, values := range [][]int{nil, {}, {4, 5}, {-1, 2, 3}} {
	  if got, want := dependencySpread(values), reference.DependencySpread(values); got != want { t.Errorf("spread(%v) = %v, Go = %v", values, got, want) }
	}
	if got, want := narrowIntegers(), reference.NarrowIntegers(); got != want { t.Errorf("narrow integers = %v, Go = %v", got, want) }
}
`
	differentialModule := `module externalmodule.test

go 1.23

require example.com/library v0.0.0

replace example.com/library => ../../library
`
	t.Setenv("GOPROXY", "off")
	runGeneratedGoDifferentialTestWithModule(t, directory, "externalmodule.test", differentialModule, generated, referenceSource, testSource)

	unexported := `import go library from "example.com/library"; function value(): int { return library.hidden(); }`
	result, err := CheckFilesWithOverlay([]string{source}, map[string]string{source: unexported})
	if err != nil {
		t.Fatal(err)
	}
	messages := make([]string, len(result.Diagnostics))
	for i, item := range result.Diagnostics {
		messages[i] = item.Message
	}
	if !strings.Contains(strings.Join(messages, "\n"), `has no exported member "hidden"`) {
		t.Fatalf("unexported external member diagnostics=%v", messages)
	}

	narrowMismatch := `import go library from "example.com/library"; function value(): int { return int(library.Eight(library.Sixteen(1))); }`
	result, err = CheckFilesWithOverlay([]string{source}, map[string]string{source: narrowMismatch})
	if err != nil {
		t.Fatal(err)
	}
	messages = messages[:0]
	for _, item := range result.Diagnostics {
		messages = append(messages, item.Message)
	}
	if !strings.Contains(strings.Join(messages, "\n"), "cannot use int16 as int8") {
		t.Fatalf("narrow integer mismatch diagnostics=%v", messages)
	}
}

func TestExternalGoModuleRequiresReadonlyModuleGraph(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	module := `module example.com/application

go 1.23

replace example.com/library => ./library
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package library\nfunc Value() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main.otm")
	if err := os.WriteFile(source, []byte(`import go library from "example.com/library"; function value(): int { return library.Value(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	messages := make([]string, len(result.Diagnostics))
	for i, item := range result.Diagnostics {
		messages[i] = item.Message
	}
	if !strings.Contains(strings.Join(messages, "\n"), "replaced but not required") {
		t.Fatalf("diagnostics=%v", messages)
	}
	contents, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(contents) != module {
		t.Fatalf("go.mod changed: err=%v\n%s", err, contents)
	}
}

func TestExternalGoModuleDoesNotDownloadMissingDependency(t *testing.T) {
	root := t.TempDir()
	module := `module example.com/application

go 1.23

require example.invalid/unavailable v1.0.0
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	checksums := `example.invalid/unavailable v1.0.0 h1:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=
example.invalid/unavailable v1.0.0/go.mod h1:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=
`
	if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main.otm")
	if err := os.WriteFile(source, []byte(`import go unavailable from "example.invalid/unavailable"; function value(): int { return unavailable.Value(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	messages := make([]string, len(result.Diagnostics))
	for i, item := range result.Diagnostics {
		messages[i] = item.Message
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, "cannot load Go package") || !strings.Contains(joined, "GOPROXY=off") {
		t.Fatalf("diagnostics=%v", messages)
	}
	contents, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(contents) != module {
		t.Fatalf("go.mod changed: err=%v\n%s", err, contents)
	}
	contents, err = os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil || string(contents) != checksums {
		t.Fatalf("go.sum changed: err=%v\n%s", err, contents)
	}
}

func TestUnusedGoImportIsNotGenerated(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "unused.otm")
	if err := os.WriteFile(source, []byte(`import go strings from "strings"; function answer(): int { return 42; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "unused")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), `"strings"`) {
		t.Fatalf("unused Go import was generated:\n%s", generated)
	}
}

func TestGoInteropFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing external package", `import go library from "example.com/library"; function value(): int { return 1; }`, "cannot load Go package"},
		{"blank alias", `import go _ from "strings"; function value(): int { return 1; }`, "cannot be used as a namespace"},
		{"unsafe package", `import go unsafe from "unsafe"; function value(): int { return 1; }`, "requires [go.interop]"},
		{"internal package", `import go abi from "internal/abi"; function value(): int { return 1; }`, "not available in current Go interop"},
		{"unknown member", `import go strings from "strings"; function value(): string { return strings.Missing("x"); }`, "has no exported member"},
		{"multiple returns", `import go os from "os"; function value(): string { return os.Getwd(); }`, "require destructuring"},
		{"variadic too few fixed arguments", `import go fmt from "fmt"; function value(): void { fmt.Fprintf(); }`, "expects at least 2 arguments, got 0"},
		{"type symbol", `import go strings from "strings"; function value(): int { const reader = strings.Reader; return 1; }`, "cannot be used as a value"},
		{"package as value", `import go strings from "strings"; function value(): int { const packageValue = strings; return 1; }`, "cannot be used as a value"},
		{"wrong argument type", `import go strings from "strings"; function value(): string { return strings.ToUpper(1); }`, "cannot use integer literal as string"},
		{"wrong argument count", `import go strings from "strings"; function value(): string { return strings.ToUpper(); }`, "expects 1 arguments, got 0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temp := t.TempDir()
			source := filepath.Join(temp, "invalid.otm")
			if err := os.WriteFile(source, []byte(test.source), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := CheckFiles([]string{source})
			if err != nil {
				t.Fatal(err)
			}
			messages := make([]string, len(result.Diagnostics))
			for i, item := range result.Diagnostics {
				messages[i] = item.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}

func TestGoImportAliasesAreLinkedAcrossModules(t *testing.T) {
	temp := t.TempDir()
	files := map[string]string{
		"upper.otm":  `import go text from "strings"; function upper(value: string): string { return text.ToUpper(value); }`,
		"trim.otm":   `import go words from "strings"; function trim(value: string): string { return words.TrimSpace(value); }`,
		"digits.otm": `import go text from "strconv"; function digits(value: int): string { return text.Itoa(value); }`,
		"entry.otm":  `import { upper } from "./upper"; import { trim } from "./trim"; import { digits } from "./digits"; function value(): string { return upper(trim(" x ")) + digits(1); }`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(temp, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(temp, "entry.otm")}, "aliases")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	if strings.Count(text, `"strings"`) != 1 || strings.Count(text, `"strconv"`) != 1 {
		t.Fatalf("Go imports were not canonicalized:\n%s", generated)
	}
	if !strings.Contains(text, ".ToUpper") || !strings.Contains(text, ".TrimSpace") || !strings.Contains(text, ".Itoa") {
		t.Fatalf("linked selectors are missing:\n%s", generated)
	}
	referenceSource := `package reference
import (
  "strconv"
  "strings"
)
func Value() string { return strings.ToUpper(strings.TrimSpace(" x "))+strconv.Itoa(1) }
`
	testSource := `package aliases
import (
  "testing"
  reference "aliases.test/reference"
)
func TestAliases(t *testing.T) {
  if got, want := value(), reference.Value(); got != want { t.Errorf("value = %q, Go = %q", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "aliases.test", generated, referenceSource, testSource)
}

func TestGoMultipleAssignmentLinksGlobalAcrossModules(t *testing.T) {
	temp := t.TempDir()
	files := map[string]string{
		"state.otm": `import go os from "os"; let directory: string = ""; function refresh(): boolean { let err: error = nil; [directory, err] = os.Getwd(); return err == nil; }`,
		"entry.otm": `import { directory, refresh } from "./state"; function current(): string { refresh(); return directory; }`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(temp, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(temp, "entry.otm")}, "multilink")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	if !strings.Contains(text, "directory, err = os.Getwd()") || !strings.Contains(text, "return directory") {
		t.Fatalf("linked multiple assignment is missing:\n%s", generated)
	}
	referenceSource := `package reference
import "os"
var directory string
func refresh() bool { var err error; directory, err = os.Getwd(); return err == nil }
func Current() string { refresh(); return directory }
`
	testSource := `package multilink
import (
  "testing"
  reference "multilink.test/reference"
)
func TestMultipleAssignment(t *testing.T) {
  if got, want := current(), reference.Current(); got != want { t.Errorf("current = %q, Go = %q", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "multilink.test", generated, referenceSource, testSource)
}

func TestQualifiedGoTypesUseCanonicalAliasesAcrossModules(t *testing.T) {
	temp := t.TempDir()
	files := map[string]string{
		"add.otm":    `import go clock from "time"; function add(value: clock.Duration): clock.Duration { return value + clock.Second; }`,
		"double.otm": `import go moment from "time"; function double(value: moment.Duration): moment.Duration { return value * 2; }`,
		"entry.otm":  `import go duration from "time"; import { add } from "./add"; import { double } from "./double"; function value(): duration.Duration { return double(add(duration.Second)); }`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(temp, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(temp, "entry.otm")}, "qualifiedaliases")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	if strings.Count(text, `"time"`) != 1 {
		t.Fatalf("time import was not canonicalized:\n%s", generated)
	}
	if strings.Contains(text, "moment.Duration") || strings.Contains(text, "duration.Duration") {
		t.Fatalf("source aliases leaked into linked Go types:\n%s", generated)
	}
	referenceSource := `package reference
import "time"
func add(value time.Duration) time.Duration { return value+time.Second }
func double(value time.Duration) time.Duration { return value*2 }
func Value() time.Duration { return double(add(time.Second)) }
`
	testSource := `package qualifiedaliases
import (
  "testing"
  reference "qualifiedaliases.test/reference"
)
func TestQualifiedAliases(t *testing.T) {
  if got, want := value(), reference.Value(); got != want { t.Errorf("value = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "qualifiedaliases.test", generated, referenceSource, testSource)
}
