package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func isAllowedExpressionStatement(expression ast.Expression) bool {
	if _, ok := expression.(*ast.CallExpr); ok {
		return true
	}
	if await, ok := expression.(*ast.AwaitExpr); ok {
		return await.Void
	}
	unary, ok := expression.(*ast.UnaryExpr)
	return ok && unary.Operator == "<-"
}

func (c *Checker) checkMultiVariableDeclaration(stmt *ast.MultiVariableDecl) {
	value := c.checkMultipleValueExpression(stmt.Value)
	results := c.multipleResults(value, len(stmt.Bindings), stmt.Value.GetSpan())
	for i := range stmt.Bindings {
		binding := &stmt.Bindings[i]
		if binding.Name == "_" {
			continue
		}
		result := Type{Kind: Invalid, Name: "<invalid>"}
		if i < len(results) {
			result = defaultLiteralType(results[i])
		}
		binding.ResolvedType = typeRefFromType(result, binding.Span)
		c.declareMultiLocal(binding.Name, result, stmt.Constant, stmt, i, binding.Span)
		c.updateIdentifierFlow(binding.Name, binding.Span, result)
	}
}

func (c *Checker) checkMultiAssignment(stmt *ast.MultiAssignmentStmt) {
	value := c.checkMultipleValueExpression(stmt.Value)
	results := c.multipleResults(value, len(stmt.Bindings), stmt.Value.GetSpan())
	for i := range stmt.Bindings {
		binding := &stmt.Bindings[i]
		if binding.Name == "_" {
			continue
		}
		symbol, exists := c.lookupAssignmentSymbol(binding.Name, binding.Span)
		if !exists {
			c.report(binding.Span, fmt.Sprintf("undefined name %q", binding.Name))
			continue
		}
		if symbol.constant {
			c.report(binding.Span, fmt.Sprintf("cannot assign to const %q", binding.Name))
		}
		binding.ResolvedDeclaration = symbol.declarationSpan
		if i < len(results) {
			declared := symbol.declaredType
			if declared.Kind == Invalid {
				declared = symbol.typeInfo
			}
			c.requireAssignable(declared, results[i], binding.Span)
			c.updateIdentifierFlow(binding.Name, binding.Span, results[i])
		}
	}
}

func (c *Checker) checkMultipleValueExpression(expression ast.Expression) Type {
	if receive, ok := expression.(*ast.UnaryExpr); ok && receive.Operator == "<-" {
		return c.checkChannelReceive(receive, true)
	}
	if index, ok := expression.(*ast.IndexExpr); ok {
		return c.checkIndex(index, true)
	}
	return c.checkExpression(expression)
}

func (c *Checker) multipleResults(value Type, bindings int, span source.Span) []Type {
	if value.Kind == Invalid {
		return nil
	}
	if value.Kind == Result && value.Element != nil {
		results := []Type{builtins["error"]}
		if value.Element.Kind != Void {
			results = []Type{*value.Element, builtins["error"]}
		}
		if len(results) != bindings {
			c.report(span, fmt.Sprintf("Result binding count mismatch: got %d bindings for %d results", bindings, len(results)))
		}
		return results
	}
	if value.Kind != MultiValue {
		c.report(span, fmt.Sprintf("multiple binding requires a multiple-return value, got %s", value.String()))
		return nil
	}
	if len(value.Results) != bindings {
		c.report(span, fmt.Sprintf("multiple binding count mismatch: got %d bindings for %d results", bindings, len(value.Results)))
	}
	return value.Results
}

func (c *Checker) checkAssignmentTarget(expr ast.Expression) Type {
	if identifier, ok := expr.(*ast.IdentifierExpr); ok {
		symbol, exists := c.lookupAssignmentSymbol(identifier.Name, identifier.Span)
		if !exists {
			c.report(identifier.Span, fmt.Sprintf("undefined name %q", identifier.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if symbol.constant {
			c.report(identifier.Span, fmt.Sprintf("cannot assign to const %q", identifier.Name))
		}
		identifier.ResolvedDeclaration = symbol.declarationSpan
		if symbol.declaredType.Kind != Invalid {
			return symbol.declaredType
		}
		return symbol.typeInfo
	}
	target := c.checkExpression(expr)
	if member, ok := expr.(*ast.MemberExpr); ok {
		if member.Constant {
			c.report(member.Span, fmt.Sprintf("cannot assign to Go constant %q", member.Name))
		} else if !member.Addressable {
			c.report(member.Span, fmt.Sprintf("member %q is not assignable", member.Name))
		}
		if key, stable := c.stableMemberFlowKey(member); stable {
			if declared, exists := c.memberTypes[key]; exists {
				return declared
			}
		}
	} else if index, ok := expr.(*ast.IndexExpr); ok && !index.Assignable {
		c.report(index.Span, "index expression is not assignable")
	}
	return target
}

func (c *Checker) markAssignmentTargetRead(expr ast.Expression) {
	if identifier, ok := expr.(*ast.IdentifierExpr); ok {
		if symbol, exists := c.lookupSymbol(identifier.Name, identifier.Span); exists {
			identifier.ResolvedDeclaration = symbol.declarationSpan
		}
	}
}

func (c *Checker) checkLoopCondition(expr ast.Expression) {
	condition := c.checkExpression(expr)
	if condition.Kind != Invalid && condition.Kind != Boolean {
		c.report(expr.GetSpan(), fmt.Sprintf("loop condition must be boolean, got %s", condition.String()))
	}
}
