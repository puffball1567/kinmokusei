package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) boundaryTypeRef(value Type, span source.Span) ast.TypeRef {
	ref := typeRefFromType(value, span)
	if value.Kind == MultiValue {
		for i, slot := range value.Results {
			ref.GoResults[i] = c.boundaryTypeRef(slot, span)
		}
	} else if !c.requiresUniformNativeNonNull(value, nil) {
		mask, ambiguous := c.mixedNonNullKinds(value)
		if ambiguous {
			c.report(span, "cannot validate non-null and raw nil-capable alternatives of the same storage kind; use a concrete type or separate uniform constraints")
		}
		ref.NonNullKinds = mask
		c.usesNonNullKinds = c.usesNonNullKinds || mask != 0
	}
	return ref
}

// Reflect storage kinds can select different non-null policies without
// rejecting scalar zeros or legitimate raw Go nils in the same type set.
// Ambiguous kinds need a concrete source contract rather than a runtime guess.
func (c *Checker) mixedNonNullKinds(value Type) (uint8, bool) {
	parameter, ok := value.GoType.(*gotypes.TypeParam)
	if !ok {
		return 0, false
	}
	var required, raw uint8
	for _, term := range c.collectionTerms(parameter) {
		kind := c.nonNullStorageKind(term)
		if c.requiresNativeNonNull(term, nil) {
			if kind == 0 {
				return 0, true
			}
			required |= kind
		} else {
			raw |= kind
		}
	}
	return required, required&raw != 0
}

func (c *Checker) nonNullStorageKind(value Type) uint8 {
	storage, ok := c.goTypeForNativeStorage(baseOfNullable(value))
	if !ok {
		storage, ok = goTypeOf(baseOfNullable(value))
	}
	if !ok {
		return 0
	}
	switch gotypes.Unalias(storage).Underlying().(type) {
	case *gotypes.Pointer:
		return 1
	case *gotypes.Signature:
		return 2
	case *gotypes.Interface:
		return 4
	case *gotypes.Map:
		return 8
	case *gotypes.Slice:
		return 16
	case *gotypes.Chan:
		return 32
	}
	return 0
}
