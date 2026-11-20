package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) declareNamedResults(ref *ast.TypeRef) {
	if len(ref.NamedResults) == 0 {
		return
	}
	c.namedResultType = ref
	values := []Type{c.result}
	if c.result.Kind == MultiValue {
		values = c.result.Results
	}
	for i, binding := range ref.NamedResults {
		value := values[i]
		switch {
		case value.Kind == Void || value.Kind == Result || value.Kind == MultiValue || containsTaskType(value):
			c.report(binding.Span, "named results must use ordinary value types")
		case !c.namedResultZeroSafe(value):
			c.report(binding.Span, fmt.Sprintf("named result %q starts at its zero value, which is invalid for %s", binding.Name, value.String()))
		}
		if binding.Name == "_" {
			continue
		}
		if _, exists := c.lookupTypeParameter(binding.Name); exists {
			c.report(binding.Span, fmt.Sprintf("named result %q conflicts with a type parameter", binding.Name))
		}
		c.declareLocal(binding.Name, value, false, nil, binding.Span)
	}
}

func (c *Checker) checkBareNamedReturn(span source.Span) {
	for _, result := range c.namedResultType.NamedResults {
		if result.Name == "_" {
			continue
		}
		if symbol, ok := c.lookupSymbol(result.Name, span); ok && symbol.declarationSpan != result.Span {
			c.report(span, fmt.Sprintf("bare return cannot use shadowed named result %q; write an explicit return", result.Name))
		}
	}
}
