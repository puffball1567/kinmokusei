package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnsignedIntegerCompileAndGoDifferentialMatrix(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "unsigned.km")
	source := `
import go bits from "math/bits";

type Word = distinct uint;

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
function maximumMachine(): uint { return ^uint(0); }
function arithmeticMachine(value: uint, multiplier: uint, divisor: uint): uint { return (value * multiplier + uint(7)) / divisor; }
function bitwiseMachine(value: uint, mask: uint): uint { return (^value & mask) | (value &^ mask); }
function shiftsMachine(value: uint, amount: byte): uint { return value << amount >> amount; }
function updateMachine(value: uint): uint { value += uint(3); value--; value <<= 1; return value; }
function convertMachine(value: uint64): uint { return uint(value); }
function aliasByte(value: uint8): byte { return value; }
function genericIdentity<T>(value: T): T { return value; }
function genericMachine(value: uint): uint { return genericIdentity<uint>(value); }
function definedMachine(value: Word, delta: Word): uint { const total: Word = value + delta; return uint(total); }
function aliasCollection(values: uint8[], lookup: Map<uint8, int32>, key: byte): int32 {
  let total: int32 = 0;
  for (const value of values) { total += int32(value); }
  const [value, present] = lookup[key];
  if (present) { total += value; }
  return total;
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "unsignedmatrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{"type Word uint", "func maximum64() uint64", "func maximumMachine() uint", "uint16(value)", "uint(value)", "bits.RotateLeft64(value, amount)", "genericIdentity[uint](value)", "value += 2", "value--", "value <<= 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated unsigned Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import "math/bits"
type Word uint
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
func MaximumMachine() uint { return ^uint(0) }
func ArithmeticMachine(value, multiplier, divisor uint) uint { return (value*multiplier + uint(7))/divisor }
func BitwiseMachine(value, mask uint) uint { return (^value & mask) | (value &^ mask) }
func ShiftsMachine(value uint, amount byte) uint { return value << amount >> amount }
func UpdateMachine(value uint) uint { value += uint(3); value--; value <<= 1; return value }
func ConvertMachine(value uint64) uint { return uint(value) }
func AliasByte(value uint8) byte { return value }
func genericIdentity[T any](value T) T { return value }
func GenericMachine(value uint) uint { return genericIdentity[uint](value) }
func DefinedMachine(value, delta Word) uint { total := value + delta; return uint(total) }
func AliasCollection(values []uint8, lookup map[uint8]int32, key byte) int32 { total := int32(0); for _, value := range values { total += int32(value) }; value, present := lookup[key]; if present { total += value }; return total }
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
  if got, want := maximumMachine(), reference.MaximumMachine(); got != want { t.Errorf("maximumMachine = %d, Go = %d", got, want) }
  machineValues := []uint{0, 1, 2, 127, 128, ^uint(0)-1, ^uint(0)}
  for _, value := range machineValues {
    if got, want := updateMachine(value), reference.UpdateMachine(value); got != want { t.Errorf("updateMachine(%d) = %d, Go = %d", value, got, want) }
    if got, want := genericMachine(value), reference.GenericMachine(value); got != want { t.Errorf("genericMachine(%d) = %d, Go = %d", value, got, want) }
    for _, mask := range []uint{0, 1, 0x55, ^uint(0)} {
      if got, want := bitwiseMachine(value, mask), reference.BitwiseMachine(value, mask); got != want { t.Errorf("bitwiseMachine(%d,%d) = %d, Go = %d", value, mask, got, want) }
    }
    for _, amount := range []byte{0, 1, 7, 31, 63, 64, 127} {
      if got, want := shiftsMachine(value, amount), reference.ShiftsMachine(value, amount); got != want { t.Errorf("shiftsMachine(%d,%d) = %d, Go = %d", value, amount, got, want) }
    }
  }
  for _, item := range [][3]uint{{0, 1, 1}, {1, 3, 2}, {42, 7, 5}, {^uint(0), 3, 7}} {
    if got, want := arithmeticMachine(item[0], item[1], item[2]), reference.ArithmeticMachine(item[0], item[1], item[2]); got != want { t.Errorf("arithmeticMachine(%v) = %d, Go = %d", item, got, want) }
  }
  for _, item := range [][2]uint{{0, 0}, {1, 2}, {^uint(0), 1}} {
    if got, want := definedMachine(Word(item[0]), Word(item[1])), reference.DefinedMachine(reference.Word(item[0]), reference.Word(item[1])); got != want { t.Errorf("definedMachine(%v) = %d, Go = %d", item, got, want) }
  }
  for _, value := range []uint64{0, 1, 1<<32 - 1, 1<<32, ^uint64(0)} {
    if got, want := convertMachine(value), reference.ConvertMachine(value); got != want { t.Errorf("convertMachine(%d) = %d, Go = %d", value, got, want) }
  }
  for _, value := range []uint8{0, 1, 127, 255} {
    if got, want := aliasByte(value), reference.AliasByte(value); got != want { t.Errorf("aliasByte(%d) = %d, Go = %d", value, got, want) }
  }
  collectionCases := []struct { values []uint8; lookup map[uint8]int32; key byte }{
    {nil, nil, 0}, {[]uint8{0, 1, 255}, map[uint8]int32{0: 65, 255: 30028}, 255}, {[]uint8{2, 3}, map[uint8]int32{7: 0}, 7},
  }
  for _, item := range collectionCases {
    if got, want := aliasCollection(item.values, item.lookup, item.key), reference.AliasCollection(item.values, item.lookup, item.key); got != want { t.Errorf("aliasCollection = %d, Go = %d", got, want) }
  }
}
func TestUnsignedDivisionPanicCompatibility(t *testing.T) {
  got := didPanic(func() { arithmetic64(1, 0) })
  want := didPanic(func() { reference.Arithmetic64(1, 0) })
  if got != want || !got { t.Errorf("zero divisor panic = %v, Go = %v", got, want) }
  got = didPanic(func() { arithmeticMachine(1, 1, 0) })
  want = didPanic(func() { reference.ArithmeticMachine(1, 1, 0) })
  if got != want || !got { t.Errorf("machine zero divisor panic = %v, Go = %v", got, want) }
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
		{"negative uint", `function value(): uint { return -1; }`, "cannot be represented as uint"},
		{"negative uint conversion", `function value(): uint { return uint(-1); }`, "cannot be represented as uint"},
		{"overflow uint64", `function value(): uint64 { return 18446744073709551616; }`, "cannot be represented as uint64"},
		{"overflow uint16", `function value(): uint16 { return 65536; }`, "cannot be represented as uint16"},
		{"conversion overflow", `function value(): uint16 { return uint16(65536); }`, "cannot be represented as uint16"},
		{"mixed signed", `function value(left: uint64, right: int64): uint64 { return left + right; }`, "cannot mix uint64 and int64"},
		{"implicit unsigned widening", `function value(input: uint32): uint64 { return input; }`, "cannot use uint32 as uint64"},
		{"mixed machine signed", `function value(left: uint, right: int): uint { return left + right; }`, "cannot mix uint and int"},
		{"implicit fixed to machine", `function value(input: uint32): uint { return input; }`, "cannot use uint32 as uint"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.km")
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
