package sema

import (
	"strings"
	"testing"
)

func TestSliceExpressionSemanticSuccessMatrix(t *testing.T) {
	tests := []struct {
		name, source string
	}{
		{"slice bounds and chaining", `function use(values: int[], low: int32, high: int64): int { const all: int[] = values[:]; const prefix = values[:high]; const suffix = values[low:]; const middle = values[low:high]; const full = values[low:high:high]; return full[0] + prefix[0] + suffix[0] + middle[0] + all[0]; }`},
		{"fixed array and pointer", `function use(pointer: *[4]int): int { let value: [4]int = [1, 2, 3, 4]; const middle: int[] = value[1:3]; const pointed: int[] = pointer[:2]; return middle[0] + pointed[0]; }`},
		{"string byte offsets", `function use(value: string): string { return value[1:3]; }`},
		{"index addressability", `function slicePointer(values: int[]): *int { return &values[0]; } function arrayPointer(values: *[2]int): *int { return &values[0]; }`},
		{"named slice and index", `import go net from "net"; function tail(value: net.IP): net.IP { const first: byte = value[0]; return value[1:]; }`},
		{"named map", `import go http from "net/http"; function first(value: http.Header): string { return value["X-Test"][0]; }`},
		{"named string", `import go template from "html/template"; function middle(value: template.HTML): template.HTML { const first: byte = value[0]; return value[1:3]; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestSliceExpressionSemanticFailureMatrix(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"noncollection", `function bad(value: int): int { return value[0:1]; }`, "cannot be sliced"},
		{"map", `function bad(value: Map<string, int>): void { const sliced = value[0:1]; }`, "cannot be sliced"},
		{"pointer not array", `function bad(value: *int): void { const sliced = value[:]; }`, "cannot be sliced"},
		{"low type", `function bad(value: int[]): int[] { return value["x":]; }`, "slice low bound must be an integer"},
		{"high type", `function bad(value: int[]): int[] { return value[:false]; }`, "slice high bound must be an integer"},
		{"max type", `function bad(value: int[]): int[] { return value[0:1:1.5]; }`, "slice max bound must be an integer"},
		{"negative low", `function bad(value: int[]): int[] { return value[-1:]; }`, "slice low bound cannot be negative"},
		{"negative high", `function bad(value: int[]): int[] { return value[:-1]; }`, "slice high bound cannot be negative"},
		{"negative max", `function bad(value: int[]): int[] { return value[0:1:-1]; }`, "slice max bound cannot be negative"},
		{"low exceeds high", `function bad(value: int[]): int[] { return value[2:1]; }`, "low exceeds high"},
		{"high exceeds max", `function bad(value: int[]): int[] { return value[0:3:2]; }`, "high exceeds max"},
		{"fixed low exceeds length", `function bad(value: [3]int): int[] { return value[4:]; }`, "low bound 4 exceeds fixed array length 3"},
		{"fixed high exceeds length", `function bad(value: [3]int): int[] { return value[:4]; }`, "high bound 4 exceeds fixed array length 3"},
		{"fixed max exceeds length", `function bad(value: [3]int): int[] { return value[0:2:4]; }`, "max bound 4 exceeds fixed array length 3"},
		{"fixed array temporary", `function make(): [3]int { return [1, 2, 3]; } function bad(): int[] { return make()[:]; }`, "requires an addressable operand"},
		{"string full slice", `function bad(value: string): string { return value[0:1:2]; }`, "3-index slice cannot be used with string"},
		{"oversized bound", `function bad(value: int[]): int[] { return value[:9223372036854775808]; }`, "slice high bound is out of range"},
		{"map index address", `function bad(value: Map<string, int>): *int { return &value["x"]; }`, "addressable operand"},
		{"string index address", `function bad(value: string): *byte { return &value[0]; }`, "addressable operand"},
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
