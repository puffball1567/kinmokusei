package sema

import (
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type loopFlowContext struct {
	continues []nullableFlowSnapshot
	breaks    []nullableFlowSnapshot
}

type breakFlowContext struct {
	breaks []nullableFlowSnapshot
}

type memberFlowKey struct {
	root source.Span
	path string
}

type memberFlowState struct {
	declaredType      Type
	nonNullType       Type
	nonNull           bool
	invalidated       source.Span
	invalidationCause string
}

type nullableFlowSnapshot struct {
	scopes  []map[string]valueSymbol
	members map[memberFlowKey]memberFlowState
}

type nullableNarrowing struct {
	name            string
	scopeIndex      int
	symbol          valueSymbol
	member          memberFlowKey
	memberType      Type
	isMember        bool
	nonNullType     Type
	nonNullWhenTrue bool
}

func (c *Checker) nullableConditionNarrowing(condition ast.Expression) (nullableNarrowing, bool) {
	binary, ok := condition.(*ast.BinaryExpr)
	if !ok || (binary.Operator != "==" && binary.Operator != "===" && binary.Operator != "!=" && binary.Operator != "!==") {
		return nullableNarrowing{}, false
	}
	operand, literal := nullableComparisonOperands(binary.Left, binary.Right)
	if operand == nil || literal == nil || literal.Kind != ast.NullLiteral {
		return nullableNarrowing{}, false
	}
	if member, ok := operand.(*ast.MemberExpr); ok {
		key, stable := c.stableMemberFlowKey(member)
		declared, typed := c.memberTypes[key]
		if !stable || !typed || declared.Kind != Nullable || declared.Element == nil {
			return nullableNarrowing{}, false
		}
		return nullableNarrowing{
			member: key, memberType: declared, isMember: true, nonNullType: *declared.Element,
			nonNullWhenTrue: binary.Operator == "!=" || binary.Operator == "!==",
		}, true
	}
	identifier, ok := operand.(*ast.IdentifierExpr)
	if !ok {
		return nullableNarrowing{}, false
	}
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][identifier.Name]
		if !exists {
			continue
		}
		if (!symbol.constant && symbol.flowEscaped) || symbol.typeInfo.Kind != Nullable || symbol.typeInfo.Element == nil {
			return nullableNarrowing{}, false
		}
		return nullableNarrowing{
			name: identifier.Name, scopeIndex: index, symbol: symbol, nonNullType: *symbol.typeInfo.Element,
			nonNullWhenTrue: binary.Operator == "!=" || binary.Operator == "!==",
		}, true
	}
	return nullableNarrowing{}, false
}

func nullableComparisonOperands(left, right ast.Expression) (ast.Expression, *ast.LiteralExpr) {
	if _, ok := left.(*ast.IdentifierExpr); ok {
		if literal, ok := right.(*ast.LiteralExpr); ok {
			return left, literal
		}
	}
	if _, ok := left.(*ast.MemberExpr); ok {
		if literal, ok := right.(*ast.LiteralExpr); ok {
			return left, literal
		}
	}
	if _, ok := right.(*ast.IdentifierExpr); ok {
		if literal, ok := left.(*ast.LiteralExpr); ok {
			return right, literal
		}
	}
	if _, ok := right.(*ast.MemberExpr); ok {
		if literal, ok := left.(*ast.LiteralExpr); ok {
			return right, literal
		}
	}
	return nil, nil
}

func (c *Checker) applyNarrowing(narrowing nullableNarrowing) {
	if narrowing.isMember {
		c.memberFlow[narrowing.member] = memberFlowState{
			declaredType: narrowing.memberType, nonNullType: narrowing.nonNullType, nonNull: true,
		}
		return
	}
	if narrowing.scopeIndex < 0 || narrowing.scopeIndex >= len(c.scopes) {
		return
	}
	symbol, exists := c.scopes[narrowing.scopeIndex][narrowing.name]
	if !exists || symbol.declarationSpan != narrowing.symbol.declarationSpan {
		return
	}
	symbol.typeInfo = narrowing.nonNullType
	symbol.flowInvalidated = source.Span{}
	symbol.flowInvalidationCause = ""
	c.scopes[narrowing.scopeIndex][narrowing.name] = symbol
}

func (c *Checker) checkStatementBranch(statement ast.Statement) {
	if block, ok := statement.(*ast.BlockStmt); ok {
		c.checkBlock(block, true)
	} else {
		c.checkStatement(statement)
	}
}

func cloneValueScopes(scopes []map[string]valueSymbol) []map[string]valueSymbol {
	cloned := make([]map[string]valueSymbol, len(scopes))
	for index, scope := range scopes {
		cloned[index] = make(map[string]valueSymbol, len(scope))
		for name, symbol := range scope {
			cloned[index][name] = symbol
		}
	}
	return cloned
}

func cloneMemberFlow(flow map[memberFlowKey]memberFlowState) map[memberFlowKey]memberFlowState {
	cloned := make(map[memberFlowKey]memberFlowState, len(flow))
	for key, state := range flow {
		cloned[key] = state
	}
	return cloned
}

func (c *Checker) snapshotNullableFlow() nullableFlowSnapshot {
	return nullableFlowSnapshot{scopes: cloneValueScopes(c.scopes), members: cloneMemberFlow(c.memberFlow)}
}

func (c *Checker) restoreNullableFlow(snapshot nullableFlowSnapshot) {
	c.scopes = cloneValueScopes(snapshot.scopes)
	c.memberFlow = cloneMemberFlow(snapshot.members)
}

func (c *Checker) mergeNullableFlow(entry nullableFlowSnapshot, continuing ...nullableFlowSnapshot) nullableFlowSnapshot {
	branches := make([][]map[string]valueSymbol, len(continuing))
	for index, branch := range continuing {
		branches[index] = branch.scopes
	}
	merged := nullableFlowSnapshot{
		scopes:  c.mergeValueScopes(entry.scopes, branches...),
		members: cloneMemberFlow(entry.members),
	}
	if len(continuing) == 0 {
		return merged
	}
	keys := map[memberFlowKey]bool{}
	for key := range entry.members {
		keys[key] = true
	}
	for _, branch := range continuing {
		for key := range branch.members {
			keys[key] = true
		}
	}
	for key := range keys {
		var common memberFlowState
		nonNullOnEveryPath := true
		for index, branch := range continuing {
			state, exists := branch.members[key]
			if !exists || !state.nonNull {
				nonNullOnEveryPath = false
				continue
			}
			if index == 0 || !common.nonNull {
				common = state
			}
		}
		if nonNullOnEveryPath {
			common.nonNull = true
			common.invalidated = source.Span{}
			common.invalidationCause = ""
			merged.members[key] = common
			continue
		}
		state := entry.members[key]
		state.nonNull = false
		for _, branch := range continuing {
			candidate, exists := branch.members[key]
			if exists && candidate.invalidated.Start.Line != 0 {
				state = candidate
				state.nonNull = false
				break
			}
		}
		if state.declaredType.Kind != Invalid || state.invalidated.Start.Line != 0 {
			merged.members[key] = state
		} else {
			delete(merged.members, key)
		}
	}
	return merged
}

func (c *Checker) mergeValueScopes(entry []map[string]valueSymbol, continuing ...[]map[string]valueSymbol) []map[string]valueSymbol {
	merged := cloneValueScopes(entry)
	if len(continuing) == 0 {
		// The joined point is unreachable. Every terminating edge has already
		// reported pending tasks at its return, so no task remains live here.
		for _, scope := range merged {
			for name, symbol := range scope {
				if symbol.taskState != taskNotTracked {
					symbol.taskState = taskConsumed
					scope[name] = symbol
				}
			}
		}
		return merged
	}
	for scopeIndex, scope := range merged {
		for name, symbol := range scope {
			if symbol.taskState != taskNotTracked {
				state := uint8(taskNotTracked)
				for _, branch := range continuing {
					candidateState := uint8(taskPending)
					if scopeIndex < len(branch) {
						if candidate, exists := branch[scopeIndex][name]; exists && candidate.declarationSpan == symbol.declarationSpan {
							candidateState = candidate.taskState
						}
					}
					if state == taskNotTracked {
						state = candidateState
					} else if state != candidateState {
						state = taskMaybeConsumed
					}
				}
				symbol.taskState = state
			}
			declared := symbol.declaredType
			escapedOnAnyPath := symbol.flowEscaped
			for _, branch := range continuing {
				if scopeIndex < len(branch) {
					candidate, exists := branch[scopeIndex][name]
					if exists && candidate.declarationSpan == symbol.declarationSpan && candidate.flowEscaped {
						escapedOnAnyPath = true
					}
				}
			}
			symbol.flowEscaped = escapedOnAnyPath
			if declared.Kind != Nullable || declared.Element == nil {
				symbol.typeInfo = declared
				scope[name] = symbol
				continue
			}
			nonNullOnEveryPath := !escapedOnAnyPath
			for _, branch := range continuing {
				if scopeIndex >= len(branch) {
					nonNullOnEveryPath = false
					break
				}
				branchSymbol, exists := branch[scopeIndex][name]
				if !exists || branchSymbol.declarationSpan != symbol.declarationSpan || branchSymbol.typeInfo.Kind == Nullable || branchSymbol.typeInfo.Kind == Null || !c.isAssignable(*declared.Element, branchSymbol.typeInfo) {
					nonNullOnEveryPath = false
					break
				}
			}
			if nonNullOnEveryPath {
				symbol.typeInfo = *declared.Element
				symbol.flowInvalidated = source.Span{}
				symbol.flowInvalidationCause = ""
			} else {
				symbol.typeInfo = declared
				for _, branch := range continuing {
					if scopeIndex < len(branch) {
						candidate, exists := branch[scopeIndex][name]
						if exists && candidate.declarationSpan == symbol.declarationSpan && candidate.flowInvalidated.Start.Line != 0 {
							symbol.flowInvalidated = candidate.flowInvalidated
							symbol.flowInvalidationCause = candidate.flowInvalidationCause
							break
						}
					}
				}
			}
			scope[name] = symbol
		}
	}
	return merged
}

func (c *Checker) checkLoopFixedPoint(entry nullableFlowSnapshot, checkIteration func() (nullableFlowSnapshot, bool)) {
	header := nullableFlowSnapshot{scopes: cloneValueScopes(entry.scopes), members: cloneMemberFlow(entry.members)}
	limit := nullableFlowSymbolCount(entry.scopes)*2 + len(entry.members)*2 + 4
	for iteration := 0; iteration < limit; iteration++ {
		diagnosticStart := len(c.diagnostics)
		c.restoreNullableFlow(header)
		c.loopFlowContexts = append(c.loopFlowContexts, loopFlowContext{})
		backedge, fallsThrough := checkIteration()
		flow := c.loopFlowContexts[len(c.loopFlowContexts)-1]
		c.loopFlowContexts = c.loopFlowContexts[:len(c.loopFlowContexts)-1]
		backedges := append([]nullableFlowSnapshot(nil), flow.continues...)
		if fallsThrough {
			backedges = append(backedges, backedge)
		}
		next := c.mergeNullableFlow(entry, append([]nullableFlowSnapshot{entry}, backedges...)...)
		if sameNullableFlowSnapshot(header, next) {
			exits := append([]nullableFlowSnapshot{next}, flow.breaks...)
			c.restoreNullableFlow(c.mergeNullableFlow(entry, exits...))
			return
		}
		c.diagnostics = c.diagnostics[:diagnosticStart]
		header = next
	}

	// The nullable lattice is finite and the transfer is monotone, so this is a
	// defensive fallback only. Check once at the most conservative state reached
	// instead of accepting a body based on an earlier iteration.
	c.restoreNullableFlow(header)
	c.loopFlowContexts = append(c.loopFlowContexts, loopFlowContext{})
	backedge, fallsThrough := checkIteration()
	flow := c.loopFlowContexts[len(c.loopFlowContexts)-1]
	c.loopFlowContexts = c.loopFlowContexts[:len(c.loopFlowContexts)-1]
	backedges := append([]nullableFlowSnapshot(nil), flow.continues...)
	if fallsThrough {
		backedges = append(backedges, backedge)
	}
	next := c.mergeNullableFlow(entry, append([]nullableFlowSnapshot{entry}, backedges...)...)
	exits := append([]nullableFlowSnapshot{next}, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entry, exits...))
}

func nullableFlowSymbolCount(scopes []map[string]valueSymbol) int {
	count := 0
	for _, scope := range scopes {
		for _, symbol := range scope {
			if symbol.declaredType.Kind == Nullable || symbol.taskState != taskNotTracked {
				count++
			}
		}
	}
	return count
}

func sameNullableFlow(left, right []map[string]valueSymbol) bool {
	if len(left) != len(right) {
		return false
	}
	for scopeIndex, leftScope := range left {
		rightScope := right[scopeIndex]
		for name, leftSymbol := range leftScope {
			rightSymbol, exists := rightScope[name]
			if leftSymbol.taskState != taskNotTracked {
				if !exists || rightSymbol.declarationSpan != leftSymbol.declarationSpan || rightSymbol.taskState != leftSymbol.taskState {
					return false
				}
			}
			if leftSymbol.declaredType.Kind != Nullable {
				continue
			}
			if !exists || rightSymbol.declarationSpan != leftSymbol.declarationSpan {
				return false
			}
			if leftSymbol.typeInfo.Kind != rightSymbol.typeInfo.Kind || leftSymbol.flowEscaped != rightSymbol.flowEscaped {
				return false
			}
		}
	}
	return true
}

func sameNullableFlowSnapshot(left, right nullableFlowSnapshot) bool {
	if !sameNullableFlow(left.scopes, right.scopes) || len(left.members) != len(right.members) {
		return false
	}
	for key, leftState := range left.members {
		rightState, exists := right.members[key]
		if !exists || leftState.nonNull != rightState.nonNull || leftState.invalidated != rightState.invalidated || leftState.invalidationCause != rightState.invalidationCause {
			return false
		}
	}
	return true
}

func (c *Checker) updateAssignmentFlow(target ast.Expression, value Type) {
	if identifier, ok := target.(*ast.IdentifierExpr); ok {
		c.invalidateMemberFactsRootedAt(identifier, target.GetSpan(), "an assignment to its receiver")
		c.updateIdentifierFlow(identifier.Name, identifier.Span, value)
		return
	}
	if member, ok := target.(*ast.MemberExpr); ok {
		key, stable := c.stableMemberFlowKey(member)
		declared, typed := c.memberTypes[key]
		c.recordMemberWrite(target.GetSpan())
		c.invalidateAllMemberFacts(target.GetSpan(), "a possibly aliased field assignment")
		if stable && typed && declared.Kind == Nullable && declared.Element != nil && value.Kind != Nullable && value.Kind != Null && value.Kind != Nil && value.Kind != Invalid && c.isAssignable(*declared.Element, value) {
			c.memberFlow[key] = memberFlowState{declaredType: declared, nonNullType: *declared.Element, nonNull: true}
		}
	}
}

func (c *Checker) updateIdentifierFlow(name string, span source.Span, value Type) {
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][name]
		if !exists {
			continue
		}
		if symbol.constant && symbol.declarationSpan != span {
			return
		}
		declared := symbol.declaredType
		if declared.Kind == Invalid {
			declared = symbol.typeInfo
		}
		if !c.isAssignable(declared, value) {
			return
		}
		if symbol.declarationSpan != span {
			c.invalidateMemberFactsForDeclaration(symbol.declarationSpan, span, "an assignment to its receiver")
		}
		c.recordCapturedWrite(index, symbol.declarationSpan, span)
		symbol.typeInfo = declared
		if declared.Kind == Nullable && declared.Element != nil && !symbol.flowEscaped && value.Kind != Nullable && value.Kind != Null && value.Kind != Nil && value.Kind != Invalid && c.isAssignable(*declared.Element, value) {
			symbol.typeInfo = *declared.Element
			symbol.flowInvalidated = source.Span{}
			symbol.flowInvalidationCause = ""
		} else if declared.Kind == Nullable && symbol.declarationSpan != span {
			symbol.flowInvalidated = span
			symbol.flowInvalidationCause = "an assignment"
		}
		c.scopes[index][name] = symbol
		return
	}
}

func (c *Checker) recordCapturedWrite(scopeIndex int, declaration, cause source.Span) {
	if c.suppressFlowEffects != 0 {
		return
	}
	for index, base := range c.callableScopeBases {
		if scopeIndex >= base {
			continue
		}
		if _, exists := c.capturedWrites[index][declaration]; !exists {
			c.capturedWrites[index][declaration] = cause
		}
	}
}

func (c *Checker) markIdentifierEscaped(name string, span source.Span, cause string) {
	for index := len(c.scopes) - 1; index >= 0; index-- {
		symbol, exists := c.scopes[index][name]
		if !exists {
			continue
		}
		c.recordCapturedWrite(index, symbol.declarationSpan, span)
		symbol.flowEscaped = true
		symbol.typeInfo = symbol.declaredType
		symbol.flowInvalidated = span
		symbol.flowInvalidationCause = cause
		c.scopes[index][name] = symbol
		return
	}
}

func (c *Checker) markDeclarationEscaped(declaration, span source.Span, cause string) {
	for scopeIndex := len(c.scopes) - 1; scopeIndex >= 0; scopeIndex-- {
		for name, symbol := range c.scopes[scopeIndex] {
			if symbol.declarationSpan != declaration {
				continue
			}
			symbol.flowEscaped = true
			symbol.typeInfo = symbol.declaredType
			symbol.flowInvalidated = span
			symbol.flowInvalidationCause = cause
			c.scopes[scopeIndex][name] = symbol
			return
		}
	}
}

func (c *Checker) flowInvalidation(expression ast.Expression) (source.Span, string) {
	if member, ok := expression.(*ast.MemberExpr); ok {
		if key, stable := c.stableMemberFlowKey(member); stable {
			state := c.memberFlow[key]
			return state.invalidated, state.invalidationCause
		}
	}
	identifier, ok := expression.(*ast.IdentifierExpr)
	if !ok {
		return source.Span{}, ""
	}
	for index := len(c.scopes) - 1; index >= 0; index-- {
		if symbol, exists := c.scopes[index][identifier.Name]; exists {
			return symbol.flowInvalidated, symbol.flowInvalidationCause
		}
	}
	return source.Span{}, ""
}

func (c *Checker) stableMemberFlowKey(expression *ast.MemberExpr) (memberFlowKey, bool) {
	if expression == nil || expression.Go || expression.GoField || !expression.Addressable || expression.ResolvedDeclaration.Start.Line == 0 {
		return memberFlowKey{}, false
	}
	root, path, ok := c.stableMemberFlowPath(expression)
	if !ok {
		return memberFlowKey{}, false
	}
	return memberFlowKey{root: root, path: path}, true
}

func (c *Checker) stableMemberFlowPath(expression ast.Expression) (source.Span, string, bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		for index := len(c.scopes) - 1; index >= 0; index-- {
			symbol, exists := c.scopes[index][expression.Name]
			if !exists {
				continue
			}
			expression.ResolvedDeclaration = symbol.declarationSpan
			return symbol.declarationSpan, "", true
		}
		return source.Span{}, "", false
	case *ast.MemberExpr:
		if expression.Go || expression.GoField || !expression.Addressable || expression.ResolvedDeclaration.Start.Line == 0 {
			return source.Span{}, "", false
		}
		root, prefix, ok := c.stableMemberFlowPath(expression.Object)
		if !ok {
			return source.Span{}, "", false
		}
		field := expression.ResolvedDeclaration
		segment := field.Path + ":" + strconv.Itoa(field.Start.Offset) + ":" + strconv.Itoa(field.End.Offset)
		if prefix != "" {
			segment = prefix + "/" + segment
		}
		return root, segment, true
	default:
		return source.Span{}, "", false
	}
}

func (c *Checker) invalidateAllMemberFacts(span source.Span, cause string) {
	if c.suppressFlowEffects != 0 {
		return
	}
	for key, state := range c.memberFlow {
		if !state.nonNull {
			continue
		}
		state.nonNull = false
		state.invalidated = span
		state.invalidationCause = cause
		c.memberFlow[key] = state
	}
}

func (c *Checker) invalidateControlTransferFlow(span source.Span) {
	if c.suppressFlowEffects != 0 {
		return
	}
	for scopeIndex, scope := range c.scopes {
		for name, symbol := range scope {
			if symbol.constant || symbol.declaredType.Kind != Nullable {
				continue
			}
			symbol.typeInfo = symbol.declaredType
			symbol.flowInvalidated = span
			symbol.flowInvalidationCause = "an arbitrary control transfer"
			c.scopes[scopeIndex][name] = symbol
		}
	}
	c.invalidateAllMemberFacts(span, "an arbitrary control transfer")
}

func (c *Checker) invalidateMemberFactsRootedAt(identifier *ast.IdentifierExpr, span source.Span, cause string) {
	var declaration source.Span
	for index := len(c.scopes) - 1; index >= 0; index-- {
		if symbol, exists := c.scopes[index][identifier.Name]; exists {
			declaration = symbol.declarationSpan
			break
		}
	}
	if declaration.Start.Line == 0 {
		return
	}
	c.invalidateMemberFactsForDeclaration(declaration, span, cause)
}

func (c *Checker) invalidateMemberFactsForDeclaration(declaration, span source.Span, cause string) {
	if c.suppressFlowEffects != 0 {
		return
	}
	if len(c.capturedMemberRoots) != 0 && c.capturedMemberRoots[len(c.capturedMemberRoots)-1][declaration] {
		c.recordMemberWrite(span)
	}
	for key, state := range c.memberFlow {
		if key.root != declaration || !state.nonNull {
			continue
		}
		state.nonNull = false
		state.invalidated = span
		state.invalidationCause = cause
		c.memberFlow[key] = state
	}
}

func (c *Checker) recordMemberWrite(span source.Span) {
	if c.suppressFlowEffects != 0 || len(c.capturedMemberWrites) == 0 {
		return
	}
	index := len(c.capturedMemberWrites) - 1
	if c.capturedMemberWrites[index].Start.Line == 0 {
		c.capturedMemberWrites[index] = span
	}
}

func (c *Checker) invalidateMemberWriteTarget(target ast.Expression, span source.Span) {
	if _, ok := target.(*ast.MemberExpr); !ok {
		return
	}
	c.recordMemberWrite(span)
	c.invalidateAllMemberFacts(span, "a possibly aliased field update")
}
