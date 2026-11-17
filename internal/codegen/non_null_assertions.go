package codegen

import (
	"fmt"
	goast "go/ast"

	kmast "github.com/puffball1567/kinmokusei/internal/ast"
)

func generateNonNullAssertion(expr *kmast.UnaryExpr) (goast.Expr, error) {
	if !expr.AssertionType.IsSpecified() {
		return nil, fmt.Errorf("non-null assertion has no checked value type")
	}
	operand, err := generateExpression(expr.Operand)
	if err != nil {
		return nil, err
	}
	if chain, ok := expr.Operand.(*kmast.OptionalChainExpr); ok && chain.Checked && !expr.ImplicitNonNull {
		return generateOptionalAssertion(expr, operand), nil
	}
	if expr.ImplicitNonNull && len(expr.NonNullResults) != 0 {
		return generateNonNullResults(expr, operand)
	}
	operation := "non-null assertion"
	if expr.ImplicitNonNull {
		operation = "non-null storage boundary"
	}
	return generateNonNullReference(operand, expr.AssertionType, expr.AssertionSpan, operation), nil
}
