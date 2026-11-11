package codegen

import (
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func hasReferenceBoundaries(boundaries []*ast.TypeRef) bool {
	for _, boundary := range boundaries {
		if boundary != nil {
			return true
		}
	}
	return false
}

func referenceSuccess(condition ast.NonNullCondition, flag goast.Expr) goast.Expr {
	switch condition {
	case ast.NonNullIfPresent:
		return flag
	case ast.NonNullIfNoError:
		return &goast.BinaryExpr{X: flag, Op: token.EQL, Y: goast.NewIdent("nil")}
	default:
		return nil
	}
}
