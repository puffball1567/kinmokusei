package sema

import gotypes "go/types"

// Go storage proves assignment for the entire type set, including direction,
// length and nominal element identity. Source terms additionally retain the
// nullable contracts erased by that storage representation.
func (c *Checker) constrainedCollectionAssignable(target, value Type) bool {
	parameter, ok := value.GoType.(*gotypes.TypeParam)
	if !ok {
		return false
	}
	storage, ok := c.goTypeForNativeStorage(target)
	if !ok || !gotypes.AssignableTo(parameter, storage) {
		return false
	}
	terms := c.collectionTerms(parameter)
	if len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		if !sameConstraintNullability(target, c.constraintArgumentShape(term)) {
			return false
		}
	}
	return true
}
