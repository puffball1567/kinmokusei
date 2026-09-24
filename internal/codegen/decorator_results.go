package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func decoratorMultipleResultReturn(target *ast.DecoratorTarget, call goast.Expr) []goast.Stmt {
	variables := make([]goast.Expr, len(target.MethodResultSlots))
	values := make([]goast.Expr, len(target.MethodResultSlots))
	for index, slot := range target.MethodResultSlots {
		name := goast.NewIdent(fmt.Sprintf("result%d", index))
		variables[index] = name
		values[index] = decoratorBox(name, slot.Type, slot.Identity, slot.Contract)
	}
	array := ast.TypeRef{Element: &ast.TypeRef{Name: ast.DecoratorValueTypeName}}
	boxed := decoratorBox(&goast.CompositeLit{Type: goType(array), Elts: values}, array, target.MethodResultIdentity, target.MethodResultContract)
	return []goast.Stmt{
		// Capture all results from exactly one typed invocation before boxing.
		&goast.AssignStmt{Lhs: variables, Tok: token.DEFINE, Rhs: []goast.Expr{call}},
		&goast.ReturnStmt{Results: []goast.Expr{boxed, goast.NewIdent("nil")}},
	}
}
