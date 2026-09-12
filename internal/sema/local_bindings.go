package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkLocalBinding(stmt *ast.VariableDecl) {
	declared := Type{Kind: Invalid, Name: "<inferred>"}
	if stmt.Type.IsSpecified() {
		declared = c.resolveType(stmt.Type)
	}
	arrow, isArrow := stmt.Value.(*ast.ArrowExpr)
	if isArrow {
		declared = c.arrowBindingSignature(arrow, declared)
		if c.scopes[len(c.scopes)-1][stmt.Name].declaration != stmt {
			c.declareLocal(stmt.Name, declared, stmt.Constant, stmt, stmt.Span)
		}
		scope := c.scopes[len(c.scopes)-1]
		symbol := scope[stmt.Name]
		if symbol.declaration == stmt {
			symbol.initializingArrow = true
			scope[stmt.Name] = symbol
		}
	}
	var value Type
	if propagated, ok := stmt.Value.(*ast.PropagateExpr); ok {
		value = c.checkPropagateExpression(propagated)
	} else {
		value = c.checkExpressionExpectedSlot(&stmt.Value, declared)
	}
	if !stmt.Type.IsSpecified() {
		declared = c.inferredVariableType(value, stmt.Value.GetSpan())
		if !stmt.Constant || !numericInitializerEmitsConstant(stmt.Value) {
			c.checkNumericMaterialization(stmt.Value, declared)
		}
	}
	if declared.Kind == Void {
		c.report(stmt.Type.Span, "variables cannot have type void")
	}
	c.rejectResultValueType(declared, stmt.Type.Span, "variables")
	c.requireAssignable(declared, value, stmt.Value.GetSpan())
	stmt.ResolvedType = typeRefFromType(declared, stmt.Span)
	if isArrow {
		scope := c.scopes[len(c.scopes)-1]
		symbol := scope[stmt.Name]
		if symbol.declaration == stmt {
			symbol.typeInfo, symbol.declaredType = declared, declared
			symbol.initializingArrow = false
			scope[stmt.Name] = symbol
		}
	} else {
		c.declareLocal(stmt.Name, declared, stmt.Constant, stmt, stmt.Span)
	}
	c.updateIdentifierFlow(stmt.Name, stmt.NameSpan, value)
}

func (c *Checker) recordLocalArrowReference(symbol valueSymbol, name string, span source.Span) {
	if !symbol.initializingArrow || symbol.declaration == nil {
		return
	}
	symbol.declaration.RecursiveBinding = true
	if c.classes[name] != nil || c.structs[name] != nil || c.interfaces[name] != nil || c.nativeTypes[name] != nil || c.enums[name] != nil || c.lookupGoPackage(span.Path, name) != nil {
		c.report(span, fmt.Sprintf("recursive or forward local arrow %q conflicts with a type or Go package name; rename the local binding", name))
	}
	for _, scope := range c.typeParameterScopes {
		if _, exists := scope[name]; exists {
			c.report(span, fmt.Sprintf("recursive or forward local arrow %q conflicts with a type parameter; rename the local binding", name))
			break
		}
	}
	if symbol.typeInfo.Kind == Invalid {
		c.report(span, fmt.Sprintf("arrow function %q needs an explicit return type for recursive or forward local references", name))
	}
}

func (c *Checker) predeclareLocalArrowGroup(group []*ast.VariableDecl) {
	for _, declaration := range group {
		declared := Type{Kind: Invalid, Name: "<inferred>"}
		if declaration.Type.IsSpecified() {
			declared = c.resolveType(declaration.Type)
		}
		declared = c.arrowBindingSignature(declaration.Value.(*ast.ArrowExpr), declared)
		c.declareLocal(declaration.Name, declared, declaration.Constant, declaration, declaration.Span)
		scope := c.scopes[len(c.scopes)-1]
		symbol := scope[declaration.Name]
		if symbol.declaration == declaration {
			symbol.initializingArrow = true
			scope[declaration.Name] = symbol
		}
	}
}
