package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"path/filepath"

	kmast "github.com/puffball1567/kinmokusei/internal/ast"
)

func generateNonNullAssertion(expr *kmast.UnaryExpr) (goast.Expr, error) {
	if !expr.AssertionType.IsSpecified() {
		return nil, fmt.Errorf("non-null assertion has no checked value type")
	}
	operand, err := generateExpression(expr.Operand)
	if err != nil {
		return nil, err
	}
	// Pass the operand as an argument: evaluate it exactly once outside the
	// helper's lexical scope, even if user code calls a binding named value.
	value := goast.NewIdent("value")
	location := expr.AssertionSpan
	message := fmt.Sprintf("non-null assertion failed at %s:%d:%d", filepath.Base(location.Path), location.Start.Line, location.Start.Column)
	helper := &goast.FuncLit{
		Type: &goast.FuncType{
			Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{value}, Type: goType(expr.AssertionType)}}},
			Results: &goast.FieldList{List: []*goast.Field{{Type: goType(expr.AssertionType)}}},
		},
		Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: value, Op: token.EQL, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{stringLiteral(message)}}},
			}}},
			&goast.ReturnStmt{Results: []goast.Expr{value}},
		}},
	}
	return &goast.CallExpr{Fun: helper, Args: []goast.Expr{operand}}, nil
}
