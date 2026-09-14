package sema

// Method contracts are invariant in both parameters and results. Ordinary
// assignment compatibility is insufficient here: nested source qualifiers may
// be erased by Go storage, and sameType deliberately accepts either direction.
func identicalMethodSignature(left, right Type) bool {
	if !sameConstraintNullability(left, right) {
		return false
	}
	if left.Kind == Function && right.Kind == Function {
		if left.Generic != right.Generic || len(left.TypeParameters) != len(right.TypeParameters) || left.Variadic != right.Variadic || len(left.Parameters) != len(right.Parameters) || left.Result == nil || right.Result == nil {
			return false
		}
		for i := range left.Parameters {
			if !identicalMethodSignature(left.Parameters[i], right.Parameters[i]) {
				return false
			}
		}
		return identicalMethodSignature(*left.Result, *right.Result)
	}
	// A source Result method may implement Go's (T, error) or error return.
	// Compare the source payload before its nullable qualifiers disappear.
	if left.Kind == Result && right.Kind != Result {
		return identicalMethodSignature(methodResultStorageShape(left), right)
	}
	if right.Kind == Result && left.Kind != Result {
		return identicalMethodSignature(left, methodResultStorageShape(right))
	}
	if left.Kind == MultiValue && right.Kind == MultiValue {
		if len(left.Results) != len(right.Results) {
			return false
		}
		for i := range left.Results {
			if !identicalMethodSignature(left.Results[i], right.Results[i]) {
				return false
			}
		}
		return true
	}
	if left.Kind == right.Kind {
		for _, pair := range [][2]*Type{{left.Element, right.Element}, {left.Key, right.Key}, {left.Result, right.Result}} {
			if (pair[0] == nil) != (pair[1] == nil) {
				return false
			}
			if pair[0] != nil && !identicalMethodSignature(*pair[0], *pair[1]) {
				return false
			}
		}
		if len(left.TypeArguments) != len(right.TypeArguments) {
			return false
		}
		for i := range left.TypeArguments {
			if !identicalMethodSignature(left.TypeArguments[i], right.TypeArguments[i]) {
				return false
			}
		}
		if left.Kind == Object {
			if len(left.Fields) != len(right.Fields) {
				return false
			}
			for name, field := range left.Fields {
				other, ok := right.Fields[name]
				if !ok || !identicalMethodSignature(field, other) {
					return false
				}
			}
		}
	}
	return exactType(left, right)
}

func methodResultStorageShape(value Type) Type {
	if value.Element == nil {
		return Type{Kind: Invalid}
	}
	if value.Element.Kind == Void {
		return builtins["error"]
	}
	return Type{Kind: MultiValue, Results: []Type{*value.Element, builtins["error"]}}
}
