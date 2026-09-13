package sema

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Peers share the lexical environment at group entry, never the environment of
// the body requesting their type. Snapshots retain declaration identities and
// generic/receiver context, while each body gets its own control/capture state.
type localArrowGroupContext struct{ checker Checker }

type localArrowInference struct {
	declaration               *ast.VariableDecl
	context                   *localArrowGroupContext
	checking, checked         bool
	symbol                    valueSymbol
	diagnostics               []diagnostic.Diagnostic
	capturedWrites            map[source.Span]source.Span
	memberWrite               source.Span
	usesTasks, usesExceptions bool
}

func resolveLocalArrowType(symbol valueSymbol) valueSymbol {
	inference := symbol.localArrowInference
	if symbol.typeInfo.Kind != Invalid || inference == nil {
		return symbol
	}
	inference.check()
	if inference.checked {
		symbol.typeInfo = inference.symbol.typeInfo
		symbol.declaredType = inference.symbol.declaredType
	}
	return symbol
}

func (inference *localArrowInference) check() {
	if inference.checking || inference.checked {
		return
	}
	inference.checking = true
	dependency := inference.context.checker
	dependency.scopes = cloneValueScopes(dependency.scopes)
	dependency.memberFlow = cloneMemberFlow(dependency.memberFlow)
	dependency.diagnostics = nil
	dependency.checkingLocalArrow = inference
	dependency.callableControlState = callableControlState{}
	dependency.directCallCallee = nil
	dependency.taskOperandDepth = 0
	dependency.callableScopeBases = nil
	dependency.capturedWrites = nil
	dependency.capturedMemberWrites = nil
	dependency.capturedMemberRoots = append([]map[source.Span]bool(nil), dependency.capturedMemberRoots...)
	dependency.typeParameterScopes = append([]map[string]Type(nil), dependency.typeParameterScopes...)
	dependency.loopFlowContexts = nil
	dependency.breakFlowContexts = nil
	dependency.checkLocalBinding(inference.declaration)
	inference.symbol = dependency.scopes[len(dependency.scopes)-1][inference.declaration.Name]
	inference.diagnostics = dependency.diagnostics
	inference.usesTasks, inference.usesExceptions = dependency.usesTasks, dependency.usesExceptions
	inference.checking, inference.checked = false, true
}

// Publish the checked signature and replay capture effects at the declaration's
// original position. On-demand type lookup must not reorder initialization or
// mutate the requesting peer's nullable flow and capture environment.
func (c *Checker) finishLocalArrowBinding(inference *localArrowInference) {
	inference.check()
	c.diagnostics = append(c.diagnostics, inference.diagnostics...)
	c.usesTasks = c.usesTasks || inference.usesTasks
	c.usesExceptions = c.usesExceptions || inference.usesExceptions
	decl := inference.declaration
	scope := c.scopes[len(c.scopes)-1]
	symbol := scope[decl.Name]
	symbol.typeInfo, symbol.declaredType = inference.symbol.typeInfo, inference.symbol.declaredType
	if symbol.flowEscaped {
		symbol.typeInfo = symbol.declaredType
	}
	symbol.initializingArrow = false
	scope[decl.Name] = symbol
	for declaration, cause := range inference.capturedWrites {
		c.markDeclarationEscaped(declaration, decl.Span, "a closure that can mutate it")
		for index, scope := range c.scopes {
			for _, captured := range scope {
				if captured.declarationSpan == declaration {
					c.recordCapturedWrite(index, declaration, cause)
				}
			}
		}
	}
	if inference.memberWrite.Start.Line != 0 {
		c.invalidateAllMemberFacts(decl.Span, "a closure with possible member mutation")
		c.recordMemberWrite(decl.Span)
	}
}
