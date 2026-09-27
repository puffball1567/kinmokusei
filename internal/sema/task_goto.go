package sema

import (
	"fmt"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type taskLabelFlow struct {
	entry    map[source.Span]uint8
	incoming map[source.Span]uint8
}

func (c *Checker) checkLabeledStatement(label *ast.LabeledStmt, fallsThrough bool) {
	c.enterTaskLabel(label, fallsThrough)
	c.invalidateControlTransferFlow(label.Span)
	c.checkStatement(label.Statement)
}

func mergeTaskState(left, right uint8) uint8 {
	if left == taskNotTracked {
		return right
	}
	if right == taskNotTracked || left == right {
		return left
	}
	return taskMaybeConsumed
}

func (c *Checker) taskLabel(span source.Span) *taskLabelFlow {
	if c.taskLabelFlows == nil {
		c.taskLabelFlows = map[source.Span]*taskLabelFlow{}
	}
	flow := c.taskLabelFlows[span]
	if flow == nil {
		flow = &taskLabelFlow{}
		c.taskLabelFlows[span] = flow
	}
	return flow
}

func (c *Checker) enterTaskLabel(label *ast.LabeledStmt, fallsThrough bool) {
	if c.suppressFlowEffects != 0 {
		return
	}
	flow := c.taskLabel(label.LabelSpan)
	flow.entry = map[source.Span]uint8{}
	for _, scope := range c.scopes {
		for name, symbol := range scope {
			if symbol.taskState == taskNotTracked {
				continue
			}
			if incoming := flow.incoming[symbol.declarationSpan]; incoming != taskNotTracked {
				if fallsThrough {
					symbol.taskState = mergeTaskState(symbol.taskState, incoming)
				} else {
					symbol.taskState = incoming
				}
				scope[name] = symbol
			}
			flow.entry[symbol.declarationSpan] = symbol.taskState
		}
	}
	// Collect fresh forward edges on each loop fixed-point pass.
	flow.incoming = nil
}

func (c *Checker) checkGotoTasks(branch *ast.BranchStmt) {
	if c.suppressFlowEffects != 0 {
		return
	}
	block := c.gotoTargetBlocks[branch]
	scopeCount, active := c.blockScopeCounts[block]
	if !active {
		return
	} // Label validation diagnoses invalid destinations.
	flow := c.taskLabel(branch.ResolvedDeclaration)
	backward := branch.ResolvedDeclaration.Start.Offset < branch.Span.Start.Offset
	base := 0
	if len(c.callableScopeBases) != 0 {
		base = c.callableScopeBases[len(c.callableScopeBases)-1]
	}
	for index := base; index < len(c.scopes); index++ {
		names := make([]string, 0, len(c.scopes[index]))
		for name := range c.scopes[index] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			symbol := c.scopes[index][name]
			if symbol.taskState == taskNotTracked {
				continue
			}
			// Leaving a nested scope, or jumping before a declaration that
			// will start a fresh task, ends the current binding's lifetime.
			if index >= scopeCount || backward && symbol.declarationSpan.Start.Offset > branch.ResolvedDeclaration.Start.Offset {
				c.reportUnconsumedTasks(map[string]valueSymbol{name: symbol})
				continue
			}
			if backward {
				if entry := flow.entry[symbol.declarationSpan]; entry != taskNotTracked && entry != symbol.taskState {
					c.report(branch.Span, fmt.Sprintf("goto %q changes the consumption state of Task %q; a backward jump must preserve Task ownership", branch.Label, name))
				}
				continue
			}
			if flow.incoming == nil {
				flow.incoming = map[source.Span]uint8{}
			}
			flow.incoming[symbol.declarationSpan] = mergeTaskState(flow.incoming[symbol.declarationSpan], symbol.taskState)
		}
	}
}
