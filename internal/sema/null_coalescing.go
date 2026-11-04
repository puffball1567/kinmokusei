package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkCoalesce(expr *ast.BinaryExpr, expected *Type) Type {
	expr.CoalesceType, expr.CoalesceLeftValue, expr.CoalesceHelper = nil, nil, ""
	expr.CoalesceNonNull = false
	left := c.singleValue(c.checkExpression(expr.Left), expr.Left.GetSpan())
	entry := c.snapshotNullableFlow()
	known := c.coalesceNilness(expr.Left)
	base := left
	if base.Kind == Nullable && base.Element != nil {
		base = *base.Element
	}
	if expected != nil && expected.Kind != Invalid {
		hint := *expected
		if hint.Kind == Nullable && hint.Element != nil {
			hint = *hint.Element
		}
		if left.Kind == Null || left.Kind == Nil || isNilable(hint) && c.isAssignable(hint, base) {
			base = hint
		}
	}
	var context *Type
	if base.Kind != Nil && base.Kind != Null && base.Kind != Invalid {
		hint := base
		if isNullableBaseType(base) {
			hint = Type{Kind: Nullable, Name: "nullable", Element: &base}
		}
		// Collection literals need their element/field context, while nullable
		// class/interface expressions need the wrapper to retain null on upcast.
		switch expr.Right.(type) {
		case *ast.ArrayLiteralExpr, *ast.ObjectLiteralExpr:
			hint = base
		}
		context = &hint
	}
	if known == 2 {
		c.suppressFlowEffects++
	}
	var right Type
	if context == nil {
		right = c.checkExpression(expr.Right)
	} else {
		right = c.checkExpressionExpectedSlot(&expr.Right, *context)
	}
	right = c.singleValue(right, expr.Right.GetSpan())
	expr.CoalesceNonNull = c.assignmentProvesNonNull(right, expr.Right)
	if known == 2 {
		c.suppressFlowEffects--
	}
	rightFlow := c.snapshotNullableFlow()
	switch known {
	case 1:
		c.restoreNullableFlow(rightFlow)
	case 2:
		c.restoreNullableFlow(entry)
	default:
		c.restoreNullableFlow(c.mergeNullableFlow(entry, entry, rightFlow))
	}
	if left.Kind == Invalid || right.Kind == Invalid {
		return Type{Kind: Invalid}
	}
	if right.Kind == Void || right.Kind == Result || right.Kind == Task {
		c.report(expr.Right.GetSpan(), "?? requires an ordinary value; Result and Task must be handled explicitly")
		return Type{Kind: Invalid}
	}
	if left.Kind != Null && left.Kind != Nil && !isNilable(baseOfNullable(left)) {
		c.report(expr.Left.GetSpan(), fmt.Sprintf("left operand of ?? must be nullable or nil-capable, got %s", left.String()))
		return Type{Kind: Invalid}
	}
	if base.Kind == Null || base.Kind == Nil {
		base = defaultLiteralType(baseOfNullable(right))
	}
	if base.Kind == Null || base.Kind == Nil {
		c.report(expr.Span, "cannot infer the result type of ?? from null or nil; add a concrete nullable type")
		return Type{Kind: Invalid}
	}
	if context == nil && !c.checkNumericMaterialization(expr.Right, base) {
		return Type{Kind: Invalid}
	}
	result := base
	if right.Kind == Nullable || right.Kind == Null || right.Kind == Nil {
		if !isNullableBaseType(base) {
			c.report(expr.Right.GetSpan(), fmt.Sprintf("cannot use %s as fallback for %s", right.String(), base.String()))
			return Type{Kind: Invalid}
		}
		result = Type{Kind: Nullable, Name: "nullable", Element: &base}
		if right.Kind == Nil {
			c.requireAssignable(base, right, expr.Right.GetSpan())
		} else {
			c.requireAssignable(result, right, expr.Right.GetSpan())
		}
	} else {
		c.requireAssignable(base, right, expr.Right.GetSpan())
	}
	ref := typeRefFromType(result, expr.Span)
	expr.CoalesceType = &ref
	value := ast.Expression(&ast.IdentifierExpr{Name: "__kinmokuseiCoalesceValue", Span: expr.Left.GetSpan()})
	if known != 1 {
		c.applyClassUpcast(&value, base, baseOfNullable(left))
	}
	expr.CoalesceLeftValue = value
	expr.CoalesceHelper = c.coalesceStaticHelper(expr, left, right, result)
	if expr.CoalesceHelper == "" && !coalesceTypeNameable(result) {
		c.report(expr.Span, "?? result contains a private Go type; add an explicit public interface or named type context")
		return Type{Kind: Invalid}
	}
	return result
}

func baseOfNullable(value Type) Type {
	if value.Kind == Nullable && value.Element != nil {
		return *value.Element
	}
	return value
}

// Only immutable, identity-matched initializers can decide whether a fallback
// runs. A narrowed class type alone is not proof: zero-value/Go ingress has
// separate checks. Bound traversal so a long chain cannot exhaust the stack.
func (c *Checker) coalesceNilness(expression ast.Expression) int {
	seen := map[source.Span]bool{}
	for steps := 0; steps < 512; steps++ {
		switch expr := expression.(type) {
		case *ast.LiteralExpr:
			if expr.Kind == ast.NullLiteral || expr.Kind == ast.NilLiteral {
				return 1
			}
			return 0
		case *ast.NewExpr, *ast.ArrowExpr, *ast.ArrayLiteralExpr:
			return 2
		case *ast.UnaryExpr:
			if expr.Operator == ast.NonNullOperator {
				return 2
			}
			if expr.Operator == "&" {
				if inner, ok := expr.Operand.(*ast.UnaryExpr); !ok || inner.Operator != "*" {
					return 2
				}
			}
			return 0
		case *ast.CallExpr:
			if expr.Builtin == ast.MakeSliceCall || expr.Builtin == ast.MakeMapCall || expr.Builtin == ast.MakeGoChannelCall || expr.Builtin == ast.MakeCall {
				return 2
			}
			return 0
		case *ast.IdentifierExpr:
			if seen[expr.ResolvedDeclaration] {
				return 0
			}
			seen[expr.ResolvedDeclaration] = true
			found := false
			for i := len(c.scopes) - 1; i >= 0; i-- {
				if symbol, ok := c.scopes[i][expr.Name]; ok && symbol.declarationSpan == expr.ResolvedDeclaration && symbol.constant && !symbol.flowEscaped && symbol.declaration != nil {
					expression = symbol.declaration.Value
					found = true
					break
				}
			}
			if !found {
				return 0
			}
		default:
			return 0
		}
	}
	return 0
}
