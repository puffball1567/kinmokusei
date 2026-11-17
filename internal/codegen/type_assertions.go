package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// A Go assertion checks dynamic type identity, but a typed nil function can
// match a source function signature without satisfying its non-null contract.
// Keep raw Go targets unchanged and validate source targets at this boundary.
func generateGoTypeAssertion(expr *ast.GoTypeAssertionExpr, operand goast.Expr) goast.Expr {
	if !expr.NonNull {
		return &goast.TypeAssertExpr{X: operand, Type: goType(expr.Type)}
	}
	if !expr.Checked {
		assertion := &goast.TypeAssertExpr{X: operand, Type: goType(expr.Type)}
		return generateNonNullReference(assertion, expr.Type, expr.Span, "non-null type assertion")
	}
	input := goast.NewIdent("__kinmokuseiAssertInput")
	value := goast.NewIdent("__kinmokuseiAssertValue")
	assertion := &goast.TypeAssertExpr{X: input, Type: goType(expr.Type)}
	results := &goast.FieldList{List: []*goast.Field{{Type: goType(expr.Type)}}}
	body := &goast.BlockStmt{}
	ok := goast.NewIdent("__kinmokuseiAssertOK")
	results.List = append(results.List, &goast.Field{Type: goast.NewIdent("bool")})
	var valid goast.Expr = &goast.BinaryExpr{X: value, Op: token.NEQ, Y: goast.NewIdent("nil")}
	if expr.Type.NonNullKinds != 0 {
		valid = &goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiIsNonNullKinds"), Args: []goast.Expr{value, &goast.BasicLit{Kind: token.INT, Value: fmt.Sprint(expr.Type.NonNullKinds)}}}
	}
	body.List = []goast.Stmt{
		&goast.AssignStmt{Lhs: []goast.Expr{value, ok}, Tok: token.DEFINE, Rhs: []goast.Expr{assertion}},
		&goast.ReturnStmt{Results: []goast.Expr{value, &goast.BinaryExpr{X: ok, Op: token.LAND, Y: valid}}},
	}
	helper := &goast.FuncLit{Type: &goast.FuncType{
		Params: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{input}, Type: &goast.InterfaceType{Methods: &goast.FieldList{}}}}}, Results: results,
	}, Body: body}
	return &goast.CallExpr{Fun: helper, Args: []goast.Expr{operand}}
}
