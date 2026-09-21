package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkMultipleReturn(stmt *ast.ReturnStmt) {
	if c.result.Kind != MultiValue {
		c.report(stmt.Span, "comma-separated returns require a multiple-result signature; add an explicit or contextual return type")
	} else if len(c.result.Results) != 1+len(stmt.AdditionalValues) {
		c.report(stmt.Span, fmt.Sprintf("multiple result count mismatch: got %d results, expected %d", 1+len(stmt.AdditionalValues), len(c.result.Results)))
	}
	check := func(slot *ast.Expression, index int) {
		expected := Type{Kind: Invalid}
		if index < len(c.result.Results) {
			expected = c.result.Results[index]
		}
		actual := c.checkExpressionExpectedSlot(slot, expected)
		// Each comma-separated expression must contribute exactly one value.
		c.requireAssignable(expected, actual, (*slot).GetSpan())
	}
	check(&stmt.Value, 0)
	for index := range stmt.AdditionalValues {
		check(&stmt.AdditionalValues[index], index+1)
	}
}
