package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestParseContextualArrowParameters(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`const f=(n)=>n;`, `const f=(left,right:int)=>left+right;`, `const f=(...values)=>{};`, `const f=(n,)=>{return n;};`} {
		program, count := parseSource(t, input)
		if count != 0 {
			t.Fatalf("%s: %d diagnostics", input, count)
		}
		arrow := program.Declarations[0].(*ast.VariableDecl).Value.(*ast.ArrowExpr)
		parameter := arrow.Parameters[0]
		if parameter.Type.IsSpecified() || input[parameter.Span.Start.Offset:parameter.Span.End.Offset] != parameter.Name {
			t.Fatalf("%s: %+v", input, parameter)
		}
	}
	for _, input := range []string{`function f(n):void{}`, `const f=(...values,n)=>{};`, `const f=(n:)=>n;`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Fatalf("accepted %s", input)
		}
	}
}
