package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkMatchSequence(pattern *ast.MatchPattern, subject Type) bool {
	shape := c.constraintArgumentShape(subject)
	pattern.FixedLength = shape.Kind == FixedArray
	hasRest := pattern.SequenceRestIndex() >= 0
	required := len(pattern.Elements)
	if hasRest {
		required--
	}
	fixedLengthFits := pattern.FixedLength && int64(required) == shape.Length
	if hasRest && pattern.FixedLength && int64(required) <= shape.Length {
		fixedLengthFits = true
	}
	element := Type{Kind: Invalid}
	switch shape.Kind {
	case Array, FixedArray:
		if shape.Element != nil {
			if c.sequencePatternContractsAgree(subject, shape) {
				element = *shape.Element
				inheritGoQualifier(&element, subject)
			} else {
				c.report(pattern.Span, "sequence patterns require identical element types and source contracts in every constraint alternative")
			}
		}
		if pattern.FixedLength && !fixedLengthFits {
			if hasRest {
				c.report(pattern.Span, fmt.Sprintf("sequence pattern needs at least %d elements, but %s has length %d", required, subject.String(), shape.Length))
			} else {
				c.report(pattern.Span, fmt.Sprintf("sequence pattern has %d elements; %s requires %d", required, subject.String(), shape.Length))
			}
		}
	case Invalid:
	default:
		c.report(pattern.Span, fmt.Sprintf("sequence pattern requires an array or slice, got %s", subject.String()))
	}
	allElements := true
	for i := range pattern.Elements {
		part := &pattern.Elements[i]
		value := element
		if part.Kind == ast.MatchSequenceRest && element.Kind != Invalid {
			value = Type{Kind: Array, Name: "array", Element: &element}
		}
		allElements = c.checkMatchPattern(part, value, true) && allElements
	}
	pattern.CoversSliceLength = shape.Kind == Array && element.Kind != Invalid && allElements
	return allElements && (fixedLengthFits || pattern.CoversSliceLength && hasRest && required == 0)
}

func (c *Checker) sequencePatternContractsAgree(subject, shape Type) bool {
	parameter, ok := subject.GoType.(*gotypes.TypeParam)
	if subject.Kind != TypeParameter || !ok {
		return true
	}
	for _, term := range c.collectionTerms(parameter) {
		alternative := c.constraintArgumentShape(term)
		if alternative.Kind != shape.Kind || alternative.Element == nil || alternative.Length != shape.Length ||
			!c.identicalCollectionElement(*shape.Element, *alternative.Element) {
			return false
		}
	}
	return true
}

// An unfiltered slice pattern covers an exact length or every length from a
// lower bound. Together they are exhaustive when no shorter length is absent.
type sliceMatchCoverage struct {
	exact          map[int]bool
	firstUncovered int
	tailStartsAt   int
	hasTail        bool
}

func (c sliceMatchCoverage) exhaustive() bool {
	return c.hasTail && c.firstUncovered >= c.tailStartsAt
}

func (c *sliceMatchCoverage) add(pattern *ast.MatchPattern) {
	length := len(pattern.Elements)
	if pattern.SequenceRestIndex() >= 0 {
		minimum := length - 1
		if !c.hasTail || minimum < c.tailStartsAt {
			c.tailStartsAt = minimum
			c.hasTail = true
		}
		return
	}
	if c.exact == nil {
		c.exact = map[int]bool{}
	}
	c.exact[length] = true
	for c.exact[c.firstUncovered] {
		c.firstUncovered++
	}
}
