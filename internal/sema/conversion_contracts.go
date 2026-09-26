package sema

import gotypes "go/types"

// Explicit conversions change nominal identity, not the checked source contract.
// Go's convertibility alone cannot see nullable qualifiers or Result payloads.
func (c *Checker) conversionPreservesSourceContract(target, value Type) bool {
	if value.Kind == Nullable {
		return false
	}
	return c.conversionContractsMatch(target, value, map[[2]gotypes.Type]bool{})
}

func (c *Checker) conversionContractsMatch(target, value Type, visiting map[[2]gotypes.Type]bool) bool {
	if (target.Kind == Nullable) != (value.Kind == Nullable) {
		return false
	}
	// Identity conversions of a type parameter preserve the actual argument,
	// rather than converting between every pair of alternatives in its bound.
	if target.Kind == TypeParameter && value.Kind == TypeParameter && target.GoType == value.GoType {
		return true
	}
	if parameter, ok := target.GoType.(*gotypes.TypeParam); target.Kind == TypeParameter && ok {
		for _, term := range c.collectionTerms(parameter) {
			if !c.conversionContractsMatch(term, value, visiting) {
				return false
			}
		}
		return true
	}
	if parameter, ok := value.GoType.(*gotypes.TypeParam); value.Kind == TypeParameter && ok {
		for _, term := range c.collectionTerms(parameter) {
			if !c.conversionContractsMatch(target, term, visiting) {
				return false
			}
		}
		return true
	}
	if (target.Kind == GoNamed || target.Kind == Struct) && (value.Kind == GoNamed || value.Kind == Struct) {
		if !sameConstraintNullability(target, value) {
			return false
		}
		// Recursive named containers must terminate without skipping a later,
		// unrelated comparison of the same Go storage and different qualifiers.
		targetStorage, _ := c.goTypeForNativeStorage(target)
		valueStorage, _ := c.goTypeForNativeStorage(value)
		pair := [2]gotypes.Type{targetStorage, valueStorage}
		if visiting[pair] {
			return true
		}
		visiting[pair] = true
		defer delete(visiting, pair)
	}
	target, value = c.constraintArgumentShape(target), c.constraintArgumentShape(value)
	records := (target.Kind == Object || target.Kind == Struct) && (value.Kind == Object || value.Kind == Struct)
	if target.Kind != value.Kind && !records {
		return true // Go convertibility checks non-qualifier shape differences.
	}
	if target.Kind == Function && target.Result != nil && value.Result != nil {
		if !compatibleResultFunctionTypes(target, value) {
			return false
		}
		// A Result may bridge to the corresponding raw Go result list, but
		// its success slots must keep their source nullable contracts.
		targetResult, valueResult := *target.Result, *value.Result
		if targetResult.Kind == Result {
			targetResult = methodResultStorageShape(targetResult)
		}
		if valueResult.Kind == Result {
			valueResult = methodResultStorageShape(valueResult)
		}
		target.Result, value.Result = &targetResult, &valueResult
	}
	for _, pair := range [][2]*Type{{target.Element, value.Element}, {target.Key, value.Key}, {target.Result, value.Result}} {
		if pair[0] != nil && pair[1] != nil && !c.conversionContractsMatch(*pair[0], *pair[1], visiting) {
			return false
		}
	}
	for _, pair := range [][2][]Type{{target.Parameters, value.Parameters}, {target.TypeArguments, value.TypeArguments}, {target.Results, value.Results}} {
		if len(pair[0]) == len(pair[1]) {
			for i := range pair[0] {
				if !c.conversionContractsMatch(pair[0][i], pair[1][i], visiting) {
					return false
				}
			}
		}
	}
	if records {
		// Struct visibility and structural-object names can give the same Go
		// field different source spellings. Compare the actual storage fields.
		fields := make(map[string]Type, len(value.Fields))
		for name, field := range value.Fields {
			if emitted := value.FieldNames[name]; emitted != "" {
				name = emitted
			}
			fields[name] = field
		}
		for name, field := range target.Fields {
			if emitted := target.FieldNames[name]; emitted != "" {
				name = emitted
			}
			if other, ok := fields[name]; ok && !c.conversionContractsMatch(field, other, visiting) {
				return false
			}
		}
	}
	return true
}
