package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Storage and result publication establish a source contract. Ordinary call
// arguments use checkExpressionExpectedSlot instead: callable entry guards run
// after all arguments have been evaluated, preserving call/defer/go ordering.
func (c *Checker) checkStorageExpressionExpectedSlot(slot *ast.Expression, expected Type) Type {
	actual := c.checkExpressionExpectedSlot(slot, expected)
	if actual.Kind == Invalid || !c.requiresUniformNativeNonNull(expected, nil) ||
		!isNilable(actual) || !c.isAssignable(expected, actual) || c.assignmentProvesNonNull(actual, *slot) {
		return actual
	}
	span := (*slot).GetSpan()
	*slot = &ast.UnaryExpr{
		Operator: ast.NonNullOperator, Operand: *slot,
		AssertionType: typeRefFromType(expected, span), AssertionSpan: span,
		ImplicitNonNull: true, Span: span,
	}
	return expected
}
