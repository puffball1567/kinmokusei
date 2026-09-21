package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) genericCallInputs(call *ast.CallExpr, actuals []Type) ([]Type, []gotypes.TypeAndValue, bool) {
	if len(actuals) == 1 && actuals[0].Kind == MultiValue {
		call.MultipleArgumentGeneric = true
		values := actuals[0].Results
		// Call results have runtime types, never constant argument values.
		return values, make([]gotypes.TypeAndValue, len(values)), true
	}
	return actuals, c.genericNumericArguments(call.Arguments, actuals), false
}

func (c *Checker) checkNativeGenericMultipleCall(call *ast.CallExpr, name string, callable Type, bindings nativeTypeBindings, values Type) Type {
	call.MultipleArgumentGeneric = true
	minimum := len(callable.Parameters)
	if callable.Variadic {
		minimum--
	}
	if len(values.Results) < minimum || (!callable.Variadic && len(values.Results) != minimum) {
		c.report(call.Span, fmt.Sprintf("multiple-result argument count mismatch for %s: got %d values for %d parameters", name, len(values.Results), len(callable.Parameters)))
		return Type{Kind: Invalid}
	}
	for i, value := range values.Results {
		index := i
		if callable.Variadic && index >= minimum {
			index = minimum
		}
		if index < 0 || index >= len(callable.Parameters) {
			continue
		}
		if err := c.inferNativeTypeArguments(callable.Parameters[index], value, bindings); err != nil {
			c.report(call.Arguments[0].GetSpan(), fmt.Sprintf("cannot infer type arguments for %s from result %d: %v", name, i+1, err))
		}
	}
	c.inferNativeConstraintArguments(callable.TypeParameters, bindings)
	arguments := make([]Type, len(callable.TypeParameters))
	for i, parameter := range callable.TypeParameters {
		arguments[i] = bindings[parameter.GoType]
		if arguments[i].Kind == Invalid {
			c.report(call.Span, fmt.Sprintf("cannot infer type argument %s for %s; provide explicit type arguments", parameter.Name, name))
			return Type{Kind: Invalid}
		}
	}
	if !c.validateNativeTypeArguments(callable.TypeParameters, arguments, call.TypeArguments, call.Span, name) {
		return Type{Kind: Invalid}
	}
	parameters := make([]Type, len(callable.Parameters))
	for i, parameter := range callable.Parameters {
		parameters[i] = substituteNativeTypeParameters(parameter, bindings)
	}
	result := substituteNativeTypeParameters(*callable.Result, bindings)
	instantiated := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: callable.Variadic, Result: &result}
	c.recordCallSignature(call, instantiated)
	c.checkMultipleCallArguments(call, name, instantiated, values)
	if member, ok := call.Callee.(*ast.MemberExpr); ok && member.GenericMethod {
		call.MultipleArgumentCount = len(values.Results)
		ref := typeRefFromType(result, call.Span)
		call.MultipleArgumentResult = &ref
	}
	call.ResolvedTypeArguments = make([]ast.TypeRef, len(arguments))
	for i, argument := range arguments {
		call.ResolvedTypeArguments[i] = typeRefFromType(argument, call.Span)
	}
	return result
}
