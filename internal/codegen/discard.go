package codegen

import (
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateDiscard(expression ast.Expression, arity int, annotation ast.TypeRef) (goast.Stmt, error) {
	value, err := generateExpression(expression)
	if err != nil {
		return nil, err
	}
	if annotation.IsSpecified() {
		// Sema has already required assignment compatibility. Preserve the type
		// context (notably nil and large constants), including in for headers.
		value = &goast.CallExpr{Fun: &goast.ParenExpr{X: goType(annotation)}, Args: []goast.Expr{value}}
	}
	targets := make([]goast.Expr, arity)
	for i := range targets {
		targets[i] = goast.NewIdent("_")
	}
	return &goast.AssignStmt{Lhs: targets, Tok: token.ASSIGN, Rhs: []goast.Expr{value}}, nil
}
