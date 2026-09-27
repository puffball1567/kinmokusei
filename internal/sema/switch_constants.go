package sema

import (
	"go/constant"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Go defaults an untyped switch subject before checking its cases. Case types
// cannot retrospectively choose a width or floating-point kind for the tag.
func (c *Checker) checkSwitchTag(expression ast.Expression, actual Type) Type {
	target := defaultLiteralType(actual)
	if info, known := c.checkedNumericConstant(expression, actual); known {
		if gt, ok := goTypeOf(target); ok {
			if err := c.checkNumericConstantAssignment(info, gt); err != nil {
				c.report(expression.GetSpan(), err.Error())
				return Type{Kind: Invalid}
			}
		}
	} else if !c.checkNumericMaterialization(expression, target) {
		return Type{Kind: Invalid}
	}
	return target
}

// Identical values can have distinct dynamic types in an interface switch.
// Keep Go type identities, not display names (aliases and private types matter).
type switchConstantCases map[any][]gotypes.Type

func (c *Checker) checkSwitchCaseConstant(expression ast.Expression, actual, target Type, seen switchConstantCases) {
	if actual.Kind == Invalid || target.Kind == Invalid || !c.isAssignable(target, actual) {
		return
	}
	var key any
	var display string
	var identity gotypes.Type
	if actual.Kind == Nil || actual.Kind == Null {
		// Retain the language's existing duplicate nil/null case diagnostic.
		key, display = "nil", actual.String()
	} else {
		info, known := c.checkedNumericConstant(expression, actual)
		if !known {
			return // Immutable runtime bindings are not constant case labels.
		}
		if basic, untyped := info.Type.(*gotypes.Basic); untyped && basic.Info()&gotypes.IsUntyped != 0 {
			gt, ok := goTypeOf(target)
			if !ok {
				return
			}
			if target.Kind == TypeParameter {
				// Implicit assignment remains constant with the parameter's
				// identity; unlike explicit T(value), it is not a conversion call.
				info.Type = gt
			} else if underlyingGoInterface(gt) != nil {
				// Go's case checker keeps the exact value here, assigning only
				// its default dynamic type. Do not introduce extra rounding.
				gt = gotypes.Default(info.Type)
				if err := c.checkNumericConstantAssignment(info, gt); err != nil {
					c.report(expression.GetSpan(), err.Error())
					return
				}
				info.Type = gt
			} else {
				converted, err := c.convertNumericConstant(info, gt)
				if err != nil || converted.Value == nil {
					return
				}
				info = converted
			}
		}
		var comparable bool
		key, comparable = switchConstantKey(info.Value)
		if !comparable {
			return
		}
		display = info.Value.ExactString()
		identity = info.Type
	}
	for _, previous := range seen[key] {
		if previous == nil && identity == nil || previous != nil && identity != nil && gotypes.Identical(previous, identity) {
			c.report(expression.GetSpan(), "duplicate value switch case "+display)
			return
		}
	}
	seen[key] = append(seen[key], identity)
}

// Match Go's finite duplicate-case restriction rather than imposing stronger
// mathematical equality: booleans, complex values, and exact fractions that
// cannot be represented by float64 are not included in its duplicate table.
func switchConstantKey(value constant.Value) (any, bool) {
	switch value.Kind() {
	case constant.Int:
		if integer, exact := constant.Int64Val(value); exact {
			return integer, true
		}
		integer, exact := constant.Uint64Val(value)
		return integer, exact
	case constant.Float:
		number, exact := constant.Float64Val(value)
		return number, exact
	case constant.String:
		return constant.StringVal(value), true
	default:
		return nil, false
	}
}
