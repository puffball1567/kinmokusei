package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Keep loops and switches in one lexical stack: the nearest break target may
// be a loop inside a switch, and a label may bypass several inner constructs.
type branchFlowTarget struct {
	statement  ast.Statement
	scopeBase  int
	loopIndex  int
	breakIndex int
}

func (c *Checker) pushBranchFlowTarget(statement ast.Statement, loop bool) {
	target := branchFlowTarget{statement: statement, scopeBase: len(c.scopes), loopIndex: -1, breakIndex: -1}
	if loop {
		target.loopIndex = len(c.loopFlowContexts)
	} else {
		target.breakIndex = len(c.breakFlowContexts)
	}
	c.branchFlowTargets = append(c.branchFlowTargets, target)
}

func (c *Checker) popBranchFlowTarget() {
	c.branchFlowTargets = c.branchFlowTargets[:len(c.branchFlowTargets)-1]
}

func (c *Checker) recordBranchFlow(branch *ast.BranchStmt) {
	if c.suppressFlowEffects != 0 {
		return
	}
	for index := len(c.branchFlowTargets) - 1; index >= 0; index-- {
		target := c.branchFlowTargets[index]
		if branch.Label != "" && c.resolvedBranchTargets[branch] != target.statement {
			continue
		}
		if branch.Kind == ast.ContinueBranch && target.loopIndex < 0 {
			continue
		}
		// These scopes will be unwound by the branch. Later awaits in them
		// cannot discharge a task on this edge, even if another path reaches
		// those awaits before the ordinary scope-exit check.
		for _, scope := range c.scopes[target.scopeBase:] {
			c.reportUnconsumedTasks(scope)
		}
		flow := c.snapshotNullableFlow()
		if target.loopIndex >= 0 {
			context := &c.loopFlowContexts[target.loopIndex]
			if branch.Kind == ast.ContinueBranch {
				context.continues = append(context.continues, flow)
			} else {
				context.breaks = append(context.breaks, flow)
			}
		} else {
			context := &c.breakFlowContexts[target.breakIndex]
			context.breaks = append(context.breaks, flow)
		}
		return
	}
}
