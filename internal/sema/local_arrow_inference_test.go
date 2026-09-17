package sema

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestLocalArrowGroupSnapshotsLexicalFlow(t *testing.T) {
	t.Parallel()
	key := memberFlowKey{path: "item"}
	c := &Checker{
		scopes:     []map[string]valueSymbol{{"value": {typeInfo: builtins["int"], declaredType: builtins["int"]}}},
		memberFlow: map[memberFlowKey]memberFlowState{key: {nonNull: true}},
	}
	first := &ast.VariableDecl{Name: "first", Value: &ast.ArrowExpr{}}
	last := &ast.VariableDecl{Name: "last", Value: &ast.ArrowExpr{}}
	c.predeclareLocalArrowGroup([]*ast.VariableDecl{first, last})
	context := c.scopes[0]["first"].localArrowInference.context
	if context != c.scopes[0]["last"].localArrowInference.context {
		t.Fatal("peers do not share their entry environment")
	}
	c.scopes[0]["value"] = valueSymbol{typeInfo: builtins["string"]}
	c.memberFlow[key] = memberFlowState{}
	if context.checker.scopes[0]["value"].typeInfo.Kind != Int || !context.checker.memberFlow[key].nonNull {
		t.Fatal("group entry snapshot changed with its caller")
	}
}
