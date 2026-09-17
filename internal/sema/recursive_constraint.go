package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Source struct storage is finalized after source constraints. In particular,
// asking an instance of an unfinished generic named type for its underlying
// type panics in go/types. Diagnose the unsupported dependency before doing so.
func (c *Checker) rejectIncompleteConstraintTerm(term ast.TypeSetTerm, value gotypes.Type) bool {
	if !incompleteConstraintValueType(value, map[gotypes.Type]bool{}) {
		return false
	}
	c.report(term.Span, "constraint term depends on a source value type whose storage is not yet resolved")
	return true
}

func incompleteConstraintValueType(value gotypes.Type, seen map[gotypes.Type]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	switch value := gotypes.Unalias(value).(type) {
	case *gotypes.Named:
		underlying := value.Origin().Underlying()
		if underlying == nil {
			return true
		}
		return incompleteConstraintValueType(value.Underlying(), seen)
	case *gotypes.Array:
		return incompleteConstraintValueType(value.Elem(), seen)
	case *gotypes.Struct:
		for i := 0; i < value.NumFields(); i++ {
			if incompleteConstraintValueType(value.Field(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// Do not ask go/types to compute array/struct comparability through a type
// parameter whose constraint is still being installed. A lazy named constraint
// can otherwise try to resolve itself while holding its own resolution lock.
// Method signatures and pointer terms do not need the parameter's comparability.
func (c *Checker) recursiveComparableConstraint(ref ast.TypeRef, bound gotypes.Type, scope map[string]Type, state map[string]uint8) bool {
	if ref.Qualifier != "" {
		return false
	}
	symbol := c.interfaces[ref.Name]
	if symbol == nil || !symbol.constraintComparable {
		return false
	}
	named, ok := bound.(*gotypes.Named)
	if !ok || named.TypeArgs().Len() != len(symbol.typeParameters) {
		return false
	}
	bindings := map[gotypes.Type]gotypes.Type{}
	for i, parameter := range symbol.typeParameters {
		bindings[parameter.GoType] = named.TypeArgs().At(i)
	}
	pending := map[gotypes.Type]bool{}
	for name, progress := range state {
		if progress == 1 {
			pending[scope[name].GoType] = true
		}
	}
	for _, shape := range symbol.constraintTermTypes {
		if value, ok := goTypeOf(shape); ok && comparableDependsOnPending(value, bindings, pending, map[gotypes.Type]bool{}) {
			return true
		}
	}
	return false
}

func comparableDependsOnPending(value gotypes.Type, bindings map[gotypes.Type]gotypes.Type, pending, seen map[gotypes.Type]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	if replacement := bindings[value]; replacement != nil && replacement != value {
		return comparableDependsOnPending(replacement, bindings, pending, seen)
	}
	if pending[value] {
		return true
	}
	child := func(value gotypes.Type) bool { return comparableDependsOnPending(value, bindings, pending, seen) }
	switch value := gotypes.Unalias(value).(type) {
	case *gotypes.Array:
		return child(value.Elem())
	case *gotypes.Struct:
		for i := 0; i < value.NumFields(); i++ {
			if child(value.Field(i).Type()) {
				return true
			}
		}
	case *gotypes.Named:
		if value.TypeArgs().Len() == 0 {
			return child(value.Underlying())
		}
		specialized := make(map[gotypes.Type]gotypes.Type, len(bindings)+value.TypeArgs().Len())
		for from, to := range bindings {
			specialized[from] = to
		}
		for i := 0; i < value.TypeArgs().Len(); i++ {
			specialized[value.Origin().TypeParams().At(i)] = value.TypeArgs().At(i)
		}
		return comparableDependsOnPending(value.Origin().Underlying(), specialized, pending, seen)
	}
	return false
}
