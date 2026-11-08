package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkParameterBoundary(parameter *ast.Parameter, value Type) {
	if parameter.Variadic && value.Kind == Array && value.Element != nil {
		value = *value.Element
	}
	// A blank parameter discards its value; it cannot expose nil to the body.
	parameter.NonNull = parameter.Name != "_" && c.requiresUniformNativeNonNull(value, nil)
}

// An entry check must apply to every allowed instantiation. In contrast to
// rejecting a nil literal, one native non-null term in a union is insufficient:
// another term may be a raw Go pointer/function that legitimately accepts nil.
func (c *Checker) requiresUniformNativeNonNull(value Type, visiting map[gotypes.Type]bool) bool {
	if !isNilable(value) {
		return false
	}
	if value.Kind != TypeParameter {
		return c.requiresNativeNonNull(value, nil)
	}
	parameter, ok := value.GoType.(*gotypes.TypeParam)
	if !ok || visiting[parameter] {
		return false
	}
	terms := c.collectionTerms(parameter)
	if len(terms) == 0 {
		return false
	}
	if visiting == nil {
		visiting = map[gotypes.Type]bool{}
	}
	visiting[parameter] = true
	defer delete(visiting, parameter)
	for _, term := range terms {
		if !c.requiresUniformNativeNonNull(term, visiting) {
			return false
		}
	}
	return true
}
