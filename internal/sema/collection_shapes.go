package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Indexing and slicing a constrained operand use its common collection shape,
// while slicing a string/slice retains the original parameter type. Reuse the
// source-aware bound metadata: Go storage alone erases native OOP/nullability.
// Mixed underlying shapes remain unsupported here, even where Go indexing can
// accept a common element type without a common underlying collection type.
func (c *Checker) collectionOperandShape(value Type) Type {
	if value.Kind != TypeParameter {
		return value
	}
	shape := c.constraintArgumentShape(value)
	switch shape.Kind {
	case Array, FixedArray, Map, String, GoPointer:
		return shape
	default:
		return value
	}
}

func (c *Checker) checkMapIndex(expr *ast.IndexExpr, key, actual Type) {
	c.checkNumericMaterialization(expr.Index, key)
	c.requireAssignable(key, actual, expr.Index.GetSpan())
	if info, known := c.checkedNumericConstant(expr.Index, actual); known && key.IsNumeric() {
		if target, ok := goTypeOf(key); ok {
			if err := c.checkNumericConstantAssignment(info, target); err != nil {
				c.report(expr.Index.GetSpan(), err.Error())
			}
		}
	}
}
