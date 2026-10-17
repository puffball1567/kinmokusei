package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkNonNullAssertion(expr *ast.UnaryExpr) Type {
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
	// An unconstrained type parameter has interface-shaped metadata but is not
	// necessarily nil-capable when instantiated. Never emit an invalid nil test.
	if value.Kind == TypeParameter {
		storage, ok := c.goTypeForNativeStorage(value)
		if !ok || !gotypes.AssignableTo(gotypes.Typ[gotypes.UntypedNil], storage) {
			c.report(expr.Span, "postfix ! requires a type parameter whose constraint guarantees a nil representation")
			return Type{Kind: Invalid}
		}
	}
	expr.AssertionType = typeRefFromType(value, expr.Span)
	return value
}
