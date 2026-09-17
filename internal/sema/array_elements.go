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
