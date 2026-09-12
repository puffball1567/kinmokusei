package ast

import "testing"

func TestLocalArrowGroup(t *testing.T) {
	t.Parallel()
	first := &VariableDecl{Value: &ArrowExpr{}}
	second := &VariableDecl{Value: &ArrowExpr{}}
	for _, boundary := range []Statement{&ExpressionStmt{}, &VariableDecl{Value: &IdentifierExpr{}}, &LabeledStmt{Statement: first}} {
		group := LocalArrowGroup([]Statement{first, second, boundary, first})
		if len(group) != 2 || group[0] != first || group[1] != second {
			t.Fatalf("group=%v", group)
		}
	}
	if len(LocalArrowGroup(nil)) != 0 {
		t.Fatal("empty block has a group")
	}
}
