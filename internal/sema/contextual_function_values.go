package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// A function-typed destination supplies equations for a generic function's
// parameters AND results. Only its own parameters are inference variables;
// captured caller parameters and source text/nullability contracts remain fixed.
func (c *Checker) checkContextualFunctionValue(slot *ast.Expression, expected Type) (Type, bool) {
	context := c.arrowContext(expected)
	if context.Kind != Function || context.Generic || context.Result == nil {
		return Type{}, false
	}
	var instantiation *ast.IndexExpr
	var operand ast.Expression
	switch expression := (*slot).(type) {
	case *ast.IdentifierExpr, *ast.MemberExpr:
		operand = expression
	case *ast.IndexExpr:
		if len(expression.TypeArguments) == 0 && len(expression.ResolvedTypeArguments) == 0 {
			return Type{}, false
		}
		switch expression.Object.(type) {
		case *ast.IdentifierExpr, *ast.MemberExpr:
		default:
			return Type{}, false
		}
		instantiation, operand = expression, expression.Object
	default:
		return Type{}, false
	}
	callable := c.genericValueOperand(operand)
	if callable.Kind != Function || !callable.Generic {
		// Ordinary selectors/identifiers were checked once, including their
		// effect/escape metadata. An index still needs its ordinary index check.
		return callable, instantiation == nil
	}
	if instantiation == nil {
		instantiation = &ast.IndexExpr{Object: operand, Span: operand.GetSpan()}
		*slot = instantiation
	}
	instantiation.ResolvedTypeArguments = nil
	parameters := callable.TypeParameters
	inference := callable
	if len(parameters) == 0 {
		if signature, ok := callable.GoType.(*gotypes.Signature); ok && signature.TypeParams() != nil {
			shape := gotypes.NewSignatureType(nil, nil, nil, signature.Params(), signature.Results(), signature.Variadic())
			var err error
			inference, err = kinmokuseiFunctionFromGo(shape)
			if err != nil {
				c.report(instantiation.Span, fmt.Sprintf("cannot infer generic Go function value: %v", err))
				return Type{Kind: Invalid, Name: "<invalid>"}, true
			}
			for i := 0; i < signature.TypeParams().Len(); i++ {
				parameter := signature.TypeParams().At(i)
				parameters = append(parameters, Type{Kind: TypeParameter, Name: parameter.Obj().Name(), GoType: parameter})
			}
		}
	}
	if len(instantiation.TypeArguments) >= len(parameters) {
		return c.checkGenericFunctionValue(instantiation, callable), true
	}
	inference.TypeParameters = parameters
	bindings := c.nativeFunctionTypeBindings(inference, instantiation.TypeArguments)
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	if err := c.inferNativeTypeArguments(inference, context, bindings); err != nil {
		c.report(instantiation.Span, fmt.Sprintf("cannot infer generic function value from destination type: %v", err))
		return invalid, true
	}
	c.inferNativeConstraintArguments(parameters, bindings)
	refs := make([]ast.TypeRef, len(parameters))
	for i, parameter := range parameters {
		argument := bindings[parameter.GoType]
		if argument.Kind == Invalid {
			c.report(instantiation.Span, fmt.Sprintf("cannot infer type argument %s for generic function value; provide explicit type arguments", parameter.Name))
			return invalid, true
		}
		if !typeNameableForEmission(argument) {
			c.report(instantiation.Span, "inferred generic function type argument contains a private Go type; use an explicit public function type or wrapper")
			return invalid, true
		}
		c.prepareGoTypeForEmission(&argument, instantiation.Span)
		refs[i] = typeRefFromType(argument, instantiation.Span)
	}
	instantiation.ResolvedTypeArguments = refs
	actual := c.checkGenericFunctionValue(instantiation, callable)
	c.checkReferenceRead(instantiation, actual)
	return actual, true
}

// Only declaration references may be deferred during higher-order inference.
// Reading a field or evaluating a receiver can change flow state and must stay
// in source order. Local/global value bindings also shadow declarations here.
func (c *Checker) deferredGenericFunctionValue(expression ast.Expression) bool {
	explicit := 0
	if index, ok := expression.(*ast.IndexExpr); ok {
		if len(index.TypeArguments) == 0 && len(index.ResolvedTypeArguments) == 0 {
			return false
		}
		explicit, expression = len(index.TypeArguments), index.Object
	}
	unshadowed := func(name string) bool {
		for _, scope := range c.scopes {
			if _, exists := scope[name]; exists {
				return false
			}
		}
		_, exists := c.globals[name]
		return !exists
	}
	var object gotypes.Object
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		if !unshadowed(expression.Name) {
			return false
		}
		if function, exists := c.functions[expression.Name]; exists && c.isTopLevelAllowed(expression.Span, expression.Name) {
			return explicit < len(function.typeParameters)
		}
		if imported, exists := c.lookupNamedGoImport(expression.Name, expression.Span); exists && imported.pack.packageInfo != nil {
			object = imported.pack.packageInfo.Scope().Lookup(imported.name)
		}
	case *ast.MemberExpr:
		if name, ok := expression.Object.(*ast.IdentifierExpr); ok && unshadowed(name.Name) {
			if pack := c.lookupGoPackage(name.Span.Path, name.Name); pack != nil && pack.packageInfo != nil {
				object = pack.packageInfo.Scope().Lookup(expression.Name)
			}
		}
	}
	if function, ok := object.(*gotypes.Func); ok && function.Exported() {
		if signature, ok := function.Type().(*gotypes.Signature); ok && signature.TypeParams() != nil {
			return explicit < signature.TypeParams().Len()
		}
	}
	return false
}
