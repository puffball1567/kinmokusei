package sema

import gotypes "go/types"

// Source references promise a non-null value; their nil-backed Go storage does
// not grant permission to initialize them with nil. Keep representation tests
// separate so nullable syntax, assertions and defensive nil comparisons work.
// Raw pointers, collections and imported Go signatures retain Go semantics.
func (c *Checker) requiresNativeNonNull(value Type, visiting map[gotypes.Type]bool) bool {
	if value.Kind == Class || value.Kind == Interface ||
		value.Kind == Function && value.GoType == nil || isSourceAnonymousInterface(value) {
		return true
	}
	storage := value
	if storage.Kind == Function && storage.GoType != nil {
		// Early constraint terms can retain a callable shape plus the native
		// defined type's storage. Recover its canonical source contract rather
		// than treating that storage as an imported Go callback signature.
		storage.Kind = GoNamed
	}
	if shape, native := c.nativeDefinedShape(storage); native {
		return c.requiresNativeNonNull(shape, visiting)
	}
	if parameter, ok := value.GoType.(*gotypes.TypeParam); value.Kind == TypeParameter && ok {
		if visiting[parameter] {
			return true
		}
		if visiting == nil {
			visiting = map[gotypes.Type]bool{}
		}
		visiting[parameter] = true
		defer delete(visiting, parameter)
		for _, term := range c.collectionTerms(parameter) {
			if c.requiresNativeNonNull(term, visiting) {
				return true
			}
		}
	}
	return false
}
