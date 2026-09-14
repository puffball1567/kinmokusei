package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/source"
)

// Normalize imported embeddings before composing them with source terms. An
// interface embeds intersections; only a Go Union contributes alternatives.
// Work on generic origins so source arguments can replace parameter shapes
// without first losing their nullable qualifiers to Go storage.
func (c *Checker) importedConstraintOperand(value gotypes.Type, arguments []Type, span source.Span) constraintOperand {
	origin := value
	bindings := nativeTypeBindings{}
	goBindings := map[gotypes.Type]gotypes.Type{}
	if named, ok := gotypes.Unalias(value).(*gotypes.Named); ok && len(arguments) != 0 {
		origin = named.Origin()
		for i, argument := range arguments {
			parameter := named.Origin().TypeParams().At(i)
			bindings[parameter] = argument
			goBindings[parameter] = named.TypeArgs().At(i)
		}
	}
	operand, problem := c.normalizeImportedConstraint(origin, bindings, goBindings, map[gotypes.Type]bool{})
	if problem != "" {
		c.report(span, fmt.Sprintf("cannot compose imported constraint: %s", problem))
		return constraintOperand{}
	}
	operand.methods = constraintMethods(underlyingGoInterface(value))
	return operand
}

func (c *Checker) normalizeImportedConstraint(value gotypes.Type, bindings nativeTypeBindings, goBindings map[gotypes.Type]gotypes.Type, active map[gotypes.Type]bool) (constraintOperand, string) {
	value = gotypes.Unalias(value)
	if value == gotypes.Universe.Lookup("comparable").Type() {
		return constraintOperand{valid: true, comparable: true}, ""
	}
	if active[value] {
		return constraintOperand{}, "cyclic type set"
	}
	active[value] = true
	defer delete(active, value)
	switch value := value.(type) {
	case *gotypes.Named:
		if underlyingGoInterface(value) != nil {
			return c.normalizeImportedConstraint(value.Underlying(), bindings, goBindings, active)
		}
	case *gotypes.Interface:
		result := constraintOperand{methods: constraintMethods(value), valid: true}
		for i := 0; i < value.NumEmbeddeds(); i++ {
			operand, problem := c.normalizeImportedConstraint(value.EmbeddedType(i), bindings, goBindings, active)
			if problem != "" {
				return constraintOperand{}, problem
			}
			result.comparable = result.comparable || operand.comparable
			if !operand.restricted {
				continue
			}
			if result.restricted {
				result.terms, result.shapes, problem = c.intersectConstraintTerms(result.terms, result.shapes, operand.terms, operand.shapes)
				if problem != "" {
					return constraintOperand{}, problem
				}
			} else {
				result.terms, result.shapes, result.restricted = operand.terms, operand.shapes, true
			}
		}
		if result.comparable {
			result.terms, result.shapes = comparableConstraintTerms(result.terms, result.shapes)
		}
		return result, ""
	case *gotypes.Union:
		result := constraintOperand{valid: true, restricted: true}
		for i := 0; i < value.Len(); i++ {
			term := value.Term(i)
			operand, problem := c.normalizeImportedConstraint(term.Type(), bindings, goBindings, active)
			if problem != "" {
				return constraintOperand{}, problem
			}
			if !operand.restricted { // A Go union may include an empty method set.
				return constraintOperand{valid: true}, ""
			}
			for j, candidate := range operand.terms {
				if term.Tilde() {
					candidate = gotypes.NewTerm(true, candidate.Type())
				}
				// Named interface alternatives may overlap legally in Go. Keep
				// a disjoint union, with an underlying term absorbing exact ones.
				keep := true
				for k := 0; k < len(result.terms); k++ {
					if !typeSetTermsOverlap(result.terms[k], candidate) {
						continue
					}
					if result.terms[k].Tilde() || !candidate.Tilde() {
						keep = false
						break
					}
					result.terms = append(result.terms[:k], result.terms[k+1:]...)
					result.shapes = append(result.shapes[:k], result.shapes[k+1:]...)
					k--
				}
				if keep {
					result.terms = append(result.terms, candidate)
					result.shapes = append(result.shapes, operand.shapes[j])
				}
			}
		}
		return result, ""
	case *gotypes.TypeParam:
		return constraintOperand{}, "a type parameter cannot be a type-set term"
	}
	shape, err := kinmokuseiTypeFromGo(value)
	if err != nil {
		return constraintOperand{}, err.Error()
	}
	value = substituteConstraintType(value, goBindings)
	shape = substituteNativeTypeParameters(c.restoreNativeRangeType(shape), bindings)
	return constraintOperand{terms: []*gotypes.Term{gotypes.NewTerm(false, value)}, shapes: []Type{shape}, restricted: true, valid: true}, ""
}

// Implements (not Satisfies) tests strict comparability without Go's exception
// allowing an interface value to satisfy a comparable type parameter. Keep
// parameter-dependent terms until substitution can decide their membership.
func comparableConstraintTerms(terms []*gotypes.Term, shapes []Type) ([]*gotypes.Term, []Type) {
	var kept []*gotypes.Term
	var keptShapes []Type
	comparable := underlyingGoInterface(gotypes.Universe.Lookup("comparable").Type())
	for i, term := range terms {
		contract := gotypes.NewInterfaceType(nil, []gotypes.Type{gotypes.NewUnion([]*gotypes.Term{term})}).Complete()
		if constraintTypeHasParameters(term.Type()) || gotypes.Implements(contract, comparable) {
			kept, keptShapes = append(kept, term), append(keptShapes, shapes[i])
		}
	}
	return kept, keptShapes
}
