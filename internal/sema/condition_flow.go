package sema

import (
	"go/constant"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// checkCondition checks each operand once, in evaluation order, and retains
// separate flow states for the two outcomes. In particular, a later call can
// invalidate a member fact established by an earlier null comparison.
func (c *Checker) checkCondition(expression ast.Expression) (Type, nullableFlowSnapshot, nullableFlowSnapshot) {
	switch expr := expression.(type) {
	case *ast.BinaryExpr:
		if expr.Operator == "&&" || expr.Operator == "||" {
			left, leftTrue, leftFalse := c.checkCondition(expr.Left)
			left = c.singleValue(left, expr.Left.GetSpan())
			entry := c.snapshotNullableFlow()
			if expr.Operator == "&&" {
				c.restoreNullableFlow(leftTrue)
			} else {
				c.restoreNullableFlow(leftFalse)
			}
			right, rightTrue, rightFalse := c.checkCondition(expr.Right)
			right = c.singleValue(right, expr.Right.GetSpan())
			result := c.checkConstantOperation(expr, c.checkBinaryOperands(expr, left, right))
			var whenTrue, whenFalse nullableFlowSnapshot
			if expr.Operator == "&&" {
				whenTrue = rightTrue
				whenFalse = c.mergeNullableFlow(entry, leftFalse, rightFalse)
			} else {
				whenTrue = c.mergeNullableFlow(entry, leftTrue, rightTrue)
				whenFalse = rightFalse
			}
			// A compile-time left operand chooses whether the right operand
			// runs at all. Still type-check it, but do not count skipped awaits
			// as consumed or turn a guaranteed await into an optional one.
			if info, known := c.scalarConstant(expr.Left); known && info.Value.Kind() == constant.Bool {
				if constant.BoolVal(info.Value) == (expr.Operator == "&&") {
					whenTrue, whenFalse = rightTrue, rightFalse
				} else if expr.Operator == "&&" {
					whenTrue, whenFalse = leftFalse, leftFalse
				} else {
					whenTrue, whenFalse = leftTrue, leftTrue
				}
			}
			c.restoreNullableFlow(c.mergeNullableFlow(entry, whenTrue, whenFalse))
			return result, whenTrue, whenFalse
		}
	case *ast.UnaryExpr:
		if expr.Operator == "!" {
			operand, whenTrue, whenFalse := c.checkCondition(expr.Operand)
			operand = c.singleValue(operand, expr.Operand.GetSpan())
			result := c.checkConstantOperation(expr, c.checkUnaryOperand(expr, operand))
			return result, whenFalse, whenTrue
		}
	}

	result := c.checkExpression(expression)
	entry := c.snapshotNullableFlow()
	whenTrue, whenFalse := entry, entry
	if narrowing, ok := c.nullableConditionNarrowing(expression); ok {
		c.applyNarrowing(narrowing)
		if narrowing.nonNullWhenTrue {
			whenTrue = c.snapshotNullableFlow()
		} else {
			whenFalse = c.snapshotNullableFlow()
		}
		c.restoreNullableFlow(entry)
	}
	return result, whenTrue, whenFalse
}
