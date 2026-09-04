package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoInterfaceTypeSwitchCompilesAndRuns(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "type_switch.km")
	input := `
import go io from "io";
import go strings from "strings";
import go bytes from "bytes";
import go atomic from "sync/atomic";

function classify(value: io.Reader): int {
  switch (value) {
    case const reader as *strings.Reader { return reader.Len(); }
    case const buffer as *bytes.Buffer { return 100 + buffer.Len(); }
    case nil { return -1; }
    default { return 0; }
  }
}
function mutate(value: io.Reader): int {
  switch (value) {
    case let reader as *strings.Reader {
      reader = strings.NewReader("changed");
      return reader.Len();
    }
    default { return 0; }
  }
}
function typedNil(value: io.Reader): boolean {
  switch (value) {
    case const reader as *strings.Reader { return reader == nil; }
    case nil { return false; }
    default { return false; }
  }
}
function nilInterface(value: io.Reader): boolean {
  switch (value) {
    case nil { return true; }
    default { return false; }
  }
}
function overlappingInterfaces(value: io.Reader): int {
  switch (value) {
    case const closer as io.Closer {
      if (closer != nil) { return 1; }
      return -1;
    }
    case const reader as io.Reader {
      if (reader != nil) { return 2; }
      return -2;
    }
    default { return 0; }
  }
}
function observed(counter: *atomic.Int64, value: io.Reader): io.Reader {
  counter.Add(1);
  return value;
}
function evaluatedOnce(value: io.Reader): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  switch (observed(&counter, value)) {
    case const _ as *strings.Reader {}
    case nil {}
    default {}
  }
  return counter.Load();
}
function evaluatedOnceWithBinding(value: io.Reader): int64 {
  let counter: atomic.Int64 = atomic.Int64{};
  let length = 0;
  switch (observed(&counter, value)) {
    case const reader as *strings.Reader { length = reader.Len(); }
    default {}
  }
  return counter.Load() * 10 + int64(length);
}
function discardBinding(value: io.Reader): int {
  switch (value) {
    case const _ as *strings.Reader { return 1; }
    default { return 0; }
  }
}
function breakSwitch(value: io.Reader): int {
  let result = 0;
  switch (value) {
    case const _ as *strings.Reader { result = 7; break; }
    default { result = 8; }
  }
  return result;
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "typeswitch")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{
		":= value.(type)",
		"case *strings.Reader:",
		"case *bytes.Buffer:",
		"case nil:",
		"case io.Closer:",
		"case io.Reader:",
		"switch observed(&counter, value).(type)",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "bytes"
  "io"
  "strings"
  "sync/atomic"
)
func Classify(value io.Reader) int {
  switch typed := value.(type) {
  case *strings.Reader:
    return typed.Len()
  case *bytes.Buffer:
    return 100 + typed.Len()
  case nil:
    return -1
  default:
    return 0
  }
}
func Mutate(value io.Reader) int {
  switch reader := value.(type) {
  case *strings.Reader:
    reader = strings.NewReader("changed")
    return reader.Len()
  default:
    return 0
  }
}
func TypedNil(value io.Reader) bool {
  switch reader := value.(type) {
  case *strings.Reader:
    return reader == nil
  case nil:
    return false
  default:
    return false
  }
}
func NilInterface(value io.Reader) bool {
  switch value.(type) {
  case nil:
    return true
  default:
    return false
  }
}
func OverlappingInterfaces(value io.Reader) int {
  switch typed := value.(type) {
  case io.Closer:
    if typed != nil { return 1 }
    return -1
  case io.Reader:
    if typed != nil { return 2 }
    return -2
  default:
    return 0
  }
}
func observed(counter *atomic.Int64, value io.Reader) io.Reader {
  counter.Add(1)
  return value
}
func EvaluatedOnce(value io.Reader) int64 {
  var counter atomic.Int64
  switch observed(&counter, value).(type) {
  case *strings.Reader:
  case nil:
  default:
  }
  return counter.Load()
}
func EvaluatedOnceWithBinding(value io.Reader) int64 {
  var counter atomic.Int64
  length := 0
  switch reader := observed(&counter, value).(type) {
  case *strings.Reader:
    length = reader.Len()
  }
  return counter.Load() * 10 + int64(length)
}
func DiscardBinding(value io.Reader) int {
  switch value.(type) {
  case *strings.Reader:
    return 1
  default:
    return 0
  }
}
func BreakSwitch(value io.Reader) int {
  result := 0
  switch value.(type) {
  case *strings.Reader:
    result = 7
    break
  default:
    result = 8
  }
  return result
}
`
	testSource := `package typeswitch
import (
  "bytes"
  "io"
  "strings"
  "testing"
  reference "typeswitch.test/reference"
)
type customReader struct{}
func (customReader) Read(buffer []byte) (int, error) { return 0, io.EOF }
func TestTypeSwitch(t *testing.T) {
  for _, value := range []io.Reader{strings.NewReader("abc"), bytes.NewBufferString("abcd"), customReader{}, nil} {
    if got, want := classify(value), reference.Classify(value); got != want { t.Errorf("classify(%T) = %d, Go = %d", value, got, want) }
    if got, want := evaluatedOnce(value), reference.EvaluatedOnce(value); got != want { t.Errorf("evaluatedOnce(%T) = %d, Go = %d", value, got, want) }
    if got, want := evaluatedOnceWithBinding(value), reference.EvaluatedOnceWithBinding(value); got != want { t.Errorf("evaluatedOnceWithBinding(%T) = %d, Go = %d", value, got, want) }
    if got, want := discardBinding(value), reference.DiscardBinding(value); got != want { t.Errorf("discardBinding(%T) = %d, Go = %d", value, got, want) }
    if got, want := breakSwitch(value), reference.BreakSwitch(value); got != want { t.Errorf("breakSwitch(%T) = %d, Go = %d", value, got, want) }
    if got, want := mutate(value), reference.Mutate(value); got != want { t.Errorf("mutate(%T) = %d, Go = %d", value, got, want) }
  }
  var pointer *strings.Reader
  var typed io.Reader = pointer
  for _, value := range []io.Reader{nil, typed, strings.NewReader("x")} {
    if got, want := typedNil(value), reference.TypedNil(value); got != want { t.Errorf("typedNil(%T) = %v, Go = %v", value, got, want) }
    if got, want := nilInterface(value), reference.NilInterface(value); got != want { t.Errorf("nilInterface(%T) = %v, Go = %v", value, got, want) }
  }
  for _, value := range []io.Reader{io.NopCloser(strings.NewReader("x")), strings.NewReader("x"), customReader{}, nil} {
    if got, want := overlappingInterfaces(value), reference.OverlappingInterfaces(value); got != want { t.Errorf("overlappingInterfaces(%T) = %d, Go = %d", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "typeswitch.test", generated, referenceSource, testSource)
}
