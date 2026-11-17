package sema

import (
	"reflect"

	"github.com/puffball1567/kinmokusei/internal/source"
)

// Check against empty collectors, then publish the resulting transfer summary.
// Capturing before/after lengths would retain effects of the surrounding path
// and make a cached loop depend on the previous iteration's exception join.
type loopCheckEffects struct {
	loops        []loopFlowContext
	breaks       []breakFlowContext
	exceptions   []*exceptionFlowContext
	returns      *arrowReturnInference
	writes       []map[source.Span]source.Span
	memberWrites []source.Span
}

func (c *Checker) loopEffects() loopCheckEffects {
	return loopCheckEffects{c.loopFlowContexts, c.breakFlowContexts, c.exceptionFlows, c.arrowReturns, c.capturedWrites, c.capturedMemberWrites}
}

func (c *Checker) collectLoopEffects(entry nullableFlowSnapshot) loopCheckEffects {
	previous := c.loopEffects()
	c.loopFlowContexts = make([]loopFlowContext, len(previous.loops))
	for i, flow := range previous.loops {
		c.loopFlowContexts[i].conditionExit = flow.conditionExit
	}
	c.breakFlowContexts = make([]breakFlowContext, len(previous.breaks))
	c.exceptionFlows = make([]*exceptionFlowContext, len(previous.exceptions))
	for i, flow := range previous.exceptions {
		c.exceptionFlows[i] = &exceptionFlowContext{entry: flow.entry, flow: c.mergeNullableFlow(flow.entry, entry), finally: flow.finally}
	}
	if previous.returns != nil {
		c.arrowReturns = &arrowReturnInference{}
	}
	c.capturedWrites = make([]map[source.Span]source.Span, len(previous.writes))
	for i := range c.capturedWrites {
		c.capturedWrites[i] = map[source.Span]source.Span{}
	}
	c.capturedMemberWrites = make([]source.Span, len(previous.memberWrites))
	return previous
}

func (c *Checker) finishLoopEffects(previous loopCheckEffects) loopCheckEffects {
	effects := c.loopEffects()
	c.loopFlowContexts, c.breakFlowContexts = previous.loops, previous.breaks
	c.exceptionFlows, c.arrowReturns = previous.exceptions, previous.returns
	c.capturedWrites, c.capturedMemberWrites = previous.writes, previous.memberWrites
	c.replayLoopEffects(effects)
	return effects
}

func (c *Checker) replayLoopEffects(effects loopCheckEffects) {
	for i, flow := range effects.loops {
		count := c.loopTargetScopeCount(i, true)
		for _, edge := range flow.breaks {
			c.loopFlowContexts[i].breaks = appendDistinctLoopFlow(c.loopFlowContexts[i].breaks, edge, count)
		}
		for _, edge := range flow.continues {
			c.loopFlowContexts[i].continues = appendDistinctLoopFlow(c.loopFlowContexts[i].continues, edge, count)
		}
	}
	for i, flow := range effects.breaks {
		count := c.loopTargetScopeCount(i, false)
		for _, edge := range flow.breaks {
			c.breakFlowContexts[i].breaks = appendDistinctLoopFlow(c.breakFlowContexts[i].breaks, edge, count)
		}
	}
	for i, flow := range effects.exceptions {
		target := c.exceptionFlows[i]
		target.flow = c.mergeNullableFlow(target.entry, target.flow, flow.flow)
		for _, edge := range flow.returns {
			target.returns = appendDistinctLoopFlow(target.returns, edge, len(target.entry.scopes))
		}
	}
	if effects.returns != nil {
		for _, returned := range effects.returns.values {
			found := false
			for _, existing := range c.arrowReturns.values {
				if existing.statement == returned.statement && reflect.DeepEqual(existing.typeInfo, returned.typeInfo) {
					found = true
					break
				}
			}
			if !found {
				c.arrowReturns.values = append(c.arrowReturns.values, returned)
			}
		}
		for _, statement := range effects.returns.tries {
			found := false
			for _, existing := range c.arrowReturns.tries {
				if existing == statement {
					found = true
					break
				}
			}
			if !found {
				c.arrowReturns.tries = append(c.arrowReturns.tries, statement)
			}
		}
	}
	for i, writes := range effects.writes {
		for declaration, cause := range writes {
			if _, exists := c.capturedWrites[i][declaration]; !exists {
				c.capturedWrites[i][declaration] = cause
			}
		}
	}
	for i, span := range effects.memberWrites {
		if c.capturedMemberWrites[i].Start.Line == 0 {
			c.capturedMemberWrites[i] = span
		}
	}
}

func (c *Checker) loopTargetScopeCount(index int, loop bool) int {
	for _, target := range c.branchFlowTargets {
		if loop && target.loopIndex == index || !loop && target.breakIndex == index {
			return target.scopeBase
		}
	}
	return len(c.scopes)
}

func appendDistinctLoopFlow(edges []nullableFlowSnapshot, flow nullableFlowSnapshot, scopeCount int) []nullableFlowSnapshot {
	// Branch-local bindings have already had their Task exit check. A loop,
	// switch or finally destination cannot observe those lexical scopes.
	if len(flow.scopes) > scopeCount {
		flow.scopes = flow.scopes[:scopeCount]
	}
	for _, existing := range edges {
		if reflect.DeepEqual(existing, flow) {
			return edges
		}
	}
	// finally applies Task effects in place. Never let that mutate a cached
	// return obligation (or a summary shared with an enclosing loop).
	flow.scopes = cloneValueScopes(flow.scopes)
	flow.members = cloneMemberFlow(flow.members)
	return append(edges, flow)
}
