package sema

import (
	"fmt"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

const resultUsageMessage = "Result values must be consumed with ?, explicitly split, or returned; use _ for intentional discard"

// Blank bindings do not introduce storage or predeclared arrow symbols.
// They still type-check and evaluate their initializer, including its effects.
func (c *Checker) checkDiscard(expression *ast.Expression, annotation ast.TypeRef, propagation bool) int {
	expected := Type{Kind: Invalid, Name: "<inferred>"}
	if annotation.IsSpecified() {
		expected = c.resolveType(annotation)
	}
	var value Type
	if expr, ok := (*expression).(*ast.PropagateExpr); ok && propagation {
		value = c.checkPropagateExpression(expr)
	} else {
		value = c.checkExpressionExpectedSlot(expression, expected)
	}
	span := (*expression).GetSpan()
	if call, ok := (*expression).(*ast.CallExpr); ok && (call.Builtin == ast.ResultOKCall || call.Builtin == ast.ResultFailCall) {
		c.report(span, "Result constructors must be returned, not discarded")
	}
	if annotation.IsSpecified() {
		c.rejectResultValueType(expected, annotation.Span, "variables")
		c.requireAssignable(expected, value, span)
	}
	switch value.Kind {
	case Result:
		if value.Element != nil && value.Element.Kind != Void {
			return 2
		}
	case MultiValue:
		return len(value.Results)
	case Task:
		c.report(span, "Task values cannot be discarded; use await or detach")
	case Void:
		if _, ok := (*expression).(*ast.PropagateExpr); !ok {
			c.report(span, "cannot discard an expression with no value; use a call statement")
		}
	default:
		if !annotation.IsSpecified() {
			inferred := c.inferredVariableType(value, span)
			c.requireAssignable(inferred, value, span)
			c.checkNumericMaterialization(*expression, inferred)
		}
	}
	return 1
}

type resultErrorUse struct {
	name string
	span source.Span
	used *bool
}

// This is a binding-use check, not ownership or path-sensitive error analysis.
// Reuse the source read flags, never the generated Go unused-local cleanup.
// Share the map with lazy arrow checkers and validate only after all bodies.
func (c *Checker) trackResultError(name string, span source.Span) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		symbol, ok := c.scopes[i][name]
		if !ok {
			continue
		}
		var used *bool
		switch {
		case symbol.declaration != nil:
			used = &symbol.declaration.Used
		case symbol.multiDeclaration != nil:
			used = &symbol.multiDeclaration.Bindings[symbol.multiIndex].Used
		case symbol.rangeBinding != nil:
			used = &symbol.rangeBinding.Used
		case symbol.selectCase != nil:
			used = &symbol.selectCase.Bindings[symbol.selectIndex].Used
		case symbol.typeSwitchCase != nil:
			used = &symbol.typeSwitchCase.Used
		}
		// Parameters and nonlocal storage already expose an ordinary error value.
		if used != nil {
			if _, exists := c.resultErrorUses[used]; !exists {
				c.resultErrorUses[used] = resultErrorUse{name: name, span: span, used: used}
			}
		}
		return
	}
}

func (c *Checker) reportUnusedResultErrors() {
	var unused []resultErrorUse
	for _, use := range c.resultErrorUses {
		if !*use.used {
			unused = append(unused, use)
		}
	}
	sort.Slice(unused, func(i, j int) bool {
		if unused[i].span.Path != unused[j].span.Path {
			return unused[i].span.Path < unused[j].span.Path
		}
		return unused[i].span.Start.Offset < unused[j].span.Start.Offset
	})
	for _, use := range unused {
		c.report(use.span, fmt.Sprintf("Result error binding %q is never used; handle or return the error, or explicitly discard it with _", use.name))
	}
}
