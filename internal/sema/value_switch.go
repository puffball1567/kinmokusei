package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkValueSwitch(stmt *ast.ValueSwitchStmt) {
	value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
	value = c.checkSwitchTag(stmt.Value, value)
	if value.Kind == Nil || value.Kind == Null {
		c.report(stmt.Value.GetSpan(), "value switch cannot infer a concrete type from nil or null")
	} else if value.Kind != Invalid && !value.IsComparable() {
		c.report(stmt.Value.GetSpan(), fmt.Sprintf("value switch expression type %s is not comparable", value.String()))
	}
	defaultSeen := false
	constantCases := switchConstantCases{}
	entryFlow := c.snapshotNullableFlow()
	continuing := make([]nullableFlowSnapshot, 0, len(stmt.Cases)+1)
	var fallthroughFlow *nullableFlowSnapshot
	c.breakFlowContexts = append(c.breakFlowContexts, breakFlowContext{})
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		clause.FallsThrough = false
		if clause.Body != nil && len(clause.Body.Statements) != 0 {
			if branch, ok := clause.Body.Statements[len(clause.Body.Statements)-1].(*ast.BranchStmt); ok && branch.Kind == ast.FallthroughBranch && index+1 < len(stmt.Cases) {
				clause.FallsThrough = true
				c.validFallthrough[branch] = true
			}
		}
		caseEntry := entryFlow
		if fallthroughFlow != nil {
			caseEntry = c.mergeNullableFlow(entryFlow, entryFlow, *fallthroughFlow)
		}
		c.restoreNullableFlow(caseEntry)
		if clause.Default {
			if defaultSeen {
				c.report(clause.Span, "value switch may contain at most one default case")
			}
			defaultSeen = true
		}
		for _, expression := range clause.Values {
			caseType := c.singleValue(c.checkExpressionExpected(expression, value), expression.GetSpan())
			c.requireAssignable(value, caseType, expression.GetSpan())
			if caseType.Kind != Invalid && caseType.Kind != Nil && caseType.Kind != Null && !caseType.IsComparable() {
				c.report(expression.GetSpan(), fmt.Sprintf("value switch case type %s is not comparable", caseType.String()))
			}
			c.checkSwitchCaseConstant(expression, caseType, value, constantCases)
		}
		c.breakableDepth++
		c.checkBlock(clause.Body, true)
		c.breakableDepth--
		caseFlow := c.snapshotNullableFlow()
		if clause.FallsThrough && valueSwitchCaseFallthroughReachable(clause) {
			fallthroughFlow = &caseFlow
		} else {
			fallthroughFlow = nil
		}
		if !clause.FallsThrough && !definitelyReturns(clause.Body) {
			continuing = append(continuing, c.snapshotNullableFlow())
		}
	}
	if !defaultSeen {
		continuing = append(continuing, entryFlow)
	}
	flow := c.breakFlowContexts[len(c.breakFlowContexts)-1]
	c.breakFlowContexts = c.breakFlowContexts[:len(c.breakFlowContexts)-1]
	continuing = append(continuing, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
}
