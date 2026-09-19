package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkSelect(stmt *ast.SelectStmt) {
	defaultSeen := false
	entryFlow := c.snapshotNullableFlow()
	continuing := make([]nullableFlowSnapshot, 0, len(stmt.Cases))
	c.breakFlowContexts = append(c.breakFlowContexts, breakFlowContext{})
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		c.restoreNullableFlow(entryFlow)
		c.pushScope()
		switch clause.Kind {
		case ast.SelectDefault:
			if defaultSeen {
				c.report(clause.Span, "select may contain at most one default case")
			}
			defaultSeen = true
		case ast.SelectSend:
			send := &ast.ChannelSendStmt{Channel: clause.Channel, Value: clause.Value, Span: clause.Span}
			c.checkStatement(send)
			clause.Value = send.Value // Keep contextual conversions such as class upcasts.
		case ast.SelectReceive:
			c.checkSelectReceive(clause)
		}
		c.breakableDepth++
		c.checkBlock(clause.Body, false)
		c.breakableDepth--
		c.popScope()
		if !definitelyReturns(clause.Body) {
			continuing = append(continuing, c.snapshotNullableFlow())
		}
	}
	flow := c.breakFlowContexts[len(c.breakFlowContexts)-1]
	c.breakFlowContexts = c.breakFlowContexts[:len(c.breakFlowContexts)-1]
	continuing = append(continuing, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
}

func (c *Checker) checkSelectReceive(clause *ast.SelectCase) {
	count := len(clause.Bindings)
	if !clause.Declare {
		count = len(clause.Targets)
	}
	checked := count == 2
	receive := &ast.UnaryExpr{Operator: "<-", Operand: clause.Channel, Span: clause.Channel.GetSpan()}
	value := c.checkChannelReceive(receive, checked)
	if count == 0 {
		return
	}
	if count != 1 && count != 2 {
		c.report(clause.Span, fmt.Sprintf("select receive expects zero, one, or two targets, got %d", count))
		return
	}
	var results []Type
	if checked {
		results = c.multipleResults(value, count, clause.Channel.GetSpan())
	} else {
		results = []Type{c.singleValue(value, clause.Channel.GetSpan())}
	}
	if clause.Declare {
		for i := range clause.Bindings {
			binding := &clause.Bindings[i]
			if binding.Name == "_" {
				continue
			}
			result := Type{Kind: Invalid, Name: "<invalid>"}
			if i < len(results) {
				result = defaultLiteralType(results[i])
			}
			binding.ResolvedType = typeRefFromType(result, binding.Span)
			c.declareSelectLocal(binding.Name, result, clause.Constant, clause, i, binding.Span)
		}
		return
	}
	for i, target := range clause.Targets {
		if identifier, ok := target.(*ast.IdentifierExpr); ok && identifier.Name == "_" {
			continue
		}
		targetType := c.checkAssignmentTarget(target)
		if i < len(results) {
			c.requireAssignable(targetType, results[i], target.GetSpan())
			c.updateAssignmentFlow(target, results[i])
		}
	}
}

func (c *Checker) checkTypeSwitch(stmt *ast.TypeSwitchStmt) {
	value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
	contract := underlyingGoInterface(value.GoType)
	if value.Kind != Invalid && contract == nil {
		c.report(stmt.Value.GetSpan(), fmt.Sprintf("type switch requires a Go interface value, got %s", value.String()))
	}
	defaultSeen := false
	nilSeen := false
	var caseTypes []gotypes.Type
	entryFlow := c.snapshotNullableFlow()
	continuing := make([]nullableFlowSnapshot, 0, len(stmt.Cases)+1)
	c.breakFlowContexts = append(c.breakFlowContexts, breakFlowContext{})
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		c.restoreNullableFlow(entryFlow)
		c.pushScope()
		switch {
		case clause.Default:
			if defaultSeen {
				c.report(clause.Span, "type switch may contain at most one default case")
			}
			defaultSeen = true
		case clause.Nil:
			if nilSeen {
				c.report(clause.Span, "type switch may contain at most one nil case")
			}
			nilSeen = true
		default:
			caseType := c.resolveType(clause.Type)
			goCaseType, ok := goTypeOf(caseType)
			if !ok {
				c.report(clause.Type.Span, fmt.Sprintf("type switch case type %s cannot be represented as a Go type", caseType.String()))
			} else {
				for _, previous := range caseTypes {
					if gotypes.Identical(previous, goCaseType) {
						c.report(clause.Type.Span, fmt.Sprintf("duplicate type switch case %s", caseType.String()))
						break
					}
				}
				caseTypes = append(caseTypes, goCaseType)
				if contract != nil && !gotypes.AssertableTo(contract, goCaseType) {
					c.report(clause.Type.Span, fmt.Sprintf("Go interface %s cannot contain type switch case %s", value.String(), caseType.String()))
				}
			}
			if clause.Name != "_" {
				c.declareTypeSwitchLocal(clause.Name, caseType, clause.Constant, clause, clause.NameSpan)
			}
		}
		c.breakableDepth++
		c.checkBlock(clause.Body, false)
		c.breakableDepth--
		c.popScope()
		if !definitelyReturns(clause.Body) {
			continuing = append(continuing, c.snapshotNullableFlow())
		}
	}
	if !defaultSeen {
		continuing = append(continuing, entryFlow)
	}
	flow := c.breakFlowContexts[len(c.breakFlowContexts)-1]
	c.breakFlowContexts = c.breakFlowContexts[:len(c.breakFlowContexts)-1]
	continuing = append(continuing, flow.breaks...)
	c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
}
