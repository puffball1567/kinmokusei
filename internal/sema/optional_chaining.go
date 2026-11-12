package sema

import (
	"fmt"
	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkOptionalChain(expr *ast.OptionalChainExpr, expected *Type) Type {
	expr.ResultType, expr.Checked, expr.ForwardChecked = ast.TypeRef{}, false, false
	object := c.singleValue(c.checkExpression(expr.Object), expr.Object.GetSpan())
	base := baseOfNullable(object)
	if base.Kind == Invalid {
		return base
	}
	if !isNilable(base) {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("optional chaining requires a nullable or nil-capable receiver, got %s", object.String()))
		return Type{Kind: Invalid}
	}
	entry := c.snapshotNullableFlow()
	known := c.coalesceNilness(expr.Object)
	if known == 1 {
		c.suppressFlowEffects++
	}
	previous := c.optionalReceiver
	c.optionalReceiver = base
	expr.Receiver.Type = typeRefFromType(base, expr.Object.GetSpan())
	var value Type
	if expected != nil && expected.Kind != Invalid && expected.Kind != MultiValue {
		hint := baseOfNullable(*expected)
		value = c.checkExpressionExpectedSlot(&expr.Value, hint)
		if c.isAssignable(hint, value) {
			value = hint
		}
	} else {
		value = c.checkExpression(expr.Value)
	}
	c.optionalReceiver = previous
	if known == 1 {
		c.suppressFlowEffects--
	}
	bodyFlow := c.snapshotNullableFlow()
	switch known {
	case 1:
		c.restoreNullableFlow(entry)
	case 2:
		c.restoreNullableFlow(bodyFlow)
	default:
		c.restoreNullableFlow(c.mergeNullableFlow(entry, entry, bodyFlow))
	}
	if nested, ok := expr.Value.(*ast.OptionalChainExpr); ok && nested.Checked && value.Kind == MultiValue {
		expr.ForwardChecked = true
		value = value.Results[0]
	}
	if value.Kind == Invalid {
		return value
	}
	if value.Kind == Result || value.Kind == Task || value.Kind == MultiValue || value.Kind == Nil || value.Kind == Null {
		c.report(expr.Value.GetSpan(), "optional chaining requires an ordinary value or void call; handle Result, Task and multiple results explicitly")
		return Type{Kind: Invalid}
	}
	if !coalesceTypeNameable(value) {
		c.report(expr.Span, "optional chaining result contains a private Go type; add an explicit public interface or named type context")
		return Type{Kind: Invalid}
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	expr.ResultType = typeRefFromType(value, expr.Span)
	if value.Kind == Void {
		return value
	}
	if value.Kind == Nullable {
		return value
	}
	if isNullableBaseType(value) {
		return Type{Kind: Nullable, Name: "nullable", Element: &value}
	}
	expr.Checked = true
	return Type{Kind: MultiValue, Results: []Type{value, builtins["boolean"]}}
}
