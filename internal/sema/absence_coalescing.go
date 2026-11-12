package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// References use the ordinary nil fallback. A value payload cannot represent
// absence as null, so a direct map lookup/receive instead tests the success flag.
// Check the producer once, preserving its operand effects and source contracts.
func (c *Checker) checkCoalesceOperand(expr *ast.BinaryExpr, expected *Type) Type {
	var value Type
	switch left := expr.Left.(type) {
	case *ast.OptionalChainExpr:
		value = c.checkOptionalChain(left, expected)
		if left.Checked && value.Kind == MultiValue && len(value.Results) == 2 {
			expr.CoalesceChecked = true
			return value.Results[0]
		}
		return c.singleValue(value, left.Span)
	case *ast.IndexExpr:
		value = c.checkIndexAccess(left, false, true)
		if !left.MapIndex {
			return c.singleValue(value, left.Span)
		}
	case *ast.UnaryExpr:
		if left.Operator != "<-" {
			return c.singleValue(c.checkExpression(left), left.Span)
		}
		pair := c.checkChannelReceive(left, true)
		if pair.Kind != MultiValue || len(pair.Results) != 2 {
			return Type{Kind: Invalid}
		}
		value = pair.Results[0]
	default:
		return c.singleValue(c.checkExpression(expr.Left), expr.Left.GetSpan())
	}
	if value.Kind == Invalid {
		return value
	}
	if value.Kind == Nullable || isNilable(value) {
		return c.absenceReadResult(value, "lookup/receive value", "ok", expr.Left.GetSpan())
	}
	if !c.coalesceValuePayload(value, map[gotypes.Type]bool{}) {
		c.report(expr.Left.GetSpan(), "?? requires a uniform nil or value payload contract; use [value, ok] and check ok for an open or mixed reference/value constraint")
		return Type{Kind: Invalid}
	}
	expr.CoalesceChecked = true
	return value
}

func (c *Checker) coalesceValuePayload(value Type, active map[gotypes.Type]bool) bool {
	parameter, generic := value.GoType.(*gotypes.TypeParam)
	if value.Kind != TypeParameter || !generic {
		return value.Kind != Nullable && !isNilable(value)
	}
	if active[parameter] {
		return false
	}
	active[parameter] = true
	defer delete(active, parameter)
	terms := c.collectionTerms(parameter)
	if len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		if !c.coalesceValuePayload(term, active) {
			return false
		}
	}
	return true
}
