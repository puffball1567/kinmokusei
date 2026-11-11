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
	parameters := []*goast.Field{{Names: []*goast.Ident{value}, Type: goType(target)}}
	arguments := []goast.Expr{operand}
	var flag goast.Expr
	if success != nil {
		name := goast.NewIdent("success")
		flag = name
		parameters = append(parameters, &goast.Field{Names: []*goast.Ident{name}, Type: goast.NewIdent("bool")})
		arguments = append(arguments, success)
	}
	helper := &goast.FuncLit{
		Type: &goast.FuncType{
			Params:  &goast.FieldList{List: parameters},
			Results: &goast.FieldList{List: []*goast.Field{{Type: goType(target)}}},
		},
		Body: &goast.BlockStmt{List: []goast.Stmt{
			generateNonNullCheck(value, location, operation, flag),
			&goast.ReturnStmt{Results: []goast.Expr{value}},
		}},
	}
	return &goast.CallExpr{Fun: helper, Args: arguments}
}

// Direct checks consume an already captured, correctly typed reference.
func generateNonNullCheck(value goast.Expr, location source.Span, operation string, success goast.Expr) goast.Stmt {
	var condition goast.Expr = &goast.BinaryExpr{X: value, Op: token.EQL, Y: goast.NewIdent("nil")}
	if success != nil {
		condition = &goast.BinaryExpr{X: success, Op: token.LAND, Y: condition}
	}
	message := fmt.Sprintf("%s failed at %s:%d:%d", operation, filepath.Base(location.Path), location.Start.Line, location.Start.Column)
	return &goast.IfStmt{Cond: condition, Body: &goast.BlockStmt{List: []goast.Stmt{
		&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{stringLiteral(message)}}},
	}}}
}
