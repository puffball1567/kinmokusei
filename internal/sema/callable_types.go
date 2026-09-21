package sema

import gotypes "go/types"

// Check source success payloads before Go storage comparison can erase nullable
// qualifiers. The normal assignability check still enforces named identity.
func compatibleResultFunctionTypes(target, value Type) bool {
	if target.Kind == Function && value.Kind == Function && target.Result != nil && value.Result != nil &&
		(target.Result.Kind == MultiValue || value.Result.Kind == MultiValue) {
		targetResult, valueResult := *target.Result, *value.Result
		if targetResult.Kind == Result {
			targetResult = methodResultStorageShape(targetResult)
		}
		if valueResult.Kind == Result {
			valueResult = methodResultStorageShape(valueResult)
		}
		target.Result, value.Result = &targetResult, &valueResult
		return sameConstraintNullability(target, value)
	}
	if target.Kind != Function || value.Kind != Function || target.Result == nil || value.Result == nil || target.Result.Kind != Result || value.Result.Kind != Result {
		return true
	}
	return sameConstraintNullability(target, value) && exactType(*target.Result, *value.Result)
}

// Native defined function types retain their source signature, including Result
// effects. Reconstructing them only from Go would turn Result<T> into raw
// multiple values and erase the source contract used for calls and contexts.
func (c *Checker) callableType(value Type) Type {
	if value.Kind != GoNamed || value.GoType == nil {
		return value
	}
	if _, callable := value.GoType.Underlying().(*gotypes.Signature); !callable {
		return value
	}
	if named, ok := gotypes.Unalias(value.GoType).(*gotypes.Named); ok {
		if symbol := c.nativeTypes[named.Obj().Name()]; symbol != nil && named.Origin() == symbol.goNamed {
			underlying := c.nativeDefinedUnderlying(symbol, value)
			if underlying.Kind == Function {
				underlying.GoType, underlying.Name = value.GoType, value.Name
				underlying.TypeArguments = value.TypeArguments
				return underlying
			}
		}
	}
	if converted, err := kinmokuseiTypeFromGo(value.GoType); err == nil && converted.Kind == Function {
		return converted
	}
	return value
}
