package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func genericArgumentParameter(callable Type, call *ast.CallExpr, index int) Type {
	parameter := index
	if callable.Variadic && parameter >= len(callable.Parameters)-1 {
		parameter = len(callable.Parameters) - 1
	}
	if parameter < 0 || parameter >= len(callable.Parameters) {
		return Type{}
	}
	expected := callable.Parameters[parameter]
	if call.Expanded && callable.Variadic && index == len(call.Arguments)-1 {
		element := expected
		expected = Type{Kind: Array, Name: "array", Element: &element}
	}
	return expected
}

// Direct arrow construction and generic declaration references do not execute
// user code. Delay their checking until other arguments provide their contexts;
// never reorder emitted arguments
// or evaluate a source expression twice. The ordinary call checker remains the
// authority for argument compatibility, arity, constants, and constraints.
func (c *Checker) checkGenericCallbackArguments(call *ast.CallExpr, callable Type, bindings nativeTypeBindings) []Type {
	actuals := make([]Type, len(call.Arguments))
	pending := make(map[int]*ast.ArrowExpr)
	// Keep contextual construction separate from authoritative inference below.
	// Explicit arguments and preceding typed values can fix a fresh literal's
	// storage, but must not default an earlier constant or recheck an expression.
	contexts := make(nativeTypeBindings, len(bindings))
	for parameter, value := range bindings {
		contexts[parameter] = value
	}
	for i, argument := range call.Arguments {
		if arrow, ok := argument.(*ast.ArrowExpr); ok {
			pending[i] = arrow
		} else if c.deferredGenericFunctionValue(argument) {
			pending[i] = nil
		} else {
			formal := genericArgumentParameter(callable, call, i)
			var actual Type
			switch argument.(type) {
			case *ast.ArrayLiteralExpr, *ast.ObjectLiteralExpr:
				c.inferNativeConstraintArguments(callable.TypeParameters, contexts)
				context := substituteNativeTypeParameters(formal, contexts)
				if !containsUnboundCallbackType(context, contexts) {
					actual = c.checkExpressionExpectedSlot(&call.Arguments[i], context)
				} else {
					actual = c.checkExpression(argument)
				}
			default:
				actual = c.checkExpression(argument)
			}
			_, isCall := argument.(*ast.CallExpr)
			if actual.Kind == MultiValue && isCall && len(call.Arguments) == 1 && !call.Expanded {
				actuals[i] = actual
			} else {
				actuals[i] = c.singleValue(actual, argument.GetSpan())
				// Untyped constants must still wait for typed arguments/callbacks;
				// choosing a contextual type must not change numeric precedence.
				values := []Type{actuals[i]}
				constant := c.genericNumericArguments([]ast.Expression{argument}, values)[0]
				if formal.Kind == TypeParameter && untypedNumericRank(constant) != 0 {
					if _, own := contexts[formal.GoType]; own {
						continue
					}
				}
				c.inferGenericArgumentContext(formal, actuals[i], contexts)
			}
		}
	}
	if len(pending) == 0 {
		return actuals
	}
	numeric := c.genericNumericArguments(call.Arguments, actuals)
	defaults := map[gotypes.Type]int{}
	for i, actual := range actuals {
		expected := genericArgumentParameter(callable, call, i)
		if arrow, deferred := pending[i]; deferred {
			if arrow == nil {
				continue
			}
			// Explicit parts of callback signatures also outrank numeric defaults.
			context := c.genericCallbackContext(expected, nil)
			if context.Kind == Function && len(context.Parameters) == len(arrow.Parameters) && context.Variadic == hasVariadicParameter(arrow.Parameters) {
				for j, parameter := range arrow.Parameters {
					if parameter.Type.IsSpecified() {
						value := c.callableParameterType(parameter, c.resolveType(parameter.Type))
						_ = c.inferNativeTypeArguments(context.Parameters[j], value, bindings)
					}
				}
				if arrow.ReturnType != nil && context.Result != nil {
					_ = c.inferNativeTypeArguments(*context.Result, c.resolveType(*arrow.ReturnType), bindings)
				}
			}
			continue
		}
		if rank := untypedNumericRank(numeric[i]); rank != 0 && expected.Kind == TypeParameter {
			if _, inferable := bindings[expected.GoType]; inferable {
				previous, exists := defaults[expected.GoType]
				if !exists || rank > untypedNumericRank(numeric[previous]) {
					defaults[expected.GoType] = i
				}
				continue
			}
		}
		c.inferGenericArgumentContext(expected, actual, bindings)
	}
	c.inferNativeConstraintArguments(callable.TypeParameters, bindings)
	defaultsApplied := false
	for len(pending) != 0 {
		c.inferNativeConstraintArguments(callable.TypeParameters, bindings)
		progress := false
		// Source order makes independent callbacks and diagnostics deterministic.
		for i := range call.Arguments {
			arrow, ok := pending[i]
			if !ok {
				continue
			}
			formal := genericArgumentParameter(callable, call, i)
			context := c.genericCallbackContext(formal, bindings)
			if arrow == nil {
				if containsUnboundCallbackType(context, bindings) {
					continue
				}
				actuals[i] = c.checkExpressionExpectedSlot(&call.Arguments[i], context)
				_ = c.inferNativeTypeArguments(c.arrowContext(formal), actuals[i], bindings)
				delete(pending, i)
				progress = true
				continue
			}
			if !genericArrowContextReady(arrow, context, bindings) {
				continue
			}
			if context.Result != nil && containsUnboundCallbackType(*context.Result, bindings) {
				// The body can infer a result parameter not fixed by other inputs.
				context.Result = nil
			}
			actuals[i] = c.checkArrowExpected(arrow, context)
			_ = c.inferNativeTypeArguments(c.arrowContext(formal), actuals[i], bindings)
			delete(pending, i)
			progress = true
		}
		if !progress {
			if !defaultsApplied {
				// A callback with known inputs can supply a typed result before
				// constants default. Otherwise defaults may unlock its inputs.
				for parameter, i := range defaults {
					if bindings[parameter].Kind == Invalid {
						bindings[parameter] = defaultLiteralType(actuals[i])
					}
				}
				defaultsApplied = true
				continue
			}
			// Report missing annotations rather than inventing a type for a cycle.
			for i := range call.Arguments {
				if arrow, ok := pending[i]; ok {
					if arrow == nil {
						c.report(call.Arguments[i].GetSpan(), fmt.Sprintf("cannot infer generic function callback %d; provide explicit type arguments or a function type annotation", i+1))
					} else {
						actuals[i] = c.checkArrow(arrow)
					}
				}
			}
			break
		}
	}
	return actuals
}

func (c *Checker) inferGenericArgumentContext(formal, actual Type, bindings nativeTypeBindings) {
	if formal.Kind == GoNamed || formal.Kind == GoInterface && !isSourceAnonymousInterface(formal) {
		if contract := underlyingGoInterface(formal.GoType); contract != nil && contract.IsMethodSet() {
			// Method signatures can determine the input types of a later arrow.
			// The ordinary call probe still verifies the instantiated signature.
			c.inferNativeConstraintMethods(contract, actual, bindings)
		}
	}
	_ = c.inferNativeTypeArguments(formal, actual, bindings)
}

// A common signature provides context for constructing the argument. Keep the
// formal parameter itself for inference and the final type-set membership check.
func (c *Checker) genericCallbackContext(formal Type, bindings nativeTypeBindings) Type {
	context := c.arrowContext(substituteNativeTypeParameters(formal, bindings))
	if context.Kind == TypeParameter {
		if signature, ok := c.constrainedCallableType(context); ok {
			return substituteNativeTypeParameters(signature, bindings)
		}
	}
	return context
}

func genericArrowContextReady(arrow *ast.ArrowExpr, context Type, bindings nativeTypeBindings) bool {
	if context.Kind != Function || len(context.Parameters) != len(arrow.Parameters) || context.Variadic != hasVariadicParameter(arrow.Parameters) {
		return true // Let ordinary arrow checking report an incompatible context.
	}
	for i, parameter := range arrow.Parameters {
		if !parameter.Type.IsSpecified() && containsUnboundCallbackType(context.Parameters[i], bindings) {
			return false
		}
	}
	return true
}

func containsUnboundCallbackType(value Type, bindings nativeTypeBindings) bool {
	if value.Kind == TypeParameter {
		binding, own := bindings[value.GoType]
		return own && binding.Kind == Invalid
	}
	for _, nested := range []*Type{value.Element, value.Key, value.Result} {
		if nested != nil && containsUnboundCallbackType(*nested, bindings) {
			return true
		}
	}
	for _, group := range [][]Type{value.Parameters, value.TypeArguments, value.Results} {
		for _, nested := range group {
			if containsUnboundCallbackType(nested, bindings) {
				return true
			}
		}
	}
	for _, method := range value.GoMethods {
		if containsUnboundCallbackType(method.Type, bindings) {
			return true
		}
	}
	// Named recursive records carry their parameters in TypeArguments.
	if value.Kind == Object {
		for _, field := range value.Fields {
			if containsUnboundCallbackType(field, bindings) {
				return true
			}
		}
	}
	return false
}

func (c *Checker) checkGoGenericCallbackArguments(call *ast.CallExpr, signature *gotypes.Signature, explicit []gotypes.Type) []Type {
	// Strip only the signature's parameter declarations to expose its generic
	// input shapes. Keep the original TypeParam identities for substitutions.
	shape := gotypes.NewSignatureType(nil, nil, nil, signature.Params(), signature.Results(), signature.Variadic())
	callable, err := kinmokuseiFunctionFromGo(shape)
	if err != nil {
		callable = Type{}
	}
	bindings := nativeTypeBindings{}
	for i := 0; i < signature.TypeParams().Len(); i++ {
		parameter := signature.TypeParams().At(i)
		callable.TypeParameters = append(callable.TypeParameters, Type{Kind: TypeParameter, Name: parameter.Obj().Name(), GoType: parameter})
		bindings[parameter] = Type{}
		if i < len(explicit) {
			bindings[parameter], _ = kinmokuseiTypeFromGo(explicit[i])
		}
	}
	return c.checkGenericCallbackArguments(call, callable, bindings)
}
