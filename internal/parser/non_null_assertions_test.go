package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestParsesCheckedPostfixAssertions(t *testing.T) {
	t.Parallel()
	for _, expression := range []string{
		`value!`, `value!.name`, `value![0]`, `value!()`, `get()!.name`,
		`(<-channel)!`, `!value!()`, `value!!.name`, `(value as! Reader)!.read()`,
	} {
		t.Run(expression, func(t *testing.T) {
			program, count := parseSource(t, `const result = `+expression+`;`)
			if count != 0 || len(program.Declarations) != 1 {
				t.Fatalf("diagnostics=%d program=%#v", count, program)
			}
		})
	}
	program, count := parseSource(t, "function run():void{\nconst result = value!\nconst other = value\n!test()\n}")
	if count != 0 || len(program.Declarations) != 1 {
		t.Fatalf("newline parsing: diagnostics=%d program=%#v", count, program)
	}
	statements := program.Declarations[0].(*ast.FunctionDecl).Body.Statements
	if len(statements) != 3 {
		t.Fatalf("newline statements=%#v", statements)
	}
	asserted := statements[0].(*ast.VariableDecl).Value.(*ast.UnaryExpr)
	if asserted.Operator != ast.NonNullOperator || asserted.AssertionSpan.Start.Column != 21 || asserted.AssertionSpan.End.Offset-asserted.AssertionSpan.Start.Offset != 1 {
		t.Fatalf("assertion operator/span=%#v", asserted)
	}
	if _, ok := statements[1].(*ast.VariableDecl).Value.(*ast.IdentifierExpr); !ok {
		t.Fatal("postfix ! crossed a newline")
	}
}
