package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkMultipleCallArguments(expr *ast.CallExpr, name string, callable, values Type) {
	minimum := len(callable.Parameters)
	if callable.Variadic {
		minimum--
	}
	if len(values.Results) < minimum || (!callable.Variadic && len(values.Results) != minimum) {
		c.report(expr.Span, fmt.Sprintf("multiple-result argument count mismatch for %s: got %d values for %d parameters", name, len(values.Results), len(callable.Parameters)))
		return
	}
	for i, actual := range values.Results {
		index := i
		if callable.Variadic && index >= minimum {
			index = minimum
		}
		if index < 0 || index >= len(callable.Parameters) {
			continue
		}
		expected := callable.Parameters[index]
		// The Go call forwards all values directly; per-slot source coercions
		// would need explicit bindings before this call.
		if !c.isAssignable(expected, actual) || !assignable(expected, actual) {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("cannot pass result %d of type %s as %s; destructure values requiring conversion", i+1, actual.String(), expected.String()))
		}
	}
}
