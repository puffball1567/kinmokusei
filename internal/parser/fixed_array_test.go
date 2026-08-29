package parser

import (
	"strings"
	"testing"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/lexer"
)

func TestParsesFixedArrayTypeMatrix(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function scalar(value: [3]int): [3]int { return value; }
function empty(value: [0]byte): [0]byte { return value; }
function nested(value: [2][4]string): [2][4]string { return value; }
function slice(value: int[]): int[] { return value; }
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	tests := []struct {
		name    string
		ref     ast.TypeRef
		lengths []int64
		base    string
		fixed   bool
		slice   bool
	}{
		{"scalar parameter", program.Declarations[0].(*ast.FunctionDecl).Parameters[0].Type, []int64{3}, "int", true, false},
		{"scalar return", program.Declarations[0].(*ast.FunctionDecl).ReturnType, []int64{3}, "int", true, false},
		{"zero", program.Declarations[1].(*ast.FunctionDecl).Parameters[0].Type, []int64{0}, "byte", true, false},
		{"nested", program.Declarations[2].(*ast.FunctionDecl).Parameters[0].Type, []int64{2, 4}, "string", true, false},
		{"slice unchanged", program.Declarations[3].(*ast.FunctionDecl).Parameters[0].Type, nil, "int", false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ref := test.ref
			if ref.IsFixedArray() != test.fixed || ref.IsSlice() != test.slice {
				t.Fatalf("shape = %#v", ref)
			}
			for _, want := range test.lengths {
				if !ref.IsFixedArray() || *ref.FixedLength != want {
					t.Fatalf("length = %#v, want %d", ref.FixedLength, want)
				}
				ref = *ref.Element
			}
			if len(test.lengths) == 0 && ref.Element != nil {
				ref = *ref.Element
			}
			if ref.Name != test.base {
				t.Fatalf("base = %q, want %q", ref.Name, test.base)
			}
		})
	}
}

func TestRejectsInvalidFixedArrayLengthMatrix(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"missing", `function bad(value: []int): void {}`, "expected fixed array length"},
		{"negative", `function bad(value: [-1]int): void {}`, "expected fixed array length"},
		{"fractional", `function bad(value: [1.5]int): void {}`, "expected fixed array length"},
		{"missing close", `function bad(value: [2int): void {}`, "expected ']' after fixed array length"},
		{"missing element", `function bad(value: [2]): void {}`, "expected type name"},
		{"overflow", `function bad(value: [9223372036854775808]int): void {}`, "fixed array length is out of range"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens, lexDiagnostics := lexer.Lex("invalid.otm", test.source)
			if len(lexDiagnostics) != 0 {
				t.Fatalf("lexer diagnostics = %v", lexDiagnostics)
			}
			_, diagnostics := Parse(tokens)
			found := false
			for _, item := range diagnostics {
				if strings.Contains(item.Message, test.want) {
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
