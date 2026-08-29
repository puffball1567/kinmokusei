package sema

import (
	"strings"
	"testing"
)

func TestCABIExportSuccessMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"void without parameters", `export c("ontama_ping") function ping(): void {}`},
		{"all fixed scalar types", `export c("ontama_scalars") function scalars(a: byte, b: int32, c: int64, d: float32, e: float, f: float64, g: number): float64 { return e; }`},
		{"multiple exports", `export c("ontama_left") function left(value: int32): int32 { return value; } export c("ontama_right") function right(value: int64): int64 { return value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
		})
	}
}

func TestCABIExportFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"empty symbol", `export c("") function value(): void {}`, "must start with an ASCII letter"},
		{"leading underscore", `export c("_value") function value(): void {}`, "must start with an ASCII letter"},
		{"hyphen", `export c("bad-value") function value(): void {}`, "contain only ASCII"},
		{"Unicode", `export c("温泉") function value(): void {}`, "must start with an ASCII letter"},
		{"Go keyword", `export c("func") function value(): void {}`, "is a Go keyword"},
		{"reserved main", `export c("main") function value(): void {}`, "is reserved by the generated Go package"},
		{"reserved init", `export c("init") function value(): void {}`, "is reserved by the generated Go package"},
		{"duplicate symbol", `export c("same") function left(): void {} export c("same") function right(): void {}`, `duplicate C ABI symbol "same"`},
		{"implementation collision", `function same(): void {} export c("same") function other(): void {}`, `generated Go name "same" collides`},
		{"machine width int", `export c("value") function value(input: int): void {}`, `parameter "input" has unsupported type int`},
		{"boolean", `export c("value") function value(input: boolean): void {}`, `parameter "input" has unsupported type boolean`},
		{"string", `export c("value") function value(input: string): void {}`, `parameter "input" has unsupported type string`},
		{"slice", `export c("value") function value(input: byte[]): void {}`, `parameter "input" has unsupported type byte[]`},
		{"object result", `export c("value") function value(): { item: int32 } { return { item: 1 }; }`, `result has unsupported type non-scalar type`},
		{"error result", `export c("value") function value(): error { return nil; }`, `result has unsupported type error`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
