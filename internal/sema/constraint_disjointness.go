package sema

import gotypes "go/types"

// Prove only stable mismatches. Substituting a type parameter can change an
// element, but cannot turn a slice into an array, change an array's length or
// change channel direction. Unknown pairs must remain deferred/rejected.
func constraintTermsProvablyDisjoint(left, right *gotypes.Term) bool {
	a, b := gotypes.Unalias(left.Type()), gotypes.Unalias(right.Type())
	if _, ok := a.(*gotypes.TypeParam); ok {
		return false
	}
	if _, ok := b.(*gotypes.TypeParam); ok {
		return false
	}
	if left.Tilde() || right.Tilde() {
		a, b = a.Underlying(), b.Underlying()
	}
	return constraintTypesProvablyDistinct(a, b)
}

func constraintTypesProvablyDistinct(left, right gotypes.Type) bool {
	left, right = gotypes.Unalias(left), gotypes.Unalias(right)
	if _, ok := left.(*gotypes.TypeParam); ok {
		return false
	}
	if _, ok := right.(*gotypes.TypeParam); ok {
		return false
	}
	if named, ok := left.(*gotypes.Named); ok {
		other, ok := right.(*gotypes.Named)
		if !ok || named.Origin() != other.Origin() {
			return true
		}
		if named.TypeArgs().Len() != other.TypeArgs().Len() {
			return false
		}
		for i := 0; i < named.TypeArgs().Len(); i++ {
			if constraintTypesProvablyDistinct(named.TypeArgs().At(i), other.TypeArgs().At(i)) {
				return true
			}
		}
		return false
	}
	if _, ok := right.(*gotypes.Named); ok {
		return true
	}
	switch left := left.(type) {
	case *gotypes.Basic:
		return !gotypes.Identical(left, right)
	case *gotypes.Slice:
		other, ok := right.(*gotypes.Slice)
		return !ok || constraintTypesProvablyDistinct(left.Elem(), other.Elem())
	case *gotypes.Array:
		other, ok := right.(*gotypes.Array)
		return !ok || left.Len() != other.Len() || constraintTypesProvablyDistinct(left.Elem(), other.Elem())
	case *gotypes.Pointer:
		other, ok := right.(*gotypes.Pointer)
		return !ok || constraintTypesProvablyDistinct(left.Elem(), other.Elem())
	case *gotypes.Chan:
		other, ok := right.(*gotypes.Chan)
		return !ok || left.Dir() != other.Dir() || constraintTypesProvablyDistinct(left.Elem(), other.Elem())
	case *gotypes.Map:
		other, ok := right.(*gotypes.Map)
		return !ok || constraintTypesProvablyDistinct(left.Key(), other.Key()) || constraintTypesProvablyDistinct(left.Elem(), other.Elem())
	default:
		// In particular, do not unfold recursive named types or attempt to
		// solve structural interface identity here.
		return false
	}
}
