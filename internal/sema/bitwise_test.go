package sema

import (
	"strings"
	"testing"
)

func TestBitwiseAndShiftValidTypeMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"all built-in integer widths", `function value(a: int, b: int32, c: int64, d: byte): int { const ai: int = a & 7 | 8 ^ 3 &^ 1; const bi: int32 = ^b << 1; const ci: int64 = c >> int32(2); const di: byte = d & byte(15); return ai + int(bi) + int(ci) + int(di); }`},
		{"untyped constant expressions", `function value(): int { const mask = ^0 & 255; const shifted = (1 | 2) << (1 + 2); return mask &^ shifted; }`},
		{"constant bindings", `const base = 1; const amount = 2; function value(): int { const mask = 7; return base << amount & mask; }`},
		{"dynamic signed shift amount", `function value(input: int, amount: int32): int { return input << amount >> amount; }`},
		{"Go named integer flags", `import go os from "os"; function value(mode: os.FileMode): os.FileMode { return mode | os.ModeDir &^ os.ModePerm; }`},
		{"Go unsigned inferred value", `import go bits from "math/bits"; function value(): int { const rotated = bits.RotateLeft(1, 8); return int(rotated | 3); }`},
		{"address and bitwise ampersands", `function value(input: int): int { let copy = input; const pointer = &copy; return *pointer & 7; }`},
		{"large dynamic right shift", `function value(input: int): int { return input >> 1000000; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestBitwiseAndShiftFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"unary float", `function value(input: float): float { return ^input; }`, "operator ^ requires an integer operand"},
		{"unary boolean", `function value(input: boolean): boolean { return ^input; }`, "operator ^ requires an integer operand"},
		{"and float left", `function value(input: float): int { return input & 1; }`, "operator & requires integer operands"},
		{"or string right", `function value(input: int): int { return input | "x"; }`, "operator | requires integer operands"},
		{"xor boolean left", `function value(input: int): int { return false ^ input; }`, "operator ^ requires integer operands"},
		{"and-not float right", `function value(input: int): int { return input &^ 1.5; }`, "operator &^ requires integer operands"},
		{"shift float left", `function value(input: float): int { return input << 1; }`, "operator << requires integer operands"},
		{"shift float right", `function value(input: int): int { return input >> 1.5; }`, "operator >> requires integer operands"},
		{"mixed built-in integer types", `function value(left: int, right: int32): int { return left & right; }`, "cannot mix int and int32"},
		{"mixed Go named integer types", `import go os from "os"; import go time from "time"; function value(left: os.FileMode, right: time.Duration): os.FileMode { return left | right; }`, "cannot mix"},
		{"byte constant overflow", `function value(input: byte): byte { return input & 256; }`, "integer constant 256 cannot be represented as byte"},
		{"Go named unsigned constant overflow", `import go os from "os"; function value(input: os.FileMode): os.FileMode { return input | 4294967296; }`, "cannot be represented as os.FileMode"},
		{"negative literal shift", `function value(input: int): int { return input << -1; }`, "shift amount cannot be negative"},
		{"negative expression shift", `function value(input: int): int { return input >> (1 - 2); }`, "shift amount cannot be negative"},
		{"negative complement shift", `function value(input: int): int { return input << ^0; }`, "shift amount cannot be negative"},
		{"negative constant binding shift", `function value(input: int): int { const amount = 1 - 2; return input << amount; }`, "shift amount cannot be negative"},
		{"excessive constant shift", `function value(): int { return 1 >> 1075; }`, "exceeds the Go implementation limit of 1074"},
		{"excessive constant binding shift", `const base = 1; const amount = 1075; function value(): int { return base >> amount; }`, "exceeds the Go implementation limit of 1074"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
