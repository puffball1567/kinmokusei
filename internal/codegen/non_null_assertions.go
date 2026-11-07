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
	return generateNonNullReference(operand, expr.AssertionType, expr.AssertionSpan, "non-null assertion"), nil
}
