package sema

import (
	"maps"
	"reflect"
	"slices"

	"github.com/puffball1567/kinmokusei/internal/source"
)

// Read inputs are separate from write-only collectors. In particular, a catch
// join accumulated before this loop is not an input to its transfer function.
type loopMemoContext struct {
	suppressed, loops, breaks, initializerDepth int
	writes, memberWrites                        int
	constructor, initializer, static, inferred  bool
	available                                   map[string]bool
	targets                                     []branchFlowTarget
	labels                                      map[source.Span]*taskLabelFlow
	exceptions                                  []loopExceptionInput
	bases                                       []int
	roots                                       []map[source.Span]bool
	arrows                                      map[*localArrowInference][2]bool
}

type loopExceptionInput struct {
	entry   nullableFlowSnapshot
	finally bool
}

func (c *Checker) loopMemoContext() loopMemoContext {
	context := loopMemoContext{
		suppressed: c.suppressFlowEffects, loops: len(c.loopFlowContexts), breaks: len(c.breakFlowContexts),
		writes: len(c.capturedWrites), memberWrites: len(c.capturedMemberWrites),
		constructor: c.inConstructor, initializer: c.inFieldInitializer, static: c.fieldInitializerStatic, initializerDepth: c.fieldInitializerArrowDepth,
		inferred: c.arrowReturns != nil, available: maps.Clone(c.fieldInitializerAvailable), targets: slices.Clone(c.branchFlowTargets),
		labels: cloneTaskLabels(c.taskLabelFlows), bases: slices.Clone(c.callableScopeBases),
		arrows: map[*localArrowInference][2]bool{},
	}
	for _, frame := range c.exceptionFlows {
		context.exceptions = append(context.exceptions, loopExceptionInput{frame.entry, frame.finally})
	}
	for _, roots := range c.capturedMemberRoots {
		context.roots = append(context.roots, maps.Clone(roots))
	}
	// Snapshots clone value symbols, but lazy inference objects are shared.
	// Record their changing status by value rather than comparing aliased pointers.
	for _, scope := range c.scopes {
		for _, symbol := range scope {
			if arrow := symbol.localArrowInference; arrow != nil {
				context.arrows[arrow] = [2]bool{arrow.checking, arrow.checked}
			}
		}
	}
	if arrow := c.checkingLocalArrow; arrow != nil {
		context.arrows[arrow] = [2]bool{arrow.checking, arrow.checked}
	}
	return context
}

func (left loopMemoContext) equal(right loopMemoContext) bool {
	// Statement identity determines the branch destination, even when two
	// different targets happen to have structurally identical syntax.
	if !slices.Equal(left.targets, right.targets) {
		return false
	}
	left.targets, right.targets = nil, nil
	return reflect.DeepEqual(left, right)
}

func cloneTaskLabels(labels map[source.Span]*taskLabelFlow) map[source.Span]*taskLabelFlow {
	if labels == nil {
		return nil
	}
	cloned := make(map[source.Span]*taskLabelFlow, len(labels))
	for label, flow := range labels {
		cloned[label] = &taskLabelFlow{entry: maps.Clone(flow.entry), incoming: maps.Clone(flow.incoming)}
	}
	return cloned
}
