package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignedNarrowIntegersMatchIndependentGo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "signed_narrow.km")
	source := `
type Tiny = distinct int8;
type Small = distinct int16;

function minimum8(): int8 { return -128; }
function maximum8(): int8 { return 127; }
function minimum16(): int16 { return -32768; }
function maximum16(): int16 { return 32767; }
function wrap8(value: int8): int8 { return value + int8(1); }
function wrap16(value: int16): int16 { return value - int16(1); }
function arithmetic(value: int16, multiplier: int16, divisor: int16): int16 { return (value * multiplier + int16(7)) / divisor; }
function bitwise(value: int8, mask: int8): int8 { return (^value & mask) | (value &^ mask); }
function shifts(value: int16, amount: byte): int16 { return value << amount >> amount; }
function update(value: int8): int8 { value += int8(3); value--; value <<= 1; return value; }
function ordered(left: int16, right: int16): boolean { return left < right; }
function convert(value: int16): int8 { return int8(value); }
function defined(value: Tiny, delta: Tiny): int8 { const total: Tiny = value + delta; return int8(total); }
function genericIdentity<T>(value: T): T { return value; }
function generic(value: int16): int16 { return genericIdentity<int16>(value); }
function collections(values: int8[], lookup: Map<int16, int8>, key: int16): int {
  let total = 0;
  for (const value of values) { total += int(value); }
  const [value, present] = lookup[key];
  if (present) { total += int(value); }
  return total;
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "signednarrow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"type Tiny int8", "type Small int16", "func minimum8() int8", "func maximum16() int16", "int8(value)", "genericIdentity[int16](value)"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
type Tiny int8
type Small int16
func Minimum8() int8 { return -128 }
func Maximum8() int8 { return 127 }
func Minimum16() int16 { return -32768 }
func Maximum16() int16 { return 32767 }
func Wrap8(value int8) int8 { return value + int8(1) }
func Wrap16(value int16) int16 { return value - int16(1) }
func Arithmetic(value, multiplier, divisor int16) int16 { return (value*multiplier + int16(7))/divisor }
func Bitwise(value, mask int8) int8 { return (^value & mask) | (value &^ mask) }
func Shifts(value int16, amount byte) int16 { return value << amount >> amount }
func Update(value int8) int8 { value += int8(3); value--; value <<= 1; return value }
func Ordered(left, right int16) bool { return left < right }
func Convert(value int16) int8 { return int8(value) }
func Defined(value, delta Tiny) int8 { total := value + delta; return int8(total) }
func genericIdentity[T any](value T) T { return value }
func Generic(value int16) int16 { return genericIdentity[int16](value) }
func Collections(values []int8, lookup map[int16]int8, key int16) int { total := 0; for _, value := range values { total += int(value) }; value, present := lookup[key]; if present { total += int(value) }; return total }
`
	testSource := `package signednarrow
import (
  "testing"
  reference "signednarrow.test/reference"
)
func didPanic(call func()) (panicked bool) { defer func() { panicked = recover() != nil }(); call(); return false }
func TestSignedNarrowMatrix(t *testing.T) {
  if got, want := minimum8(), reference.Minimum8(); got != want { t.Errorf("minimum8=%d Go=%d", got, want) }
  if got, want := maximum8(), reference.Maximum8(); got != want { t.Errorf("maximum8=%d Go=%d", got, want) }
  if got, want := minimum16(), reference.Minimum16(); got != want { t.Errorf("minimum16=%d Go=%d", got, want) }
  if got, want := maximum16(), reference.Maximum16(); got != want { t.Errorf("maximum16=%d Go=%d", got, want) }
  for _, value := range []int8{-128, -127, -2, -1, 0, 1, 126, 127} {
    if got, want := wrap8(value), reference.Wrap8(value); got != want { t.Errorf("wrap8(%d)=%d Go=%d", value, got, want) }
    if got, want := update(value), reference.Update(value); got != want { t.Errorf("update(%d)=%d Go=%d", value, got, want) }
    for _, mask := range []int8{-128, -1, 0, 1, 0x55, 127} {
      if got, want := bitwise(value, mask), reference.Bitwise(value, mask); got != want { t.Errorf("bitwise(%d,%d)=%d Go=%d", value, mask, got, want) }
    }
  }
  for _, value := range []int16{-32768, -32767, -129, -1, 0, 1, 127, 128, 32766, 32767} {
    if got, want := wrap16(value), reference.Wrap16(value); got != want { t.Errorf("wrap16(%d)=%d Go=%d", value, got, want) }
    if got, want := convert(value), reference.Convert(value); got != want { t.Errorf("convert(%d)=%d Go=%d", value, got, want) }
    if got, want := generic(value), reference.Generic(value); got != want { t.Errorf("generic(%d)=%d Go=%d", value, got, want) }
    for _, amount := range []byte{0, 1, 7, 15, 16, 31} {
      if got, want := shifts(value, amount), reference.Shifts(value, amount); got != want { t.Errorf("shifts(%d,%d)=%d Go=%d", value, amount, got, want) }
    }
  }
  for _, item := range [][2]int16{{-32768,-32768}, {-1,0}, {0,0}, {1,-1}, {32767,32767}} {
    if got, want := ordered(item[0], item[1]), reference.Ordered(item[0], item[1]); got != want { t.Errorf("ordered(%v)=%v Go=%v", item, got, want) }
  }
  for _, item := range [][2]int8{{-128,-1}, {-1,1}, {0,0}, {100,27}, {127,1}} {
    if got, want := defined(Tiny(item[0]), Tiny(item[1])), reference.Defined(reference.Tiny(item[0]), reference.Tiny(item[1])); got != want { t.Errorf("defined(%v)=%d Go=%d", item, got, want) }
  }
  collectionCases := []struct { values []int8; lookup map[int16]int8; key int16 }{
    {nil, nil, 0}, {[]int8{-128, 0, 127}, map[int16]int8{-32768: -1, 32767: 2}, -32768}, {[]int8{1,2,3}, map[int16]int8{7: 0}, 7},
  }
  for _, item := range collectionCases {
    if got, want := collections(item.values, item.lookup, item.key), reference.Collections(item.values, item.lookup, item.key); got != want { t.Errorf("collections=%d Go=%d", got, want) }
  }
}
func TestSignedNarrowPanicCompatibility(t *testing.T) {
  for _, divisor := range []int16{0, 1, -1} {
    gotPanic := didPanic(func() { arithmetic(-32768, 1, divisor) })
    wantPanic := didPanic(func() { reference.Arithmetic(-32768, 1, divisor) })
    if gotPanic != wantPanic { t.Errorf("panic divisor %d=%v Go=%v", divisor, gotPanic, wantPanic) }
    if !gotPanic {
      if got, want := arithmetic(-32768, 1, divisor), reference.Arithmetic(-32768, 1, divisor); got != want { t.Errorf("arithmetic divisor %d=%d Go=%d", divisor, got, want) }
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, root, "signednarrow.test", generated, referenceSource, testSource)
}

func TestSignedNarrowIntegerFailureMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"int8 positive overflow", `function value(): int8 { return 128; }`, "cannot be represented as int8"},
		{"int8 negative overflow", `function value(): int8 { return -129; }`, "cannot be represented as int8"},
		{"int16 positive overflow", `function value(): int16 { return 32768; }`, "cannot be represented as int16"},
		{"int16 negative overflow", `function value(): int16 { return -32769; }`, "cannot be represented as int16"},
		{"conversion overflow", `function value(): int8 { return int8(128); }`, "cannot be represented as int8"},
		{"mixed widths", `function value(left: int8, right: int16): int16 { return left + right; }`, "cannot mix int8 and int16"},
		{"implicit widening", `function value(input: int8): int16 { return input; }`, "cannot use int8 as int16"},
	} {
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
				t.Fatalf("diagnostics=%v want=%q", messages, test.want)
			}
		})
	}
}
