package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Indexing has a wider type-set rule than range: arrays of different lengths,
// slices and array pointers can share an element without sharing a core type.
// Keep individual source shapes so Go's erased storage cannot drop nullability.
func (c *Checker) collectionTerms(parameter *gotypes.TypeParam) []Type {
	if terms, ok := c.parameterCollectionTerms[parameter]; ok {
		return terms
	}
	operand, problem := c.normalizeImportedConstraint(parameter.Constraint(), nil, nil, map[gotypes.Type]bool{})
	if problem != "" || !operand.restricted {
		return nil
	}
	return operand.shapes
}

func (c *Checker) recordParameterCollectionTerms(parameter *gotypes.TypeParam, ref ast.TypeRef) {
	delete(c.parameterCollectionTerms, parameter)
	terms := c.collectionTerms(parameter)
	var sourceTerms []Type
	if symbol := c.interfaces[ref.Name]; ref.Qualifier == "" && symbol != nil && symbol.constraint && len(ref.GenericArguments) == len(symbol.typeParameters) {
		bindings := make(nativeTypeBindings, len(symbol.typeParameters))
		for i, argument := range ref.GenericArguments {
			bindings[symbol.typeParameters[i].GoType] = c.resolveType(argument)
		}
		for _, term := range symbol.constraintTermTypes {
			sourceTerms = append(sourceTerms, substituteNativeTypeParameters(term, bindings))
		}
	} else if named, ok := gotypes.Unalias(parameter.Constraint()).(*gotypes.Named); ok && named.TypeArgs().Len() == len(ref.GenericArguments) {
		arguments := make([]Type, len(ref.GenericArguments))
		for i, argument := range ref.GenericArguments {
			arguments[i] = c.resolveType(argument)
		}
		sourceTerms = c.importedConstraintOperand(named, arguments, ref.Span).shapes
	}
	// The normalized bound may filter terms through comparable/intersections.
	// Replace only surviving storage shapes with their source counterparts.
	for i, term := range terms {
		storage, ok := c.goTypeForNativeStorage(term)
		if !ok {
			continue
		}
		for _, candidate := range sourceTerms {
			other, ok := c.goTypeForNativeStorage(candidate)
			if ok && gotypes.Identical(storage, other) {
				terms[i] = candidate
				break
			}
		}
	}
	c.setParameterCollectionTerms(parameter, terms)
}

func (c *Checker) setParameterCollectionTerms(parameter *gotypes.TypeParam, terms []Type) {
	if c.parameterCollectionTerms == nil {
		c.parameterCollectionTerms = map[*gotypes.TypeParam][]Type{}
	}
	c.parameterCollectionTerms[parameter] = terms
}

// A mixed collection bound must not accept a nullable element merely because
// its Go representation is the same as the non-nullable element it promises.
// Go satisfaction remains responsible for storage/type-set membership.
func (c *Checker) collectionArgumentNullabilityMatches(parameter *gotypes.TypeParam, argument Type, bindings nativeTypeBindings) bool {
	expected := c.collectionTerms(parameter)
	actual := []Type{argument}
	if other, ok := argument.GoType.(*gotypes.TypeParam); argument.Kind == TypeParameter && ok {
		actual = c.collectionTerms(other)
	}
	for _, value := range actual {
		value = c.constraintArgumentShape(value)
		storage, ok := c.goTypeForNativeStorage(value)
		if !ok {
			continue
		}
		matched, compatible := false, false
		for _, term := range expected {
			shape := c.constraintArgumentShape(substituteNativeTypeParameters(term, bindings))
			other, ok := c.goTypeForNativeStorage(shape)
			if ok && gotypes.Identical(storage.Underlying(), other.Underlying()) {
				matched = true
				compatible = compatible || sameConstraintNullability(shape, value)
			}
		}
		if matched && !compatible {
			return false
		}
	}
	return true
}
