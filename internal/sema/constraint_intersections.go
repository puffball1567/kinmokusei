package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Resolve a source operand to a disjoint union and retain the source shape of
// each term: Go storage alone cannot distinguish nullable class elements.
func (c *Checker) resolveConstraintTerm(term ast.TypeSetTerm, resolved Type) ([]*gotypes.Term, []Type, bool) {
	if resolved.Kind == Invalid {
		return nil, nil, false
	}
	goType, ok := goTypeOf(resolved)
	if !ok {
		goType, ok = c.goTypeForNativeStorage(resolved)
	}
	if !ok || goType == nil {
		c.report(term.Span, fmt.Sprintf("constraint term %s cannot be represented as a Go type", formatTypeRefForDiagnostic(term.Type)))
		return nil, nil, false
	}
	goType = gotypes.Unalias(goType)
	if c.rejectIncompleteConstraintTerm(term, goType) {
		return nil, nil, false
	}
	if underlyingGoInterface(goType) != nil {
		c.report(term.Span, fmt.Sprintf("constraint term %s must be a concrete type, not an interface", formatTypeRefForDiagnostic(term.Type)))
		return nil, nil, false
	}
	if term.Underlying && !gotypes.Identical(goType, goType.Underlying()) {
		c.report(term.Span, fmt.Sprintf("underlying constraint term ~%s must name its own underlying type", formatTypeRefForDiagnostic(term.Type)))
		return nil, nil, false
	}
	return []*gotypes.Term{gotypes.NewTerm(term.Underlying, goType)}, []Type{resolved}, true
}

// Intersect disjoint unions. An exact term is narrower than an overlapping
// underlying term; its source shape must accompany it into the normalized set.
// Keeping one normalized union also lets references, inference and range use
// the same representation for both source operators.
func (c *Checker) intersectConstraintTerms(left []*gotypes.Term, leftShapes []Type, right []*gotypes.Term, rightShapes []Type) ([]*gotypes.Term, []Type, string) {
	if sameConstraintTermSet(left, right) {
		// Repeating the same union cannot add a substitution-dependent match.
		// Still check each matching source shape below for nullable conflicts.
		terms, shapes := left, leftShapes
		for i, a := range left {
			for j, b := range right {
				if a.Tilde() == b.Tilde() && gotypes.Identical(a.Type(), b.Type()) && !sameConstraintNullability(c.constraintArgumentShape(leftShapes[i]), c.constraintArgumentShape(rightShapes[j])) {
					return terms, shapes, "constraint intersection has incompatible nullable source types"
				}
			}
		}
		return terms, shapes, ""
	}
	var terms []*gotypes.Term
	var shapes []Type
	problem := ""
	for i, a := range left {
		for j, b := range right {
			if !typeSetTermsOverlap(a, b) {
				// A later substitution could make currently different parameterized
				// terms identical. Do not silently discard such a possible match.
				if constraintTypeHasParameters(a.Type()) || constraintTypeHasParameters(b.Type()) {
					problem = "constraint intersection cannot discard unmatched parameter-dependent terms; instantiate the operands with concrete types first"
				}
				continue
			}
			if !sameConstraintNullability(c.constraintArgumentShape(leftShapes[i]), c.constraintArgumentShape(rightShapes[j])) {
				problem = "constraint intersection has incompatible nullable source types"
			}
			term, shape := a, leftShapes[i]
			if a.Tilde() && !b.Tilde() {
				term, shape = b, rightShapes[j]
			}
			terms = append(terms, term)
			shapes = append(shapes, shape)
		}
	}
	return terms, shapes, problem
}

func sameConstraintTermSet(left, right []*gotypes.Term) bool {
	if len(left) != len(right) {
		return false
	}
	for _, a := range left {
		found := false
		for _, b := range right {
			if a.Tilde() == b.Tilde() && gotypes.Identical(a.Type(), b.Type()) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func constraintTypeHasParameters(value gotypes.Type) bool {
	switch value := gotypes.Unalias(value).(type) {
	case *gotypes.TypeParam:
		return true
	case *gotypes.Named:
		// Do not follow recursive named underlyings; free parameters are
		// represented by the instance's type arguments.
		for i := 0; i < value.TypeArgs().Len(); i++ {
			if constraintTypeHasParameters(value.TypeArgs().At(i)) {
				return true
			}
		}
	case *gotypes.Slice:
		return constraintTypeHasParameters(value.Elem())
	case *gotypes.Array:
		return constraintTypeHasParameters(value.Elem())
	case *gotypes.Pointer:
		return constraintTypeHasParameters(value.Elem())
	case *gotypes.Chan:
		return constraintTypeHasParameters(value.Elem())
	case *gotypes.Map:
		return constraintTypeHasParameters(value.Key()) || constraintTypeHasParameters(value.Elem())
	case *gotypes.Signature:
		return constraintTypeHasParameters(value.Params()) || constraintTypeHasParameters(value.Results())
	case *gotypes.Tuple:
		for i := 0; i < value.Len(); i++ {
			if constraintTypeHasParameters(value.At(i).Type()) {
				return true
			}
		}
	case *gotypes.Struct:
		for i := 0; i < value.NumFields(); i++ {
			if constraintTypeHasParameters(value.Field(i).Type()) {
				return true
			}
		}
	case *gotypes.Interface:
		for i := 0; i < value.NumExplicitMethods(); i++ {
			if constraintTypeHasParameters(value.ExplicitMethod(i).Type()) {
				return true
			}
		}
		for i := 0; i < value.NumEmbeddeds(); i++ {
			if constraintTypeHasParameters(value.EmbeddedType(i)) {
				return true
			}
		}
	}
	return false
}
