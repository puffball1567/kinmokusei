package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

func clearReferenceRead(expression ast.Expression) {
	if slot := ast.ReferenceReadSlot(expression); slot != nil {
		*slot = nil
	}
}

// Shared Go storage can be changed without a source assignment. Validate each
// native reference as it is extracted, preserving copies of aggregates and
// reference identity without traversing arbitrary or cyclic object graphs.
func (c *Checker) checkReferenceRead(expression ast.Expression, value Type) {
	slot := ast.ReferenceReadSlot(expression)
	if slot == nil {
		return
	}
	*slot = nil
	if c.unevaluatedDepth != 0 || value.Generic || !c.requiresUniformNativeNonNull(value, nil) || c.assignmentProvesNonNull(value, expression) {
		return
	}
	if c.compilerOwnedReferenceCall(expression) {
		return
	}
	if member, ok := expression.(*ast.MemberExpr); ok && (member.BoundMethod || member.MethodExpressionReceiver != nil) {
		return
	}
	if identifier, ok := expression.(*ast.IdentifierExpr); ok {
		for i := len(c.scopes) - 1; i >= 0; i-- {
			if symbol, exists := c.scopes[i][identifier.Name]; exists {
				if c.localReferenceReadProven(identifier.Name, symbol) {
					return
				}
				break
			}
		}
	}
	ref := typeRefFromType(value, expression.GetSpan())
	ref.NonNullKinds = c.nonNullStorageKind(value)
	if value.Kind == TypeParameter {
		ref.NonNullKinds, _ = c.mixedNonNullKinds(value)
	}
	if ref.NonNullKinds == 0 {
		return
	}
	*slot = &ref
	c.usesReferenceReads = true
	if ref.NonNullKinds != 1 && ref.NonNullKinds != 4 || ref.TypeParameter {
		c.usesNonNullKinds = true
	}
}

// Concrete reference result contracts are enforced in compiler-owned bodies.
// An opaque generic T result or a mutable callback still needs a read check at
// its concrete use site; Go can provide or replace those values.
func (c *Checker) compilerOwnedReferenceCall(expression ast.Expression) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || call.Conversion || call.Builtin != ast.NotBuiltinCall {
		return false
	}
	var result *Type
	switch callee := call.Callee.(type) {
	case *ast.IdentifierExpr:
		if callee.GoMember != nil {
			return false
		}
		if fn, exists := c.functions[callee.Name]; exists && call.StaticCallee && fn.declarationSpan == callee.ResolvedDeclaration {
			result = &fn.result
		}
		if symbol, exists := c.globals[callee.Name]; exists && symbol.declaration != nil && symbol.declaration.FunctionBinding && symbol.declarationSpan == callee.ResolvedDeclaration {
			result = symbol.typeInfo.Result
		}
	case *ast.MemberExpr:
		if callee.Go || !callee.Static || !callee.BoundMethod {
			return false
		}
		if owner, ok := callee.Object.(*ast.IdentifierExpr); ok {
			if class := c.classes[owner.Name]; class != nil {
				if method, exists := class.methods[callee.Name]; exists {
					result = method.typeInfo.Result
				}
			}
		}
	}
	return result != nil && c.requiresUniformNativeNonNull(*result, nil)
}

func (c *Checker) localReferenceReadProven(name string, symbol valueSymbol) bool {
	return !(symbol.presence.required && symbol.declaredType.Kind == Nullable) && symbol.rangeBinding == nil && symbol.typeSwitchCase == nil && symbol.catchClause == nil && !symbol.flowEscaped && (name != "this" || c.inConstructor || c.inFieldInitializer)
}
