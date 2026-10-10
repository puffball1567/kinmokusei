package sema

import "github.com/puffball1567/kinmokusei/internal/source"

// Nullable facts, loop exits and reachability belong to the body being checked,
// not the body that happens to construct its closure. In particular, an arrow
// inside unreachable outer code still needs full checks of its own reachable
// paths. Keep this boundary separate from return/control-transfer state and
// from receiver/type-parameter access, which closures deliberately retain.
type callableFlowState struct {
	loopFlowContexts    []loopFlowContext
	breakFlowContexts   []breakFlowContext
	suppressFlowEffects int
	memberFlow          map[memberFlowKey]memberFlowState
	memberTypes         map[memberFlowKey]Type
}

func (c *Checker) enterCallableFlow() callableFlowState {
	previous := callableFlowState{
		loopFlowContexts: c.loopFlowContexts, breakFlowContexts: c.breakFlowContexts,
		suppressFlowEffects: c.suppressFlowEffects,
		memberFlow:          c.memberFlow, memberTypes: c.memberTypes,
	}
	c.loopFlowContexts, c.breakFlowContexts = nil, nil
	c.suppressFlowEffects = 0
	c.memberFlow = map[memberFlowKey]memberFlowState{}
	c.memberTypes = map[memberFlowKey]Type{}
	return previous
}

func (c *Checker) leaveCallableFlow(previous callableFlowState) {
	c.loopFlowContexts, c.breakFlowContexts = previous.loopFlowContexts, previous.breakFlowContexts
	c.suppressFlowEffects = previous.suppressFlowEffects
	c.memberFlow, c.memberTypes = previous.memberFlow, previous.memberTypes
}

type closureLexicalContext struct {
	outerFlow   nullableFlowSnapshot
	outerBody   callableFlowState
	captureBase int
}

type closureEffects struct {
	writes      map[source.Span]source.Span
	memberWrite source.Span
}

func (c *Checker) enterClosureLexical() closureLexicalContext {
	context := closureLexicalContext{
		outerFlow: c.snapshotNullableFlow(), captureBase: len(c.callableScopeBases),
	}
	memberRoots := map[source.Span]bool{}
	if len(c.capturedMemberRoots) != 0 {
		for declaration := range c.capturedMemberRoots[len(c.capturedMemberRoots)-1] {
			memberRoots[declaration] = true
		}
	}
	for key, state := range context.outerFlow.members {
		if state.nonNull {
			memberRoots[key.root] = true
		}
	}
	context.outerBody = c.enterCallableFlow()
	c.scopes = cloneValueScopes(context.outerFlow.scopes)
	for _, scope := range c.scopes {
		for name, symbol := range scope {
			// A mutable capture may change between closure creation and invocation.
			if !symbol.constant && symbol.declaredType.Kind == Nullable {
				symbol.typeInfo = symbol.declaredType
				scope[name] = symbol
			}
		}
	}
	c.callableScopeBases = append(c.callableScopeBases, len(c.scopes))
	c.capturedWrites = append(c.capturedWrites, map[source.Span]source.Span{})
	c.capturedMemberWrites = append(c.capturedMemberWrites, source.Span{})
	c.capturedMemberRoots = append(c.capturedMemberRoots, memberRoots)
	c.pushScope()
	return context
}

// Restore the enclosing environment before replaying the capture summary.
// Lexical declarations and local nullable facts never escape a closure check;
// only its potential writes are published at the declaration's source position.
func (c *Checker) leaveClosureLexical(context closureLexicalContext) closureEffects {
	c.popScope()
	effects := closureEffects{
		writes:      c.capturedWrites[context.captureBase],
		memberWrite: c.capturedMemberWrites[context.captureBase],
	}
	c.callableScopeBases = c.callableScopeBases[:context.captureBase]
	c.capturedWrites = c.capturedWrites[:context.captureBase]
	c.capturedMemberWrites = c.capturedMemberWrites[:context.captureBase]
	c.capturedMemberRoots = c.capturedMemberRoots[:context.captureBase]
	c.scopes = cloneValueScopes(context.outerFlow.scopes)
	c.leaveCallableFlow(context.outerBody)
	return effects
}

func (c *Checker) publishClosureEffects(effects closureEffects, span source.Span) {
	for declaration, cause := range effects.writes {
		c.markDeclarationEscaped(declaration, span, "a closure that can mutate it")
		for index, scope := range c.scopes {
			for _, symbol := range scope {
				if symbol.declarationSpan == declaration {
					c.recordCapturedWrite(index, declaration, cause)
				}
			}
		}
	}
	if effects.memberWrite.Start.Line != 0 {
		c.invalidateAllMemberFacts(span, "a closure with possible member mutation")
		c.recordMemberWrite(span)
	}
}
