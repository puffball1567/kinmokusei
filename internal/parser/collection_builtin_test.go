package parser

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
)

func TestParsesCollectionBuiltinCallShapes(t *testing.T) {
	program, diagnosticCount := parseSource(t, `
function use(values: int[], other: int[], lookup: Map<string, int>): void {
  const length = len(values);
  const capacity = cap(values);
  const one = append(values, 1, 2);
  const many = append(values, other...);
  const copied = copy(values, other);
  delete(lookup, "x");
  clear(values);
  const lower = min(1, 2, 3);
  const upper = max(1, 2, 3);
  const made = makeSlice[int](2, 4);
  const mapped = makeMap[string, int](8);
  const copiedArray = copyArray[[2]int](values);
  const viewedArray = viewArray[[2]int](values);
}
`)
	if diagnosticCount != 0 {
		t.Fatalf("got %d parser diagnostics", diagnosticCount)
	}
	function := program.Declarations[0].(*ast.FunctionDecl)
	for index, name := range []string{"len", "cap", "append", "append", "copy", "delete", "clear", "min", "max", "makeSlice", "makeMap", "copyArray", "viewArray"} {
		var call *ast.CallExpr
		if declaration, ok := function.Body.Statements[index].(*ast.VariableDecl); ok {
			call = declaration.Value.(*ast.CallExpr)
		} else {
			call = function.Body.Statements[index].(*ast.ExpressionStmt).Value.(*ast.CallExpr)
		}
		callee := call.Callee.(*ast.IdentifierExpr)
		if callee.Name != name {
			t.Fatalf("statement %d callee = %q, want %q", index, callee.Name, name)
		}
		if name == "makeSlice" && len(call.TypeArguments) != 1 {
			t.Fatalf("makeSlice type arguments = %d", len(call.TypeArguments))
		}
		if name == "makeMap" && len(call.TypeArguments) != 2 {
			t.Fatalf("makeMap type arguments = %d", len(call.TypeArguments))
		}
		if (name == "copyArray" || name == "viewArray") && (len(call.TypeArguments) != 1 || !call.TypeArguments[0].IsFixedArray()) {
			t.Fatalf("%s target = %#v", name, call.TypeArguments)
		}
		if index == 3 && !call.Expanded {
			t.Fatal("append spread was not preserved")
		}
	}
}
