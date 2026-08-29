package sema

import (
	"strings"
	"testing"
)

func TestCompoundAssignmentAndIncrementValidMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"all integer compound operators", `function value(input: int): int { input += 1; input -= 1; input *= 2; input /= 2; input %= 7; input &= 6; input |= 1; input ^= 2; input &^= 1; input <<= 2; input >>= 1; return input; }`},
		{"numeric increments", `function value(integer: int, decimal: float, small: byte): float { integer++; integer--; decimal++; decimal--; small++; small--; return decimal + float(integer) + float(small); }`},
		{"string concatenation", `function value(input: string): string { input += "suffix"; return input; }`},
		{"slice map pointer and fixed array targets", `function value(items: int[], table: Map<string, int>, fixed: [2]int, pointer: *int): int { items[0] += 1; table["x"]++; fixed[0]--; (*pointer) *= 2; return items[0] + table["x"] + fixed[0] + *pointer; }`},
		{"class field", `class Counter { public count: int; constructor() { this.count = 0; } public function bump(): int { this.count++; this.count += 2; return this.count; } }`},
		{"for initializer and post", `function value(limit: int): int { let index = 0; for (index += 1; index < limit; index++) {} return index; }`},
		{"Go named integers", `import go os from "os"; import go time from "time"; function value(mode: os.FileMode, duration: time.Duration): int { mode |= os.ModeDir; mode &^= os.ModePerm; duration += time.Second; duration >>= 1; return int(mode) + int(duration); }`},
		{"dynamic divisor and shift", `function value(input: int, divisor: int, amount: int): int { input /= divisor; input %= divisor; input <<= amount; input >>= amount; return input; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestCompoundAssignmentAndIncrementFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"const compound", `function bad(): void { const value = 1; value += 1; }`, "cannot assign to const"},
		{"const increment", `function bad(): void { const value = 1; value++; }`, "cannot assign to const"},
		{"undefined increment", `function bad(): void { missing++; }`, "undefined name"},
		{"boolean increment", `function bad(value: boolean): void { value++; }`, "requires a numeric assignable operand"},
		{"string decrement", `function bad(value: string): void { value -= "x"; }`, "requires numeric operands"},
		{"float remainder", `function bad(value: float): void { value %= 2; }`, "requires integer operands"},
		{"bitwise float", `function bad(value: float): void { value &= 1; }`, "requires integer operands"},
		{"mixed typed integers", `function bad(left: int, right: int32): void { left += right; }`, "cannot mix int and int32"},
		{"negative shift", `function bad(value: int): void { value <<= -1; }`, "shift amount cannot be negative"},
		{"fixed width overflow", `function bad(value: byte): void { value += 256; }`, "integer constant 256 cannot be represented as byte"},
		{"integer division by zero", `function bad(value: int): void { value /= 0; }`, "integer divisor cannot be zero"},
		{"integer remainder by zero", `function bad(value: int): void { value %= 1 - 1; }`, "integer divisor cannot be zero"},
		{"string index assignment", `function bad(value: string): void { value[0] = byte(1); }`, "index expression is not assignable"},
		{"string index increment", `function bad(value: string): void { value[0]++; }`, "index expression is not assignable"},
		{"temporary array index", `function make(): [1]int { return [1]; } function bad(): void { make()[0]++; }`, "index expression is not assignable"},
		{"temporary object member", `function make(): { count: int } { return { count: 1 }; } function bad(): void { make().count++; }`, "member \"count\" is not assignable"},
		{"Go constant", `import go os from "os"; function bad(): void { os.ModePerm |= os.ModeDir; }`, "cannot assign to Go constant"},
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
