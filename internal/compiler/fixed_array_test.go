package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixedArraysCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "fixed_array.km")
	dependency := filepath.Join(temp, "dependency.km")
	input := `
import go sha256 from "crypto/sha256";
import go netip from "net/netip";
import { pair } from "./dependency";

function digest(values: byte[]): [32]byte { return sha256.Sum256(values); }
function address(value: [4]byte): [4]byte { return netip.AddrFrom4(value).As4(); }
function literal(): [3]int { return [1, 2, 3]; }
function copied(): int {
  let original: [2]int = [1, 2];
  let duplicate: [2]int = original;
  duplicate[0] = 9;
  return original[0] * 10 + duplicate[0];
}
function compared(): boolean {
  const left: [2]string = ["a", "b"];
  const same: [2]string = ["a", "b"];
  const different: [2]string = ["a", "c"];
  return left === same && left !== different && ["a", "b"] === left;
}
function empty(): [0]int { return []; }
function nested(): int {
  const values: [2][2]int = [[1, 2], [3, 4]];
  return values[0][1] * 10 + values[1][0];
}
function lookup(values: Map<[2]byte, string>, key: [2]byte): string { return values[key]; }
function imported(): [2]int { return pair(); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependency, []byte(`function pair(): [2]int { return [4, 2]; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "fixedarray")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"[32]byte", "[3]int{1, 2, 3}", "[0]int{}", "[2][2]int", "map[[2]byte]string"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "crypto/sha256"
  "net/netip"
)
func Digest(values []byte) [32]byte { return sha256.Sum256(values) }
func Address(value [4]byte) [4]byte { return netip.AddrFrom4(value).As4() }
func Literal() [3]int { return [3]int{1, 2, 3} }
func Copied() int {
  original := [2]int{1, 2}
  duplicate := original
  duplicate[0] = 9
  return original[0] * 10 + duplicate[0]
}
func Compared() bool {
  left := [2]string{"a", "b"}
  same := [2]string{"a", "b"}
  different := [2]string{"a", "c"}
  return left == same && left != different && [2]string{"a", "b"} == left
}
func Empty() [0]int { return [0]int{} }
func Nested() int {
  values := [2][2]int{{1, 2}, {3, 4}}
  return values[0][1] * 10 + values[1][0]
}
func Lookup(values map[[2]byte]string, key [2]byte) string { return values[key] }
func Imported() [2]int { return [2]int{4, 2} }
`
	testSource := `package fixedarray
import (
  "testing"
  reference "fixedarray.test/reference"
)
func TestFixedArrays(t *testing.T) {
  for _, input := range [][]byte{[]byte("abc"), {}, nil, []byte("温泉卵")} {
    if got, want := digest(input), reference.Digest(input); got != want { t.Errorf("digest(%q) = %x, Go = %x", input, got, want) }
  }
  for _, input := range [][4]byte{{127, 0, 0, 1}, {0, 0, 0, 0}, {255, 255, 255, 255}} {
    if got, want := address(input), reference.Address(input); got != want { t.Errorf("address(%v) = %v, Go = %v", input, got, want) }
  }
  if got, want := literal(), reference.Literal(); got != want { t.Errorf("literal = %v, Go = %v", got, want) }
  if got, want := copied(), reference.Copied(); got != want { t.Errorf("copied = %d, Go = %d", got, want) }
  if got, want := compared(), reference.Compared(); got != want { t.Errorf("compared = %v, Go = %v", got, want) }
  if got, want := empty(), reference.Empty(); got != want { t.Errorf("empty = %v, Go = %v", got, want) }
  if got, want := nested(), reference.Nested(); got != want { t.Errorf("nested = %d, Go = %d", got, want) }
  for _, key := range [][2]byte{{1, 2}, {0, 0}} {
    values := map[[2]byte]string{{1, 2}: "ok"}
    if got, want := lookup(values, key), reference.Lookup(values, key); got != want { t.Errorf("lookup(%v) = %q, Go = %q", key, got, want) }
  }
  if got, want := imported(), reference.Imported(); got != want { t.Errorf("imported = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "fixedarray.test", generated, referenceSource, testSource)
}
