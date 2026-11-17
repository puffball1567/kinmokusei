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
	if condition == ast.NonNullAlways {
		return producer, nil
	}
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

// Inferred ordinary results already have their destination's exact Go type.
// Direct checks preserve private companion result types and local type shadowing.
func generateInferredBindingChecks(bindings []ast.Binding) []goast.Stmt {
	var checks []goast.Stmt
	for _, binding := range bindings {
		if binding.NonNull {
			checks = append(checks, generateTypedNonNullCheck(
				goast.NewIdent(goName(binding.Name)), binding.ResolvedType, binding.Span, "non-null storage boundary", nil,
			))
		}
	}
	return checks
}
