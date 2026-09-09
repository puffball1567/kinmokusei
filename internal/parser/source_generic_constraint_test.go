package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestParsesSourceGenericConstraint(t *testing.T) {
	program, count := parseSource(t, `constraint Rows<S extends Slice<E>, E> = ~S[] | [2]S;`)
	if count != 0 {
		t.Fatalf("parser diagnostics=%d", count)
	}
	declaration := program.Declarations[0].(*ast.InterfaceDecl)
	if !declaration.Constraint || len(declaration.TypeParameters) != 2 || declaration.TypeParameters[0].Constraint.GenericArguments[0].Name != "E" || len(declaration.Terms) != 2 {
		t.Fatalf("constraint=%#v", declaration)
	}
	for _, input := range []string{`constraint Slice<`, `constraint Slice<E extends> = ~E[];`, `constraint Slice<E> = ;`, `constraint Slice<> = ~int[];`} {
		if _, count := parseSource(t, input); count == 0 {
			t.Fatalf("malformed declaration accepted: %s", input)
		}
	}
}
