package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestConstraintIntersectionSyntax(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`constraint Both = A & B & ~int;`,
		"constraint Both<E> = A<E> &\n B<E>\n",
		`constraint Both<E>=~E[] & E[];`,
	} {
		program, count := parseSource(t, source)
		if count != 0 {
			t.Fatalf("%s: diagnostics=%d", source, count)
		}
		declaration := program.Declarations[0].(*ast.InterfaceDecl)
		if !declaration.Constraint || !declaration.Intersection || len(declaration.Terms) < 2 {
			t.Fatalf("constraint=%#v", declaration)
		}
	}
	for _, source := range []string{
		`constraint Bad = A &;`, `constraint Bad = A &`,
		`constraint Bad = A & B | C;`, `constraint Bad = A | B & C;`,
		`constraint Bad = & A;`, `constraint Bad = A && B;`,
	} {
		if _, count := parseSource(t, source); count == 0 {
			t.Fatalf("accepted malformed intersection: %s", source)
		}
	}
}
