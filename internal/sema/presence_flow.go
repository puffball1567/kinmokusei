package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// The payload and success binding refer to each other's declaration identity.
// Reassignment clears the relationship before a new checked pair installs it.
// A verified payload is a snapshot: removing the original map entry cannot
// invalidate it. Nullable payloads keep their own independent null contract.
type presenceFlow struct {
	required, checked bool
	guard, payload    source.Span
}

func checkedPresenceProducer(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.IndexExpr:
		return expression.MapIndex
	case *ast.UnaryExpr:
		return expression.Operator == "<-"
	case *ast.GoTypeAssertionExpr:
		return expression.Checked
	}
	return false
}

func (c *Checker) bindPresencePair(expression ast.Expression, value Type, bindings []ast.Binding) {
	if !checkedPresenceProducer(expression) || value.Kind != MultiValue || len(value.Results) != 2 || len(bindings) != 2 || c.zeroValueSafe(value.Results[0]) || bindings[0].Name == "_" {
		return
	}
	first, firstScope, ok := c.presenceBinding(bindings[0].Name)
	if !ok {
		c.report(bindings[0].Span, "checked values without a safe zero require a local payload binding; verify ok before assigning to global storage")
		return
	}
	if first.constant && first.declarationSpan != bindings[0].Span {
		return
	}
	first.presence = presenceFlow{required: true}
	guard, guardScope, paired := c.presenceBinding(bindings[1].Name)
	if bindings[1].Name != "_" && paired && first.declarationSpan != guard.declarationSpan && (!guard.constant || guard.declarationSpan == bindings[1].Span) && guard.declaredType.IsBoolean() {
		first.presence.guard = guard.declarationSpan
		guard.presence = presenceFlow{payload: first.declarationSpan}
		c.scopes[guardScope][bindings[1].Name] = guard
	}
	c.scopes[firstScope][bindings[0].Name] = first
}

func (c *Checker) presenceBinding(name string) (valueSymbol, int, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if symbol, ok := c.scopes[i][name]; ok {
			return symbol, i, true
		}
	}
	return valueSymbol{}, 0, false
}

func (c *Checker) checkPresenceRead(name string, symbol valueSymbol, span source.Span) {
	if c.unevaluatedDepth == 0 && symbol.presence.required && !symbol.presence.checked {
		c.report(span, fmt.Sprintf("value %q may be absent; check its corresponding ok binding before using it", name))
	}
}

// Recognize direct success tests, boolean negation (handled by checkCondition),
// and equality with boolean constants. Copying a flag does not copy its proof.
func (c *Checker) applyPresenceCondition(expression ast.Expression) (bool, bool) {
	identifier, ok := expression.(*ast.IdentifierExpr)
	positive := true
	if !ok {
		binary, yes := expression.(*ast.BinaryExpr)
		if !yes || (binary.Operator != "==" && binary.Operator != "===" && binary.Operator != "!=" && binary.Operator != "!==") {
			return true, false
		}
		var literal *ast.LiteralExpr
		identifier, literal = c.presenceComparison(binary.Left, binary.Right)
		if identifier == nil || literal == nil {
			return true, false
		}
		positive = (literal.Text == "true") == (binary.Operator == "==" || binary.Operator == "===")
	}
	guard, _, found := c.presenceBinding(identifier.Name)
	if !found || guard.presence.payload.Start.Line == 0 || guard.flowEscaped {
		return positive, false
	}
	for i, scope := range c.scopes {
		for name, payload := range scope {
			if payload.declarationSpan == guard.presence.payload && payload.presence.required && payload.presence.guard == guard.declarationSpan && !payload.flowEscaped {
				payload.presence.checked = true
				c.scopes[i][name] = payload
			}
		}
	}
	return positive, true
}

func (c *Checker) presenceComparison(left, right ast.Expression) (*ast.IdentifierExpr, *ast.LiteralExpr) {
	if id, ok := left.(*ast.IdentifierExpr); ok {
		if literal, yes := right.(*ast.LiteralExpr); yes && literal.Kind == ast.BooleanLiteral {
			return id, literal
		}
	}
	if id, ok := right.(*ast.IdentifierExpr); ok {
		if literal, yes := left.(*ast.LiteralExpr); yes && literal.Kind == ast.BooleanLiteral {
			return id, literal
		}
	}
	return nil, nil
}

func mergePresence(symbol valueSymbol, scope int, name string, branches ...[]map[string]valueSymbol) presenceFlow {
	var result presenceFlow
	result.checked = true
	guardSet, payloadSet := false, false
	for _, branch := range branches {
		candidate := symbol
		if scope < len(branch) {
			if other, ok := branch[scope][name]; ok && other.declarationSpan == symbol.declarationSpan {
				candidate = other
			}
		}
		state := candidate.presence
		if !payloadSet {
			result.payload = state.payload
			payloadSet = true
		} else if result.payload != state.payload {
			result.payload = source.Span{}
		}
		if !state.required {
			continue
		}
		result.required = true
		result.checked = result.checked && state.checked
		if !guardSet {
			result.guard = state.guard
			guardSet = true
		} else if result.guard != state.guard {
			result.guard = source.Span{}
		}
	}
	if !result.required {
		result.checked = false
	}
	return result
}
