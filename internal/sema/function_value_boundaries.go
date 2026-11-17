package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) recordFunctionValueBoundary(expression *ast.IndexExpr, callable Type) {
	boundary := &ast.FunctionValueBoundary{}
	needed := false
	for i, value := range callable.Parameters {
		variadic := callable.Variadic && i == len(callable.Parameters)-1
		if variadic {
			element := value
			value = Type{Kind: Array, Name: "array", Element: &element}
		}
		parameter := ast.Parameter{Name: fmt.Sprintf("__kinmokuseiValueArgument%d", i), Type: typeRefFromType(value, expression.Span), Variadic: variadic, Span: expression.Span}
		c.checkParameterBoundary(&parameter, value)
		needed = needed || parameter.NonNull || parameter.NonNullKinds != 0
		boundary.Parameters = append(boundary.Parameters, parameter)
	}
	if callable.Result != nil {
		result := *callable.Result
		var values []Type
		switch result.Kind {
		case Void:
		case MultiValue:
			values = result.Results
		case Result:
			boundary.Condition = ast.NonNullIfNoError
			if result.Element != nil && result.Element.Kind != Void {
				values = []Type{*result.Element, builtins["error"]}
			} else {
				values = []Type{builtins["error"]}
			}
		default:
			values = []Type{result}
		}
		boundary.Results = make([]*ast.TypeRef, len(values))
		for i, value := range values {
			if c.needsStorageBoundary(value, value, nil) {
				ref := c.boundaryTypeRef(value, expression.Span)
				boundary.Results[i] = &ref
				needed = true
			}
		}
	}
	if needed {
		expression.FunctionBoundary = boundary
	}
}
