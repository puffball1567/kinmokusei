package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestGenericFunctionValueParsing(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"id<int>", "id[int]", "pair<int,bstring>", "pair[int,bstring]", "id[Map<int,int[]>]", "id<Box<Box<int>>>", "id[[2]int]", "id[*S]", "slices.Clone<int[],int>"} {
		t.Run(input, func(t *testing.T) {
			program, count := parseSource(t, "const f="+input+";")
			if count != 0 || len(program.Declarations) != 1 {
				t.Fatalf("parse %s: %d diagnostics", input, count)
			}
			value := program.Declarations[0].(*ast.VariableDecl).Value
			if index, ok := value.(*ast.IndexExpr); !ok || len(index.TypeArguments) == 0 {
				t.Fatalf("instantiation AST=%#v", value)
			}
		})
	}
	for _, input := range []string{"a<b>c", "a<b>>c", "a<b>=c", "a[index+1]", "a[low:high]", "a[low:high:max]"} {
		t.Run(input, func(t *testing.T) {
			program, count := parseSource(t, "const f="+input+";")
			if count != 0 || len(program.Declarations) != 1 {
				t.Fatalf("ordinary expression %s: %d diagnostics", input, count)
			}
			if index, ok := program.Declarations[0].(*ast.VariableDecl).Value.(*ast.IndexExpr); ok && len(index.TypeArguments) != 0 {
				t.Fatalf("ordinary index became type arguments: %#v", index)
			}
		})
	}
}
