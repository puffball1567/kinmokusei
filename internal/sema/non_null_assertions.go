package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkNonNullAssertion(expr *ast.UnaryExpr) Type {
	if expr.ImplicitNonNull {
		// Flow analysis may check a lowered expression again. Keep the target
		// source contract, rather than replacing it with the operand's Go type.
		target := c.resolveType(expr.AssertionType)
		actual := c.checkExpression(expr.Operand)
		c.requireAssignable(target, actual, expr.Operand.GetSpan())
		return target
	}
	value := c.singleValue(c.checkExpression(expr.Operand), expr.Operand.GetSpan())
	if value.Kind == Invalid {
		return value
	}
	if value.Kind == Nullable && value.Element != nil {
		value = *value.Element
	}
	if value.Kind == Nil || value.Kind == Null || !isNilable(value) {
		c.report(expr.Span, fmt.Sprintf("postfix ! requires a nullable or nil-capable reference value, got %s", value.String()))
		return Type{Kind: Invalid}
	}
	expr.AssertionType = typeRefFromType(value, expr.Span)
	return value
}
