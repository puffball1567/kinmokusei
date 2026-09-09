package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Constructors and generic method helpers use the owner type in a function
// signature, where a same-named type parameter would hide that type in Go.
func (c *Checker) checkOwnerTypeParameterNames(owner string, parameters []ast.TypeParameter) {
	for _, parameter := range parameters {
		if generatedIdentifier(parameter.Name) == owner {
			c.report(parameter.Span, fmt.Sprintf("type parameter %q conflicts with enclosing type %s in generated Go; use a different parameter name", parameter.Name, owner))
		}
	}
}
