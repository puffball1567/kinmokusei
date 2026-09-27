package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// A wrapper introduced by lowering must not make a value-only built-in legal
// as a statement or deferred call. Decide this from the source operation.
func unusedBuiltinResult(call *ast.CallExpr) string {
	switch call.Builtin {
	case ast.NotBuiltinCall, ast.CloseGoChannelCall, ast.CopyCall, ast.DeleteCall, ast.ClearCall,
		ast.ResultOKCall, ast.ResultFailCall:
		return ""
	}
	switch callee := call.Callee.(type) {
	case *ast.IdentifierExpr:
		return callee.Name
	case *ast.MemberExpr:
		return callee.Name
	}
	return "built-in"
}
