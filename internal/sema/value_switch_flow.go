package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Case tests run in source order until the first match. Keep every possible
// match in a grouped case, but carry the final failed-test state to later tests.
// Bodies are checked separately: they cannot affect tests on another path, and
// fallthrough enters a body without evaluating that clause's expressions.
func (c *Checker) checkValueSwitchCases(stmt *ast.ValueSwitchStmt, value Type, entry nullableFlowSnapshot) ([]nullableFlowSnapshot, nullableFlowSnapshot) {
	entries := make([]nullableFlowSnapshot, len(stmt.Cases))
	constants := switchConstantCases{}
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		if clause.Default {
			continue
		}
		matches := make([]nullableFlowSnapshot, 0, len(clause.Values))
		for _, expression := range clause.Values {
			caseType := c.singleValue(c.checkExpressionExpected(expression, value), expression.GetSpan())
			c.requireAssignable(value, caseType, expression.GetSpan())
			if caseType.Kind != Invalid && caseType.Kind != Nil && caseType.Kind != Null && !caseType.IsComparable() {
				c.report(expression.GetSpan(), fmt.Sprintf("value switch case type %s is not comparable", caseType.String()))
			}
			c.checkSwitchCaseConstant(expression, caseType, value, constants)
			matches = append(matches, c.snapshotNullableFlow())
		}
		entries[index] = c.mergeNullableFlow(entry, matches...)
	}
	unmatched := c.snapshotNullableFlow()
	for index := range stmt.Cases {
		if stmt.Cases[index].Default {
			// Default is selected only after all tests fail, regardless of its
			// textual position among the case bodies.
			entries[index] = unmatched
		}
	}
	return entries, unmatched
}
