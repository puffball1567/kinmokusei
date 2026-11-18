package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

func (c *Checker) sourceConstraintStorageBoundDeferred(ref ast.TypeRef) bool {
	if ref.Qualifier != "" || ref.Nullable || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() || ref.IsGoStruct() {
		return false
	}
	if _, parameter := c.lookupTypeParameter(ref.Name); parameter {
		return false
	}
	return c.sourceConstraintStorageDeferred(c.interfaces[ref.Name])
}

// Source struct identities exist before their layouts. Delay constraints using
// those layouts (also through aliases, dependent bounds and containers), then
// install them before validating any early type applications. Never substitute
// an empty struct/interface for an unresolved source contract.
func (c *Checker) sourceConstraintStorageDeferred(symbol *interfaceSymbol) bool {
	if c.structGoTypesFinalized || symbol == nil || symbol.constraintDeclaration == nil {
		return false
	}
	type item struct {
		ref    ast.TypeRef
		shadow map[string]bool
	}
	var pending []item
	push := func(ref ast.TypeRef, shadow map[string]bool) { pending = append(pending, item{ref, shadow}) }
	parameters := func(values []ast.TypeParameter) map[string]bool {
		shadow := map[string]bool{}
		for _, parameter := range values {
			shadow[parameter.Name] = true
		}
		for _, parameter := range values {
			if parameter.Constraint != nil {
				push(*parameter.Constraint, shadow)
			}
		}
		return shadow
	}
	declaration := symbol.constraintDeclaration
	shadow := parameters(declaration.TypeParameters)
	for _, term := range declaration.Terms {
		push(term.Type, shadow)
	}
	seen := map[string]bool{}
	for budget := 32768; len(pending) != 0; budget-- {
		if budget == 0 {
			// Defer an unproved dependency to the final pass.
			return true
		}
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		ref := current.ref
		for _, child := range []*ast.TypeRef{ref.Element, ref.Pointee, ref.Return} {
			if child != nil {
				push(*child, current.shadow)
			}
		}
		for _, group := range [][]ast.TypeRef{ref.GenericArguments, ref.Parameters, ref.GoResults} {
			for _, child := range group {
				push(child, current.shadow)
			}
		}
		for _, field := range ref.ObjectFields {
			push(field.Type, current.shadow)
		}
		if ref.Qualifier != "" || ref.Name == "" || current.shadow[ref.Name] {
			continue
		}
		if c.structs[ref.Name] != nil {
			return true
		}
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		if dependency := c.interfaces[ref.Name]; dependency != nil && dependency.constraintDeclaration != nil {
			decl := dependency.constraintDeclaration
			scope := parameters(decl.TypeParameters)
			for _, term := range decl.Terms {
				push(term.Type, scope)
			}
		}
		if dependency := c.nativeTypes[ref.Name]; dependency != nil && dependency.declaration != nil {
			decl := dependency.declaration
			push(decl.Underlying, parameters(decl.TypeParameters))
		}
	}
	return false
}
