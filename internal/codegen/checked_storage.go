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

// Checked declarations have a payload and a presence/error result. Their
// inferred native payload type supplies the helper's public result contract.
// Keeping validation in the initializer also supports Go for initializers.
func generateCheckedBindingValue(producer goast.Expr, bindings []ast.Binding, condition ast.NonNullCondition) (goast.Expr, error) {
	results := make([]ast.TypeRef, len(bindings))
	checks := make([]bool, len(bindings))
	needed := false
	for i, binding := range bindings {
		results[i] = binding.ResolvedType
		checks[i] = binding.NonNull
		needed = needed || binding.NonNull
	}
	if !needed {
		return producer, nil
	}
	return generateNonNullResultsWhen(&ast.UnaryExpr{
		AssertionType: ast.TypeRef{GoResults: results},
		AssertionSpan: bindings[0].Span, NonNullResults: checks,
	}, producer, condition)
}
