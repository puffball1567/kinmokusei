package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Storage and result publication establish a source contract. Ordinary call
// arguments use checkExpressionExpectedSlot instead: callable entry guards run
// after all arguments have been evaluated, preserving call/defer/go ordering.
func (c *Checker) checkStorageExpressionExpectedSlot(slot *ast.Expression, expected Type) Type {
	actual := c.checkExpressionExpectedSlot(slot, expected)
	if expected.Kind == MultiValue && actual.Kind == MultiValue {
		return c.checkMultipleStorageSlot(slot, expected, actual)
	}
	if !c.needsStorageBoundary(expected, actual, *slot) {
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

func (c *Checker) needsStorageBoundary(expected, actual Type, expression ast.Expression) bool {
	// Extracted results can come from Go collections or callbacks. Validate
	// their runtime references even when their declared types are native.
	return actual.Kind != Invalid && c.requiresUniformNativeNonNull(expected, nil) &&
		isNilable(actual) && c.isAssignable(expected, actual) &&
		(expression == nil || !c.assignmentProvesNonNull(actual, expression))
}

func (c *Checker) checkMultipleStorageSlot(slot *ast.Expression, expected, actual Type) Type {
	if len(expected.Results) != len(actual.Results) {
		return actual
	}
	if checked, ok := (*slot).(*ast.UnaryExpr); ok && checked.ImplicitNonNull && len(checked.NonNullResults) != 0 {
		return actual
	}
	checks := make([]bool, len(expected.Results))
	needed := false
	for i, target := range expected.Results {
		// Do not let the helper's typed results hide an invalid transfer in a
		// different slot, including a class upcast that Go cannot forward.
		if !c.isAssignable(target, actual.Results[i]) || !assignable(target, actual.Results[i]) {
			return actual
		}
		checks[i] = c.needsStorageBoundary(target, actual.Results[i], nil)
		needed = needed || checks[i]
	}
	if !needed {
		return actual
	}
	span := (*slot).GetSpan()
	*slot = &ast.UnaryExpr{
		Operator: ast.NonNullOperator, Operand: *slot, ImplicitNonNull: true,
		AssertionType: typeRefFromType(expected, span), AssertionSpan: span,
		NonNullResults: checks, Span: span,
	}
	return expected
}
