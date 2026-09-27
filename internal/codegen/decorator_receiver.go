package codegen

import (
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func decoratorCheckedReceiver(target *ast.DecoratorTarget, receiver goast.Expr, label string, failure func(string) *goast.ReturnStmt) ([]goast.Stmt, goast.Expr) {
	typed := goast.NewIdent("typedReceiver")
	ok := goast.NewIdent("receiverOK")
	checks := []goast.Stmt{
		&goast.AssignStmt{Lhs: []goast.Expr{typed, ok}, Tok: token.DEFINE, Rhs: []goast.Expr{decoratorDecode(receiver, ast.TypeRef{Name: target.RuntimeClassName}, target.ClassContract)}},
		&goast.IfStmt{Cond: &goast.BinaryExpr{
			X: &goast.UnaryExpr{Op: token.NOT, X: ok}, Op: token.LOR,
			Y: &goast.BinaryExpr{X: typed, Op: token.EQL, Y: goast.NewIdent("nil")},
		}, Body: &goast.BlockStmt{List: []goast.Stmt{failure(label + " expects a non-null " + target.ClassName + " receiver")}}},
	}
	name := target.RuntimeMethodName
	var object goast.Expr = typed
	if owner := target.MethodVirtualOwner; owner != "" {
		// Use the same virtual slot as ordinary source calls. Calling an
		// override body directly would bypass further derived implementations.
		dispatch := goast.NewIdent("dispatchReceiver")
		checks = append(checks,
			&goast.AssignStmt{Lhs: []goast.Expr{dispatch}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.SelectorExpr{X: typed, Sel: goast.NewIdent(virtualSelfName(owner))}}},
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: dispatch, Op: token.EQL, Y: goast.NewIdent("nil")},
				Body: &goast.BlockStmt{List: []goast.Stmt{failure(label + " expects an initialized virtual receiver")}}},
		)
		object = dispatch
		name = virtualSlotName(owner, name)
	}
	return checks, &goast.SelectorExpr{X: object, Sel: goast.NewIdent(goName(name))}
}
