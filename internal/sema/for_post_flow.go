package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// A for-loop continue reaches the post statement before returning to the
// condition. Join those paths with ordinary body completion, then apply the
// post's effects once to produce the backedge used by fixed-point checking.
func (c *Checker) checkForPostFlow(stmt *ast.ForStmt, bodyEntry nullableFlowSnapshot) (nullableFlowSnapshot, bool) {
	bodyFlow := c.snapshotNullableFlow()
	fallsThrough := !statementDefinitelyStopsBlock(stmt.Body)
	if stmt.Post == nil {
		return bodyFlow, fallsThrough
	}
	index := len(c.loopFlowContexts) - 1
	paths := append([]nullableFlowSnapshot(nil), c.loopFlowContexts[index].continues...)
	c.loopFlowContexts[index].continues = nil
	if fallsThrough {
		paths = append(paths, bodyFlow)
	}
	if len(paths) == 0 {
		// An unreachable post must still be checked for invalid syntax/types,
		// but must not supply mutation effects or a backedge after a break.
		c.suppressFlowEffects++
		c.checkStatement(stmt.Post)
		c.suppressFlowEffects--
		c.restoreNullableFlow(bodyFlow)
		return bodyFlow, false
	}
	c.restoreNullableFlow(c.mergeNullableFlow(bodyEntry, paths...))
	c.checkStatement(stmt.Post)
	return c.snapshotNullableFlow(), true
}
