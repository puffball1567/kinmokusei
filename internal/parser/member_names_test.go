package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestKeywordMemberNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"static", "default", "class", "function", "go", "await", "null", "constructor"} {
		program, count := parseSource(t, "function read(value:DecoratorContext):void{value."+name+";}")
		if count != 0 {
			t.Fatalf("member %s: diagnostics=%d", name, count)
		}
		function := program.Declarations[0].(*ast.FunctionDecl)
		member := function.Body.Statements[0].(*ast.ExpressionStmt).Value.(*ast.MemberExpr)
		if member.Name != name {
			t.Fatalf("member=%s want %s", member.Name, name)
		}
	}
	for _, invalid := range []string{"1", `"static"`, ";", ""} {
		_, count := parseSource(t, "function read():void{value."+invalid+";}")
		if count == 0 {
			t.Errorf("invalid member %q accepted", invalid)
		}
	}
}
