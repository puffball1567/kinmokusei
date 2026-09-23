package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkDecoratorValueBuiltin(expr *ast.CallExpr, unwrap bool) Type {
	c.usesDecoratorContext = true
	name := "decoratorValue"
	if unwrap {
		name = "decoratorValueAs"
	}
	if expr.Expanded {
		c.report(expr.Span, name+" does not accept spread arguments")
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("%s expects 1 argument, got %d", name, len(expr.Arguments)))
	}
	if unwrap {
		return c.checkDecoratorValueUnwrap(expr)
	}
	if len(expr.TypeArguments) > 1 {
		c.report(expr.Span, "decoratorValue accepts at most one explicit type argument")
	}
	value := Type{Kind: Invalid, Name: "<invalid>"}
	if len(expr.TypeArguments) == 0 && len(expr.Arguments) != 0 {
		value = defaultLiteralType(c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan()))
	}
	if len(expr.TypeArguments) == 1 {
		expected := c.resolveType(expr.TypeArguments[0])
		if len(expr.Arguments) != 0 {
			actual := c.checkExpressionExpectedSlot(&expr.Arguments[0], expected)
			c.requireAssignable(expected, actual, expr.Arguments[0].GetSpan())
		}
		value = expected
	}
	if !decoratorValuePayloadType(value) {
		if value.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("type %s cannot be stored in DecoratorValue", value.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	ref := typeRefFromType(value, expr.Span)
	expr.ResolvedTypeArguments = []ast.TypeRef{ref}
	expr.DecoratorValueIdentity = decoratorValueRuntimeIdentity(value)
	expr.Builtin = ast.DecoratorValueCall
	expr.Signature = &ast.CallableSignature{ParameterNames: []string{"value"}, ParameterTypes: []string{value.String()}, Result: ast.DecoratorValueTypeName}
	return decoratorValueType()
}

func (c *Checker) checkDecoratorValueUnwrap(expr *ast.CallExpr) Type {
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, "decoratorValueAs expects exactly one explicit type argument")
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	target := c.resolveType(expr.TypeArguments[0])
	if !decoratorValuePayloadType(target) {
		if target.Kind != Invalid {
			c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be extracted from DecoratorValue", target.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if len(expr.Arguments) != 0 {
		actual := c.checkExpressionExpectedSlot(&expr.Arguments[0], decoratorValueType())
		c.requireAssignable(decoratorValueType(), actual, expr.Arguments[0].GetSpan())
	}
	c.prepareGoTypeForEmission(&target, expr.Span)
	ref := typeRefFromType(target, expr.Span)
	expr.ResolvedTypeArguments = []ast.TypeRef{ref}
	expr.DecoratorValueIdentity = decoratorValueRuntimeIdentity(target)
	expr.Builtin = ast.DecoratorValueAsCall
	result := Type{Kind: Result, Name: "Result", Element: &target}
	expr.Signature = &ast.CallableSignature{ParameterNames: []string{"value"}, ParameterTypes: []string{ast.DecoratorValueTypeName}, Result: result.String()}
	return result
}

func decoratorValuePayloadType(value Type) bool {
	switch value.Kind {
	case Invalid, Void, Result, Task, MultiValue, GoPackage, GoTypeName, Nil, Null, UntypedInt:
		return false
	default:
		return true
	}
}

func decoratorValueRuntimeIdentity(value Type) string {
	switch value.Kind {
	case Class:
		return "type|" + value.String()
	case Interface:
		return "interface|" + value.String()
	case Struct:
		return "struct|" + value.String()
	default:
		return value.String()
	}
}
