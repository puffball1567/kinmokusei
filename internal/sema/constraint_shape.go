package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Go storage alone cannot distinguish Leaf from Leaf | null. Keep the checked
// source collection shape beside each bound, including after owner substitution.
func (c *Checker) setParameterRangeShape(parameter *gotypes.TypeParam, shape Type) {
	if c.parameterRangeShapes == nil {
		c.parameterRangeShapes = map[*gotypes.TypeParam]Type{}
	}
	c.parameterRangeShapes[parameter] = shape
}

func (c *Checker) parameterRangeShape(parameter *gotypes.TypeParam) (Type, bool) {
	if shape, ok := c.parameterRangeShapes[parameter]; ok {
		return shape, shape.Kind != Invalid
	}
	core := goRangeCoreType(parameter)
	if core == nil {
		c.setParameterRangeShape(parameter, Type{Kind: Invalid})
		return Type{}, false
	}
	shape, err := kinmokuseiTypeFromGo(core)
	if err != nil {
		c.setParameterRangeShape(parameter, Type{Kind: Invalid})
		return Type{}, false
	}
	shape = c.restoreNativeRangeType(shape)
	c.setParameterRangeShape(parameter, shape)
	return shape, true
}

func (c *Checker) recordParameterRangeShape(parameter *gotypes.TypeParam, ref ast.TypeRef) {
	core := goRangeCoreType(parameter)
	if core == nil {
		c.setParameterRangeShape(parameter, Type{Kind: Invalid})
		return
	}
	shape, err := kinmokuseiTypeFromGo(core)
	if err != nil {
		c.setParameterRangeShape(parameter, Type{Kind: Invalid})
		return
	}
	shape = c.restoreNativeRangeType(shape)
	if symbol := c.interfaces[ref.Name]; ref.Qualifier == "" && symbol != nil && symbol.constraint && len(ref.GenericArguments) == len(symbol.typeParameters) {
		bindings := make(nativeTypeBindings, len(symbol.typeParameters))
		for i, argument := range ref.GenericArguments {
			bindings[symbol.typeParameters[i].GoType] = c.resolveType(argument)
		}
		for _, term := range symbol.constraintTermTypes {
			candidate := c.constraintArgumentShape(substituteNativeTypeParameters(term, bindings))
			storage, valid := c.goTypeForNativeStorage(candidate)
			if !valid {
				continue
			}
			if gotypes.Identical(storage.Underlying(), core) {
				shape = candidate
				break
			}
			if channel, ok := storage.Underlying().(*gotypes.Chan); ok {
				if target, ok := core.(*gotypes.Chan); ok && gotypes.Identical(channel.Elem(), target.Elem()) {
					shape.Element = candidate.Element
					break
				}
			}
		}
	}
	c.setParameterRangeShape(parameter, shape)
}

func (c *Checker) constraintArgumentShape(value Type) Type {
	if value.Kind == TypeParameter {
		if parameter, ok := value.GoType.(*gotypes.TypeParam); ok {
			if shape, ok := c.parameterRangeShape(parameter); ok {
				return shape
			}
		}
	}
	if value.Kind != GoNamed || value.GoType == nil {
		return value
	}
	if named, ok := gotypes.Unalias(value.GoType).(*gotypes.Named); ok {
		if symbol := c.nativeTypes[named.Obj().Name()]; symbol != nil && named.Origin() == symbol.goNamed {
			return c.nativeDefinedUnderlying(symbol, value)
		}
	}
	if shape, err := kinmokuseiTypeFromGo(value.GoType.Underlying()); err == nil {
		return c.restoreNativeRangeType(shape)
	}
	return value
}

// Go satisfaction checks the storage type; this additional check protects the
// source qualifiers it erases. Other shape/type mismatches remain Go's checks.
func sameConstraintNullability(expected, actual Type) bool {
	if (expected.Kind == Nullable) != (actual.Kind == Nullable) {
		return false
	}
	if expected.Kind != actual.Kind {
		return true
	}
	for _, pair := range [][2]*Type{{expected.Element, actual.Element}, {expected.Key, actual.Key}, {expected.Result, actual.Result}} {
		if pair[0] != nil && pair[1] != nil && !sameConstraintNullability(*pair[0], *pair[1]) {
			return false
		}
	}
	for _, pair := range [][2][]Type{{expected.Parameters, actual.Parameters}, {expected.TypeArguments, actual.TypeArguments}} {
		if len(pair[0]) == len(pair[1]) {
			for i := range pair[0] {
				if !sameConstraintNullability(pair[0][i], pair[1][i]) {
					return false
				}
			}
		}
	}
	return true
}
