package codegen

import (
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateNullableInterfaceUpcast(expr *ast.ClassUpcastExpr, operand goast.Expr) goast.Expr {
	// Evaluate the operand outside the helper's scope, exactly once. Only a
	// non-nil pointer is boxed; returning untyped nil creates a nil interface.
	value := goast.NewIdent("value")
	helper := &goast.FuncLit{
		Type: &goast.FuncType{
			Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{value}, Type: goType(expr.SourceType)}}},
			Results: &goast.FieldList{List: []*goast.Field{{Type: goType(expr.TargetType)}}},
		},
		Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: value, Op: token.EQL, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("nil")}},
			}}},
			&goast.ReturnStmt{Results: []goast.Expr{value}},
		}},
	}
	return &goast.CallExpr{Fun: helper, Args: []goast.Expr{operand}}
}
