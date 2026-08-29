package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnsignedIntegerCompileAndGoDifferentialMatrix(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "unsigned.otm")
	source := `
import go bits from "math/bits";

function maximum16(): uint16 { return 65535; }
function maximum32(): uint32 { return 4294967295; }
function maximum64(): uint64 { return 18446744073709551615; }
function add16(value: uint16, delta: uint16): uint16 { return value + delta; }
function wrap32(value: uint32): uint32 { return value + 1; }
function arithmetic64(value: uint64, divisor: uint64): uint64 { return (value * 3 + 7) / divisor; }
function bitwise64(value: uint64, mask: uint64): uint64 { return (^value & mask) | (value &^ mask); }
function shifts64(value: uint64, amount: uint32): uint64 { return value << amount >> amount; }
function rotate64(value: uint64, amount: int): uint64 { return bits.RotateLeft64(value, amount); }
function convert(value: uint64): uint16 { return uint16(value); }
function update(value: uint64): uint64 { value += 2; value--; value <<= 1; return value; }
function ordered(left: uint32, right: uint32): boolean { return left < right; }
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "unsignedmatrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{"func maximum64() uint64", "uint16(value)", "bits.RotateLeft64(value, amount)", "value += 2", "value--", "value <<= 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated unsigned Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import "math/bits"
func Maximum16() uint16 { return 65535 }
func Maximum32() uint32 { return 4294967295 }
func Maximum64() uint64 { return 18446744073709551615 }
func Add16(value, delta uint16) uint16 { return value + delta }
func Wrap32(value uint32) uint32 { return value + 1 }
func Arithmetic64(value, divisor uint64) uint64 { return (value*3 + 7) / divisor }
func Bitwise64(value, mask uint64) uint64 { return (^value & mask) | (value &^ mask) }
func Shifts64(value uint64, amount uint32) uint64 { return value << amount >> amount }
func Rotate64(value uint64, amount int) uint64 { return bits.RotateLeft64(value, amount) }
func Convert(value uint64) uint16 { return uint16(value) }
func Update(value uint64) uint64 { value += 2; value--; value <<= 1; return value }
func Ordered(left, right uint32) bool { return left < right }
`
	testSource := `package unsignedmatrix
import (
  "testing"
  reference "unsigned.test/reference"
)
func didPanic(call func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  call()
  return false
}
func TestUnsignedMatrix(t *testing.T) {
  if got, want := maximum16(), reference.Maximum16(); got != want { t.Errorf("maximum16 = %d, Go = %d", got, want) }
  if got, want := maximum32(), reference.Maximum32(); got != want { t.Errorf("maximum32 = %d, Go = %d", got, want) }
  if got, want := maximum64(), reference.Maximum64(); got != want { t.Errorf("maximum64 = %d, Go = %d", got, want) }
  for _, item := range [][2]uint16{{0, 0}, {1, 2}, {65535, 1}, {65000, 1000}} {
    if got, want := add16(item[0], item[1]), reference.Add16(item[0], item[1]); got != want { t.Errorf("add16(%d,%d) = %d, Go = %d", item[0], item[1], got, want) }
  }
  for _, value := range []uint32{0, 1, 4294967294, 4294967295} {
    if got, want := wrap32(value), reference.Wrap32(value); got != want { t.Errorf("wrap32(%d) = %d, Go = %d", value, got, want) }
  }
  for _, item := range [][2]uint64{{0, 1}, {1, 1}, {42, 7}, {1<<63, 3}, {^uint64(0), 5}} {
    if got, want := arithmetic64(item[0], item[1]), reference.Arithmetic64(item[0], item[1]); got != want { t.Errorf("arithmetic64(%d,%d) = %d, Go = %d", item[0], item[1], got, want) }
  }
  for _, item := range [][2]uint64{{0, 0}, {1, 3}, {0xff00, 0xffff}, {^uint64(0), 0xaaaaaaaaaaaaaaaa}} {
    if got, want := bitwise64(item[0], item[1]), reference.Bitwise64(item[0], item[1]); got != want { t.Errorf("bitwise64(%d,%d) = %d, Go = %d", item[0], item[1], got, want) }
  }
  for _, amount := range []uint32{0, 1, 31, 63, 64, 127} {
    if got, want := shifts64(^uint64(0), amount), reference.Shifts64(^uint64(0), amount); got != want { t.Errorf("shifts64(%d) = %d, Go = %d", amount, got, want) }
  }
  for _, amount := range []int{-65, -1, 0, 1, 31, 64, 65} {
    if got, want := rotate64(0x123456789abcdef0, amount), reference.Rotate64(0x123456789abcdef0, amount); got != want { t.Errorf("rotate64(%d) = %d, Go = %d", amount, got, want) }
  }
  for _, value := range []uint64{0, 1, 65535, 65536, ^uint64(0)} {
    if got, want := convert(value), reference.Convert(value); got != want { t.Errorf("convert(%d) = %d, Go = %d", value, got, want) }
    if got, want := update(value), reference.Update(value); got != want { t.Errorf("update(%d) = %d, Go = %d", value, got, want) }
  }
  for _, item := range [][2]uint32{{0,0}, {0,1}, {1,0}, {4294967295,4294967295}} {
    if got, want := ordered(item[0], item[1]), reference.Ordered(item[0], item[1]); got != want { t.Errorf("ordered(%d,%d) = %v, Go = %v", item[0], item[1], got, want) }
  }
}
func TestUnsignedDivisionPanicCompatibility(t *testing.T) {
  got := didPanic(func() { arithmetic64(1, 0) })
  want := didPanic(func() { reference.Arithmetic64(1, 0) })
  if got != want || !got { t.Errorf("zero divisor panic = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, root, "unsigned.test", generated, referenceSource, testSource)
}

func TestUnsignedIntegerRejectsInvalidMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"negative uint64", `function value(): uint64 { return -1; }`, "cannot be represented as uint64"},
		{"overflow uint64", `function value(): uint64 { return 18446744073709551616; }`, "cannot be represented as uint64"},
		{"overflow uint16", `function value(): uint16 { return 65536; }`, "cannot be represented as uint16"},
		{"conversion overflow", `function value(): uint16 { return uint16(65536); }`, "cannot be represented as uint16"},
		{"mixed signed", `function value(left: uint64, right: int64): uint64 { return left + right; }`, "cannot mix uint64 and int64"},
		{"implicit unsigned widening", `function value(input: uint32): uint64 { return input; }`, "cannot use uint32 as uint64"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.otm")
			if err := os.WriteFile(path, []byte(test.source), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := CheckFiles([]string{path})
			if err != nil {
				t.Fatal(err)
			}
			messages := make([]string, len(result.Diagnostics))
			for index, item := range result.Diagnostics {
				messages[index] = item.Message
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", messages, test.want)
			}
		})
	}
}
