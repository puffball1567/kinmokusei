package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) singleValue(value Type, span source.Span) Type {
	if value.Kind == Result {
		c.report(span, "Result values must be consumed with ?, explicitly split, or returned")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if value.Kind != MultiValue {
		return value
	}
	c.report(span, fmt.Sprintf("multiple values %s require destructuring", value.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkResultConstructor(expr *ast.CallExpr, success bool) Type {
	if c.result.Kind != Result || c.result.Element == nil {
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		c.report(expr.Span, "ok and fail may only be used inside a Result-returning function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, "Result constructors do not accept type arguments")
	}
	if expr.Expanded {
		c.report(expr.Span, "Result constructors do not accept spread arguments")
	}
	if success {
		expr.Builtin = ast.ResultOKCall
		expected := 1
		if c.result.Element.Kind == Void {
			expected = 0
		}
		expr.Signature = &ast.CallableSignature{Result: c.result.String()}
		if expected == 1 {
			expr.Signature.ParameterNames = []string{"value"}
			expr.Signature.ParameterTypes = []string{c.result.Element.String()}
		}
		if len(expr.Arguments) != expected {
			c.report(expr.Span, fmt.Sprintf("ok for %s expects %d arguments, got %d", c.result.String(), expected, len(expr.Arguments)))
		}
		for i, argument := range expr.Arguments {
			actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], *c.result.Element)
			if i == 0 && expected == 1 {
				c.requireAssignable(*c.result.Element, actual, argument.GetSpan())
			}
		}
		return c.result
	}
	expr.Builtin = ast.ResultFailCall
	expr.Signature = &ast.CallableSignature{
		ParameterNames: []string{"error"}, ParameterTypes: []string{builtins["error"].String()}, Result: c.result.String(),
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("fail expects 1 error argument, got %d", len(expr.Arguments)))
	}
	for i, argument := range expr.Arguments {
		actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], builtins["error"])
		if i == 0 {
			c.requireAssignable(builtins["error"], actual, argument.GetSpan())
		}
	}
	return c.result
}

func (c *Checker) checkResultReturn(stmt *ast.ReturnStmt) {
	if c.result.Element == nil {
		return
	}
	stmt.ResultType = typeRefFromType(*c.result.Element, stmt.Span)
	if stmt.Value == nil {
		c.report(stmt.Span, fmt.Sprintf("Result function must return ok(...) or fail(...), expected %s", c.result.String()))
		return
	}
	value := c.checkExpressionExpectedSlot(&stmt.Value, c.result)
	if call, ok := stmt.Value.(*ast.CallExpr); ok {
		switch call.Builtin {
		case ast.ResultOKCall:
			stmt.ResultKind = ast.ResultSuccessReturn
			return
		case ast.ResultFailCall:
			stmt.ResultKind = ast.ResultFailureReturn
			return
		}
	}
	if value.Kind == Result && exactType(c.result, value) {
		stmt.ResultKind = ast.ResultForwardReturn
		return
	}
	if value.Kind == MultiValue {
		c.report(stmt.Value.GetSpan(), "Go multiple results are not implicitly converted to Result; propagate them with ? and return ok(value)")
		return
	}
	if value.Kind != Invalid {
		c.report(stmt.Value.GetSpan(), fmt.Sprintf("Result function must return ok(...) or fail(...), got %s", value.String()))
	}
}

func (c *Checker) checkPropagateExpression(expr *ast.PropagateExpr) Type {
	operand := c.checkExpression(expr.Value)
	if c.result.Kind != Result || c.result.Element == nil {
		c.report(expr.Span, "operator ? may only be used inside a Result-returning function")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	var value Type
	switch {
	case operand.Kind == Result && operand.Element != nil:
		value = *operand.Element
	case operand.Kind == MultiValue && len(operand.Results) == 2 && c.isAssignable(builtins["error"], operand.Results[1]):
		value = operand.Results[0]
	case c.isAssignable(builtins["error"], operand):
		value = builtins["void"]
	default:
		if operand.Kind != Invalid {
			c.report(expr.Value.GetSpan(), fmt.Sprintf("operator ? requires Result<T>, (T, error), or error; got %s", operand.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	resultElement := *c.result.Element
	c.prepareGoTypeForEmission(&resultElement, expr.Span)
	expr.ValueType = typeRefFromType(value, expr.Span)
	expr.ResultType = typeRefFromType(resultElement, expr.Span)
	expr.ErrorName = fmt.Sprintf("__kinmokusei_result_error_%d", expr.Span.Start.Offset)
	return value
}

func (c *Checker) rejectResultValueType(value Type, span source.Span, context string) {
	if containsResultType(value) {
		c.report(span, fmt.Sprintf("Result may only be used as a function or method return type, not for %s", context))
	}
}

func containsResultType(value Type) bool {
	return containsResultTypeSeen(value, map[string]bool{})
}

func containsResultTypeSeen(value Type, visiting map[string]bool) bool {
	if value.Kind == Task {
		return false
	}
	if value.Kind == Result {
		return true
	}
	if value.Kind == Struct {
		if visiting[value.Name] {
			return false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
	}
	if value.Element != nil && containsResultTypeSeen(*value.Element, visiting) {
		return true
	}
	if value.Key != nil && containsResultTypeSeen(*value.Key, visiting) {
		return true
	}
	if value.Result != nil && containsResultTypeSeen(*value.Result, visiting) {
		return true
	}
	for _, parameter := range value.Parameters {
		if containsResultTypeSeen(parameter, visiting) {
			return true
		}
	}
	if value.Kind == Object || value.Kind == Struct {
		for _, field := range value.Fields {
			if containsResultTypeSeen(field, visiting) {
				return true
			}
		}
	}
	return false
}
