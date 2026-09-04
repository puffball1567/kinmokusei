package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSliceToArrayConversionsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "array_conversion.km")
	input := `
import go net from "net";
import go crc32 from "hash/crc32";

function copyBehavior(): int {
  let values = [1, 2, 3];
  let copied = copyArray[[3]int](values);
  copied[0] = 9;
  return values[0] * 10 + copied[0];
}
function viewBehavior(): int {
  let values = [1, 2, 3];
  let viewed = viewArray[[3]int](values);
  viewed[0] = 9;
  return values[0] * 10 + viewed[0];
}
function zeroCopy(values: byte[]): [0]byte { return copyArray[[0]byte](values); }
function zeroView(values: byte[]): *[0]byte { return viewArray[[0]byte](values); }
function copyIP(values: net.IP): [4]byte { return copyArray[[4]byte](values); }
function copyTable(value: *crc32.Table): crc32.Table { return copyArray[crc32.Table](value[:]); }
function viewTable(value: *crc32.Table): *crc32.Table { return viewArray[crc32.Table](value[:]); }
function shortCopy(values: int[]): [3]int { return copyArray[[3]int](values); }
function shortView(values: int[]): *[3]int { return viewArray[[3]int](values); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "arrayconversion")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"[3]int(values)", "(*[3]int)(values)", "[0]byte(values)", "(*[0]byte)(values)", "[4]byte(values)", "crc32.Table(value[:])", "(*crc32.Table)(value[:])"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "hash/crc32"
  "net"
)
func CopyBehavior() int {
  values := []int{1, 2, 3}
  copied := [3]int(values)
  copied[0] = 9
  return values[0] * 10 + copied[0]
}
func ViewBehavior() int {
  values := []int{1, 2, 3}
  viewed := (*[3]int)(values)
  viewed[0] = 9
  return values[0] * 10 + viewed[0]
}
func ZeroCopy(values []byte) [0]byte { return [0]byte(values) }
func ZeroView(values []byte) *[0]byte { return (*[0]byte)(values) }
func CopyIP(values net.IP) [4]byte { return [4]byte(values) }
func CopyTable(value *crc32.Table) crc32.Table { return crc32.Table(value[:]) }
func ViewTable(value *crc32.Table) *crc32.Table { return (*crc32.Table)(value[:]) }
func ShortCopy(values []int) [3]int { return [3]int(values) }
func ShortView(values []int) *[3]int { return (*[3]int)(values) }
`
	testSource := `package arrayconversion
import (
  "hash/crc32"
  "net"
  "testing"
  reference "arrayconversion.test/reference"
)
func didPanic(operation func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  operation()
  return false
}
func TestArrayConversions(t *testing.T) {
  if got, want := copyBehavior(), reference.CopyBehavior(); got != want { t.Errorf("copyBehavior = %d, Go = %d", got, want) }
  if got, want := viewBehavior(), reference.ViewBehavior(); got != want { t.Errorf("viewBehavior = %d, Go = %d", got, want) }
  if got, want := zeroCopy(nil), reference.ZeroCopy(nil); got != want { t.Errorf("zeroCopy = %v, Go = %v", got, want) }
  if got, want := zeroView(nil), reference.ZeroView(nil); (got == nil) != (want == nil) { t.Errorf("zeroView nil = %v, Go = %v", got, want) }
  for _, ip := range []net.IP{{127, 0, 0, 1}, {0, 0, 0, 0}} {
    if got, want := copyIP(ip), reference.CopyIP(ip); got != want { t.Errorf("copyIP(%v) = %v, Go = %v", ip, got, want) }
  }
  gotTable, wantTable := crc32.MakeTable(crc32.IEEE), crc32.MakeTable(crc32.IEEE)
  gotCopy, wantCopy := copyTable(gotTable), reference.CopyTable(wantTable)
  gotCopy[0], wantCopy[0] = 0, 0
  if *gotTable != *wantTable || gotCopy != wantCopy { t.Errorf("copyTable source/copy differs from Go") }
  gotView, wantView := viewTable(gotTable), reference.ViewTable(wantTable)
  gotView[0], wantView[0] = 0, 0
  if *gotTable != *wantTable || *gotView != *wantView { t.Errorf("viewTable alias differs from Go") }
  for name, operations := range map[string][2]func(){
    "copy": {func() { shortCopy([]int{1, 2}) }, func() { reference.ShortCopy([]int{1, 2}) }},
    "view": {func() { shortView([]int{1, 2}) }, func() { reference.ShortView([]int{1, 2}) }},
  } {
    if got, want := didPanic(operations[0]), didPanic(operations[1]); got != want { t.Errorf("short %s panic = %v, Go = %v", name, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "arrayconversion.test", generated, referenceSource, testSource)
}
