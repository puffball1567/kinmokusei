package parser

import (
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/lexer"
)

func TestParsesSliceExpressionMatrix(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function slices(values: int[], low: int, high: int, max: int): void {
  const all = values[:];
  const suffix = values[low:];
  const prefix = values[:high];
  const middle = values[low:high];
  const full = values[low:high:max];
  const omittedLow = values[:high:max];
  const chained = values[low:high][0];
  const indexed = values[low];
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	tests := []struct {
		name                 string
		statement            int
		low, high, max, full bool
	}{
		{"all", 0, false, false, false, false},
		{"suffix", 1, true, false, false, false},
		{"prefix", 2, false, true, false, false},
		{"middle", 3, true, true, false, false},
		{"full", 4, true, true, true, true},
		{"omitted low", 5, false, true, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := function.Body.Statements[test.statement].(*ast.VariableDecl).Value
			sliced, ok := value.(*ast.SliceExpr)
			if !ok {
				t.Fatalf("expression = %T", value)
			}
			if (sliced.Low != nil) != test.low || (sliced.High != nil) != test.high || (sliced.Max != nil) != test.max || sliced.Full != test.full {
				t.Fatalf("slice = %#v", sliced)
			}
		})
	}
	chained := function.Body.Statements[6].(*ast.VariableDecl).Value.(*ast.IndexExpr)
	if _, ok := chained.Object.(*ast.SliceExpr); !ok {
		t.Fatalf("chained object = %T", chained.Object)
	}
	if _, ok := function.Body.Statements[7].(*ast.VariableDecl).Value.(*ast.IndexExpr); !ok {
		t.Fatalf("ordinary index changed shape")
	}
}

func TestRejectsMalformedSliceExpressionMatrix(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"missing high in full slice", `function bad(values: int[]): int[] { return values[1::3]; }`, "3-index slice requires a high bound"},
		{"missing max", `function bad(values: int[]): int[] { return values[1:2:]; }`, "3-index slice requires a max bound"},
		{"missing close", `function bad(values: int[]): int[] { return values[1:2; }`, "expected ']' after slice expression"},
		{"missing index and colon", `function bad(values: int[]): int { return values[]; }`, "expected expression"},
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
				}
			}
			if !found {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
