package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"
)

// Base initialization keeps the existing phase-local dispatch rule. An abstract
// slot has no base implementation: indirect calls during that phase (or calls
// on a Go-created zero value) must fail explicitly, never return a zero result.
func abstractMethodBody(class, method string) *goast.BlockStmt {
	return &goast.BlockStmt{List: []goast.Stmt{&goast.ExprStmt{X: &goast.CallExpr{
		Fun: goast.NewIdent("panic"), Args: []goast.Expr{&goast.BasicLit{
			Kind: token.STRING, Value: strconv.Quote("abstract method " + class + "." + method + " has no implementation in the current construction phase"),
		}},
	}}}}
}
