package sema

import (
	"maps"
	"reflect"
	"slices"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Retain only the most recently checked entry for a syntax node. Its AST
// annotations then still describe that entry; a multi-entry cache would also
// need to save and restore all flow-sensitive semantic annotations.
type loopCheckMemo struct {
	entry, exit nullableFlowSnapshot
	memberTypes map[memberFlowKey]Type
	diagnostics []diagnostic.Diagnostic
	suppressed  int
}

// Fixed-point passes may repeat an inner loop with exactly the same entry.
// Reuse it only if checking has no effects on enclosing control/capture
// collectors. Otherwise run the normal solver, including every side effect.
// In particular, this is not the weaker lattice equality used for convergence:
// declaration types, task states, escape flags and diagnostic causes all count.
func (c *Checker) memoizeLoopCheck(statement ast.Statement, entry nullableFlowSnapshot) (bool, func()) {
	memo, exists := c.loopCheckMemos[statement]
	delete(c.loopCheckMemos, statement)
	noop := func() {}
	if !c.canMemoizeLoopCheck() {
		return false, noop
	}
	if exists && memo.suppressed == c.suppressFlowEffects && reflect.DeepEqual(memo.entry, entry) && reflect.DeepEqual(memo.memberTypes, c.memberTypes) {
		c.restoreNullableFlow(memo.exit)
		c.diagnostics = append(c.diagnostics, memo.diagnostics...)
		c.loopCheckMemos[statement] = memo
		return true, noop
	}
	marker := c.markLoopCollectors()
	diagnosticStart := len(c.diagnostics)
	suppressed := c.suppressFlowEffects
	return false, func() {
		if !c.canMemoizeLoopCheck() || !marker.unchanged(c) {
			return
		}
		if c.loopCheckMemos == nil {
			c.loopCheckMemos = map[ast.Statement]loopCheckMemo{}
		}
		c.loopCheckMemos[statement] = loopCheckMemo{
			entry: entry, exit: c.snapshotNullableFlow(), memberTypes: maps.Clone(c.memberTypes),
			diagnostics: append([]diagnostic.Diagnostic(nil), c.diagnostics[diagnosticStart:]...), suppressed: suppressed,
		}
	}
}

func (c *Checker) canMemoizeLoopCheck() bool {
	// These collectors depend on intermediate paths, not just the loop exit.
	// Keep their ordinary checks until their effects have explicit summaries.
	if len(c.exceptionFlows) != 0 || c.arrowReturns != nil || len(c.taskLabelFlows) != 0 || c.checkingLocalArrow != nil || c.inConstructor || c.inFieldInitializer {
		return false
	}
	for _, scope := range c.scopes {
		for _, symbol := range scope {
			if symbol.localArrowInference != nil || symbol.initializingArrow {
				return false
			}
		}
	}
	return true
}

type loopCollectorMarker struct {
	loopEdges    [][2]int
	breakEdges   []int
	writes       []map[source.Span]source.Span
	memberWrites []source.Span
}

func (c *Checker) markLoopCollectors() loopCollectorMarker {
	marker := loopCollectorMarker{}
	for _, flow := range c.loopFlowContexts {
		marker.loopEdges = append(marker.loopEdges, [2]int{len(flow.breaks), len(flow.continues)})
	}
	for _, flow := range c.breakFlowContexts {
		marker.breakEdges = append(marker.breakEdges, len(flow.breaks))
	}
	for _, writes := range c.capturedWrites {
		marker.writes = append(marker.writes, maps.Clone(writes))
	}
	marker.memberWrites = append(marker.memberWrites, c.capturedMemberWrites...)
	return marker
}

func (marker loopCollectorMarker) unchanged(c *Checker) bool {
	if len(marker.loopEdges) != len(c.loopFlowContexts) || len(marker.breakEdges) != len(c.breakFlowContexts) {
		return false
	}
	for index, flow := range c.loopFlowContexts {
		if marker.loopEdges[index] != [2]int{len(flow.breaks), len(flow.continues)} {
			return false
		}
	}
	for index, flow := range c.breakFlowContexts {
		if marker.breakEdges[index] != len(flow.breaks) {
			return false
		}
	}
	if len(marker.writes) != len(c.capturedWrites) || !slices.Equal(marker.memberWrites, c.capturedMemberWrites) {
		return false
	}
	for index, writes := range marker.writes {
		if !maps.Equal(writes, c.capturedWrites[index]) {
			return false
		}
	}
	return true
}
