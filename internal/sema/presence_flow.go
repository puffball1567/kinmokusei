package sema

import (
	"fmt"
	"go/constant"

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

// Normalize a checked condition without reevaluating its operands. Compile-time
// constants may qualify a guard; copying a runtime flag does not copy its proof.
func (c *Checker) applyPresenceCondition(expression ast.Expression) (bool, bool) {
	identifier, positive := c.presenceGuard(expression)
	if identifier == nil {
		return positive, false
	}
	guard, _, found := c.presenceBinding(identifier.Name)
	if !found || identifier.ResolvedDeclaration != guard.declarationSpan || guard.presence.payload.Start.Line == 0 || guard.flowEscaped {
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

func (c *Checker) presenceGuard(expression ast.Expression) (*ast.IdentifierExpr, bool) {
	positive := true
	for steps := 0; steps < 512; steps++ {
		switch expr := expression.(type) {
		case *ast.IdentifierExpr:
			return expr, positive
		case *ast.UnaryExpr:
			if expr.Operator != "!" {
				return nil, positive
			}
			positive = !positive
			expression = expr.Operand
		case *ast.BinaryExpr:
			if expr.Operator != "==" && expr.Operator != "===" && expr.Operator != "!=" && expr.Operator != "!==" {
				return nil, positive
			}
			info, known := c.scalarConstant(expr.Right)
			expression = expr.Left
			if !known || info.Value.Kind() != constant.Bool {
				info, known = c.scalarConstant(expr.Left)
				expression = expr.Right
			}
			if !known || info.Value.Kind() != constant.Bool {
				return nil, positive
			}
			equality := expr.Operator == "==" || expr.Operator == "==="
			if equality != constant.BoolVal(info.Value) {
				positive = !positive
			}
		default:
			return nil, positive
		}
	}
	return nil, positive
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
