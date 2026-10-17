package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Checking an instantiation operand shares call binding precedence, without
// treating a bare generic function as a runtime value or invoking its body.
func (c *Checker) genericValueOperand(expression ast.Expression) Type {
	if name, ok := expression.(*ast.IdentifierExpr); ok {
		if _, exists := c.lookupValue(name.Name, name.Span); !exists {
			if fn, exists := c.functions[name.Name]; exists && c.isTopLevelAllowed(name.Span, name.Name) {
				c.recordGlobalDependency(name.Name)
				name.ResolvedDeclaration = fn.declarationSpan
				return callableTypeForFunction(fn)
			}
			if imported, exists := c.lookupNamedGoImport(name.Name, name.Span); exists {
				return c.checkNamedGoIdentifier(name, imported)
			}
		}
	}
	previous := c.directCallCallee
	c.directCallCallee = expression
	defer func() { c.directCallCallee = previous }()
	return c.checkExpression(expression)
}

func (c *Checker) checkGenericFunctionValue(expr *ast.IndexExpr, callable Type) Type {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	if callable.Kind == Invalid {
		return invalid
	}
	if callable.Kind != Function || !callable.Generic || callable.Result == nil {
		c.report(expr.Span, "explicit function instantiation requires a generic function")
		return invalid
	}
	if member, ok := expr.Object.(*ast.MemberExpr); ok && member.GenericMethod {
		c.report(expr.Span, "instantiated generic method values are not yet supported; call the method directly")
		return invalid
	}
	var instantiated Type
	if len(callable.TypeParameters) != 0 {
		if len(expr.TypeArguments) != len(callable.TypeParameters) {
			c.report(expr.Span, fmt.Sprintf("generic function value requires all %d type arguments, got %d", len(callable.TypeParameters), len(expr.TypeArguments)))
			return invalid
		}
		bindings := c.nativeFunctionTypeBindings(callable, expr.TypeArguments)
		arguments := make([]Type, len(callable.TypeParameters))
		for i, parameter := range callable.TypeParameters {
			arguments[i] = bindings[parameter.GoType]
			if arguments[i].Kind == Invalid {
				return invalid
			}
		}
		if !c.validateNativeTypeArguments(callable.TypeParameters, arguments, expr.TypeArguments, expr.Span, "generic function value") {
			return invalid
		}
		instantiated = substituteNativeTypeParameters(callable, bindings)
		instantiated.Generic, instantiated.TypeParameters, instantiated.GoType = false, nil, nil
	} else {
		signature, ok := callable.GoType.(*gotypes.Signature)
		if !ok || signature.TypeParams() == nil {
			c.report(expr.Span, "generic function value has invalid Go type information")
			return invalid
		}
		if len(expr.TypeArguments) != signature.TypeParams().Len() {
			c.report(expr.Span, fmt.Sprintf("generic Go function value requires all %d type arguments, got %d", signature.TypeParams().Len(), len(expr.TypeArguments)))
			return invalid
		}
		arguments, ok := c.explicitGoFunctionTypeArguments(expr.TypeArguments)
		if !ok {
			return invalid
		}
		applied, err := gotypes.Instantiate(nil, signature, arguments, true)
		if err != nil {
			c.report(expr.Span, fmt.Sprintf("cannot instantiate generic Go function value: %v", err))
			return invalid
		}
		instantiated, err = kinmokuseiFunctionFromGo(applied.(*gotypes.Signature))
		if err != nil {
			c.report(expr.Span, fmt.Sprintf("instantiated Go function value is not supported: %v", err))
			return invalid
		}
		if c.containsVerifiedText(instantiated, nil) {
			c.report(expr.Span, "Go generic function values cannot preserve a verified native text contract; use bstring/raw storage")
			return invalid
		}
	}
	expr.Instantiated = true
	expr.ResolvedType = typeRefFromType(instantiated, expr.Span)
	return instantiated
}

func (c *Checker) nativeFunctionTypeBindings(callable Type, refs []ast.TypeRef) nativeTypeBindings {
	bindings := make(nativeTypeBindings, len(callable.TypeParameters))
	for _, parameter := range callable.TypeParameters {
		bindings[parameter.GoType] = Type{Kind: Invalid}
	}
	for index, ref := range refs {
		resolved := c.resolveType(ref)
		if resolved.Kind == Invalid {
			continue
		}
		if !validNativeTypeArgument(resolved) {
			c.report(ref.Span, fmt.Sprintf("type %s cannot be used as a generic function type argument", resolved.String()))
			continue
		}
		if index < len(callable.TypeParameters) {
			bindings[callable.TypeParameters[index].GoType] = resolved
		}
	}
	return bindings
}

func (c *Checker) explicitGoFunctionTypeArguments(refs []ast.TypeRef) ([]gotypes.Type, bool) {
	arguments := make([]gotypes.Type, len(refs))
	for i, ref := range refs {
		resolved := c.resolveType(ref)
		if resolved.Kind == Invalid {
			return nil, false
		}
		argument, ok := goTypeOf(resolved)
		if !ok {
			c.report(ref.Span, fmt.Sprintf("type argument %s cannot be represented as a Go type", resolved.String()))
			return nil, false
		}
		arguments[i] = argument
	}
	return arguments, true
}
