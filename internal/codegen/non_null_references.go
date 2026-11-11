package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"path/filepath"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Keep one runtime check for explicit assertions and native conversions. The
// operand runs once outside the helper's lexical scope; the result retains its
// concrete type and interface checks preserve Go's typed-nil semantics.
func generateNonNullReference(operand goast.Expr, target ast.TypeRef, location source.Span, operation string) goast.Expr {
	return generateNonNullReferenceWhen(operand, target, location, operation, nil)
}

func generateNonNullReferenceWhen(operand goast.Expr, target ast.TypeRef, location source.Span, operation string, success goast.Expr) goast.Expr {
	value := goast.NewIdent("value")
	message := fmt.Sprintf("%s failed at %s:%d:%d", operation, filepath.Base(location.Path), location.Start.Line, location.Start.Column)
	parameters := []*goast.Field{{Names: []*goast.Ident{value}, Type: goType(target)}}
	arguments := []goast.Expr{operand}
	var condition goast.Expr = &goast.BinaryExpr{X: value, Op: token.EQL, Y: goast.NewIdent("nil")}
	if success != nil {
		flag := goast.NewIdent("success")
		parameters = append(parameters, &goast.Field{Names: []*goast.Ident{flag}, Type: goast.NewIdent("bool")})
		condition = &goast.BinaryExpr{X: flag, Op: token.LAND, Y: condition}
		arguments = append(arguments, success)
	}
	helper := &goast.FuncLit{
		Type: &goast.FuncType{
			Params:  &goast.FieldList{List: parameters},
			Results: &goast.FieldList{List: []*goast.Field{{Type: goType(target)}}},
		},
		Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.IfStmt{Cond: condition, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{stringLiteral(message)}}},
			}}},
			&goast.ReturnStmt{Results: []goast.Expr{value}},
		}},
	}
	return &goast.CallExpr{Fun: helper, Args: arguments}
}
