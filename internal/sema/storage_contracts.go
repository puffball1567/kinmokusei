package sema

import gotypes "go/types"

// Explicit conversions change nominal identity, not the checked source contract.
// Go's convertibility alone cannot see nullable qualifiers or Result payloads.
func (c *Checker) conversionPreservesSourceContract(target, value Type) bool {
	if value.Kind == Nullable {
		return false
	}
	return c.sourceStorageContractsMatch(target, value)
}

// Source qualifiers are invariant within shared storage and callable contracts.
// The caller separately checks Go assignment/conversion and nominal identity;
// ordinary nullable widening and class upcasts are handled at the value boundary.
func (c *Checker) sourceStorageContractsMatch(target, value Type) bool {
	return c.sourceStorageContractsMatchSeen(target, value, nil)
}

func (c *Checker) sourceStorageContractsMatchSeen(target, value Type, visiting [][2]gotypes.Type) bool {
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
			if !c.sourceStorageContractsMatchSeen(term, value, visiting) {
				return false
			}
		}
		return true
	}
	if parameter, ok := value.GoType.(*gotypes.TypeParam); value.Kind == TypeParameter && ok {
		for _, term := range c.collectionTerms(parameter) {
			if !c.sourceStorageContractsMatchSeen(target, term, visiting) {
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
		if targetStorage != nil && valueStorage != nil {
			// Substitution can instantiate equivalent named types as distinct
			// Go objects, so pointer identity is not a valid cycle key.
			for _, pair := range visiting {
				if gotypes.Identical(pair[0], targetStorage) && gotypes.Identical(pair[1], valueStorage) {
					return true
				}
			}
			visiting = append(visiting, [2]gotypes.Type{targetStorage, valueStorage})
		}
	}
	target, value = c.constraintArgumentShape(target), c.constraintArgumentShape(value)
	if textContractMismatch(target, value) {
		return false
	}
	records := (target.Kind == Object || target.Kind == Struct) && (value.Kind == Object || value.Kind == Struct)
	if target.Kind != value.Kind && !records {
		return true // The caller checks non-qualifier shape differences.
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
		if pair[0] != nil && pair[1] != nil && !c.sourceStorageContractsMatchSeen(*pair[0], *pair[1], visiting) {
			return false
		}
	}
	for _, pair := range [][2][]Type{{target.Parameters, value.Parameters}, {target.TypeArguments, value.TypeArguments}, {target.Results, value.Results}} {
		if len(pair[0]) == len(pair[1]) {
			for i := range pair[0] {
				if !c.sourceStorageContractsMatchSeen(pair[0][i], pair[1][i], visiting) {
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
			if other, ok := fields[name]; ok && !c.sourceStorageContractsMatchSeen(field, other, visiting) {
				return false
			}
		}
	}
	return true
}
