package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/source"
)

// Arrays and their pointer views must retain the source element contract.
// Reconstructing elements from Go storage alone loses class identity and
// nullable qualifiers, notably after indexing or slicing a viewArray result.
func (c *Checker) fixedArrayElementType(array *gotypes.Array, owner Type, span source.Span) Type {
	shape := owner
	if shape.Kind == GoPointer && shape.Element != nil {
		shape = *shape.Element
	}
	shape = c.constraintArgumentShape(shape)
	if shape.Kind == FixedArray && shape.Element != nil {
		element := *shape.Element
		inheritGoQualifier(&element, owner)
		return element
	}
	return c.collectionElementType(array.Elem(), owner, span)
}

// A copy target may contain arrays of different lengths, but every alternative
// must preserve the same source element contract. Go checks the actual
// conversion against the complete bound; no element conversion is introduced.
func (c *Checker) parameterArrayElement(parameter *gotypes.TypeParam, span source.Span) (Type, bool) {
	terms := c.collectionTerms(parameter)
	var element Type
	for i, term := range terms {
		storage, ok := c.goTypeForNativeStorage(term)
		if !ok {
			return Type{}, false
		}
		array, ok := gotypes.Unalias(storage).Underlying().(*gotypes.Array)
		if !ok {
			return Type{}, false
		}
		next := c.fixedArrayElementType(array, term, span)
		if next.Kind == Invalid || i != 0 && !c.identicalCollectionElement(element, next) {
			return Type{}, false
		}
		element = next
	}
	return element, len(terms) != 0
}
