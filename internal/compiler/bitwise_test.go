package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBitwiseAndShiftCompileAndRunMatrix(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "bitwise.otm")
	input := `
import go os from "os";
import go bits from "math/bits";

function precedence(): int { return 1 | 2 ^ 3 + 4 << 1 &^ 2 * 3; }
function operations(value: int, mask: int): int { return (value & mask) + (value | mask) + (value ^ mask) + (value &^ mask); }
function complementByte(value: byte): byte { return ^value; }
function signedRight(value: int32): int32 { return value >> 2; }
function shifts(value: int, amount: int): int { return value << amount >> amount; }
function hugeRight(value: int): int { return value >> 1000000; }
function flags(): int { const mode = os.ModeDir | os.ModePerm; return int(mode & os.ModePerm); }
function unsignedAPI(): int { const rotated = bits.RotateLeft(1, 8); return int(rotated | 3); }
function addressAndMask(value: int): int { let copy = value; const pointer = &copy; return *pointer & 7; }
function runtimeNegative(amount: int): int { return 1 << amount; }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "bitwisematrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{"1 | 2 ^ 3 + 4<<1&^2*3", "value&mask", "value | mask", "value ^ mask", "value&^mask", "^value", "value >> 2", "value << amount >> amount", "os.ModeDir | os.ModePerm", "bits.RotateLeft(1, 8)", "rotated | 3", "&copy", "*pointer & 7"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "math/bits"
  "os"
)
func Precedence() int { return 1 | 2 ^ 3 + 4<<1 &^ 2*3 }
func Operations(value, mask int) int { return (value & mask) + (value | mask) + (value ^ mask) + (value &^ mask) }
func ComplementByte(value byte) byte { return ^value }
func SignedRight(value int32) int32 { return value >> 2 }
func Shifts(value, amount int) int { return value << amount >> amount }
func HugeRight(value int) int { return value >> 1000000 }
func Flags() int { mode := os.ModeDir | os.ModePerm; return int(mode & os.ModePerm) }
func UnsignedAPI() int { rotated := bits.RotateLeft(1, 8); return int(rotated | 3) }
func AddressAndMask(value int) int { copy := value; pointer := &copy; return *pointer & 7 }
func RuntimeNegative(amount int) int { return 1 << amount }
`
	testSource := `package bitwisematrix
import (
  "testing"
  reference "bitwise.test/reference"
)
func didPanic(call func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  call()
  return false
}
func TestBitwiseMatrix(t *testing.T) {
  if got, want := precedence(), reference.Precedence(); got != want { t.Errorf("precedence = %d, Go = %d", got, want) }
  for _, pair := range [][2]int{{0, 0}, {10, 12}, {-1, 7}, {31, 5}, {-12, -5}} {
    if got, want := operations(pair[0], pair[1]), reference.Operations(pair[0], pair[1]); got != want { t.Errorf("operations(%d, %d) = %d, Go = %d", pair[0], pair[1], got, want) }
  }
  for _, value := range []byte{0, 1, 127, 128, 255} {
    if got, want := complementByte(value), reference.ComplementByte(value); got != want { t.Errorf("complementByte(%d) = %d, Go = %d", value, got, want) }
  }
  for _, value := range []int32{-9, -8, -1, 0, 8, 9} {
    if got, want := signedRight(value), reference.SignedRight(value); got != want { t.Errorf("signedRight(%d) = %d, Go = %d", value, got, want) }
  }
  for _, item := range [][2]int{{0, 0}, {13, 0}, {13, 1}, {13, 3}, {-13, 2}} {
    if got, want := shifts(item[0], item[1]), reference.Shifts(item[0], item[1]); got != want { t.Errorf("shifts(%d, %d) = %d, Go = %d", item[0], item[1], got, want) }
  }
  for _, value := range []int{-1, 0, 1, 99} {
    if got, want := hugeRight(value), reference.HugeRight(value); got != want { t.Errorf("hugeRight(%d) = %d, Go = %d", value, got, want) }
  }
  if got, want := flags(), reference.Flags(); got != want { t.Errorf("flags = %d, Go = %d", got, want) }
  if got, want := unsignedAPI(), reference.UnsignedAPI(); got != want { t.Errorf("unsignedAPI = %d, Go = %d", got, want) }
  for _, value := range []int{-1, 0, 7, 8, 14, 255} {
    if got, want := addressAndMask(value), reference.AddressAndMask(value); got != want { t.Errorf("addressAndMask(%d) = %d, Go = %d", value, got, want) }
  }
  for _, amount := range []int{0, 1, 5} {
    if got, want := runtimeNegative(amount), reference.RuntimeNegative(amount); got != want { t.Errorf("runtimeNegative(%d) = %d, Go = %d", amount, got, want) }
  }
}
func TestDynamicShiftPanicCompatibility(t *testing.T) {
  got := didPanic(func() { runtimeNegative(-1) })
  want := didPanic(func() { reference.RuntimeNegative(-1) })
  if got != want || !got { t.Errorf("negative shift panic = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "bitwise.test", generated, referenceSource, testSource)
}
