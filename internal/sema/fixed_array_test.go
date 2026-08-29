package sema

import (
	"strings"
	"testing"
)

func TestFixedArraySemanticSuccessMatrix(t *testing.T) {
	tests := []struct {
		name, source string
	}{
		{"literal parameter and return", `function copy(value: [3]int): [3]int { let result: [3]int = value; return result; } function use(): int { return copy([1, 2, 3])[1]; }`},
		{"zero length", `function empty(): [0]int { const value: [0]int = []; return value; }`},
		{"nested literal and index", `function nested(): int { const value: [2][2]int = [[1, 2], [3, 4]]; return value[1][0]; }`},
		{"comparable", `function equal(left: [2]string, right: [2]string): boolean { return left === right && left !== ["x", "y"] && ["x", "y"] !== left; }`},
		{"comparable map key", `function lookup(values: Map<[2]byte, string>, key: [2]byte): string { return values[key]; }`},
		{"Go fixed array result", `import go sha256 from "crypto/sha256"; function digest(values: byte[]): [32]byte { return sha256.Sum256(values); }`},
		{"Go fixed array argument and result", `import go netip from "net/netip"; function roundTrip(value: [4]byte): [4]byte { return netip.AddrFrom4(value).As4(); } function literal(): [4]byte { return netip.AddrFrom4([127, 0, 0, 1]).As4(); }`},
		{"fixed array of slices", `function first(value: [2]int[]): int { return value[0][0]; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestFixedArraySemanticFailureMatrix(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"literal too few", `function bad(): [3]int { return [1, 2]; }`, "fixed array literal has 2 elements, expected 3"},
		{"literal too many", `function bad(): [1]int { return [1, 2]; }`, "fixed array literal has 2 elements, expected 1"},
		{"zero length nonempty", `function bad(): [0]int { return [1]; }`, "fixed array literal has 1 elements, expected 0"},
		{"element mismatch", `function bad(): [2]int { return [1, "wrong"]; }`, "cannot use string as int"},
		{"different lengths", `function bad(value: [2]int): [3]int { return value; }`, "cannot use [2]int as [3]int"},
		{"slice to fixed", `function bad(value: int[]): [2]int { return value; }`, "cannot use int[] as [2]int"},
		{"fixed to slice", `function bad(value: [2]int): int[] { return value; }`, "cannot use [2]int as int[]"},
		{"nil", `function bad(): [2]int { return nil; }`, "cannot use nil as [2]int"},
		{"index type", `function bad(value: [2]int): int { return value["x"]; }`, "array index must be an integer"},
		{"noncomparable map key", `function bad(value: Map<[2]int[], string>): void {}`, "cannot be used as a Map key"},
		{"void element", `function bad(value: [2]void): void {}`, "cannot be used as a fixed array element"},
		{"nested length mismatch", `function bad(): [2][1]int { return [[1], [2, 3]]; }`, "fixed array literal has 2 elements, expected 1"},
		{"Go argument literal length mismatch", `import go netip from "net/netip"; function bad(): void { netip.AddrFrom4([127, 0, 1]); }`, "fixed array literal has 3 elements, expected 4"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			found := false
			for _, message := range diagnostics {
				if strings.Contains(message, test.want) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
