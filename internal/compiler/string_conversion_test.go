package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoCompatibleStringConversions(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "string_conversion.otm")
	input := `
import go net from "net";

function bytesText(value: byte[]): string { return string(value); }
function runesText(value: int32[]): string { return string(value); }
function namedBytesText(value: net.IP): string { return string(value); }
function integerText(value: int32): string { return string(value); }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "stringconversion")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"string(value)"} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference
import "net"
func BytesText(value []byte) string { return string(value) }
func RunesText(value []int32) string { return string(value) }
func NamedBytesText(value net.IP) string { return string(value) }
func IntegerText(value int32) string { return string(value) }
`
	testSource := `package stringconversion
import (
  "net"
  "testing"
  reference "stringconversion.test/reference"
)
func TestConversionsMatchGo(t *testing.T) {
  byteCases := [][]byte{nil, {}, {'a', 'b', 'c'}, {0, 'a', 0}, {0xe6, 0xb8, 0xa9, 0xe6, 0xb3, 0x89}, {0xff, 0xfe, 'x'}}
  for _, value := range byteCases {
    if got, want := bytesText(value), reference.BytesText(value); got != want {
      t.Errorf("bytesText(%v) = %q, Go = %q", value, got, want)
    }
  }
  runeCases := [][]int32{nil, {}, {'a', 0x6e29, 0x6cc9}, {0, -1, 0x10ffff, 0x110000}}
  for _, value := range runeCases {
    if got, want := runesText(value), reference.RunesText(value); got != want {
      t.Errorf("runesText(%v) = %q, Go = %q", value, got, want)
    }
  }
  for _, value := range []net.IP{nil, {}, {127, 0, 0, 1}, {0xff, 0, 1}} {
    if got, want := namedBytesText(value), reference.NamedBytesText(value); got != want {
      t.Errorf("namedBytesText(%v) = %q, Go = %q", value, got, want)
    }
  }
  for _, value := range []int32{-1, 0, 'a', 0x6e29, 0x10ffff, 0x110000} {
    if got, want := integerText(value), reference.IntegerText(value); got != want {
      t.Errorf("integerText(%d) = %q, Go = %q", value, got, want)
    }
  }
}
`
	runGeneratedGoDifferentialTest(t, temp, "stringconversion.test", generated, referenceSource, testSource)
}
