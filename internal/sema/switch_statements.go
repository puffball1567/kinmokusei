package sema

import (
	"fmt"
	"go/constant"
	gotoken "go/token"
	gotypes "go/types"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
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
			c.checkStatement(&ast.ChannelSendStmt{Channel: clause.Channel, Value: clause.Value, Span: clause.Span})
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

func (c *Checker) checkValueSwitch(stmt *ast.ValueSwitchStmt) {
	value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
	if value.Kind == Nil || value.Kind == Null {
		c.report(stmt.Value.GetSpan(), "value switch cannot infer a concrete type from nil or null")
	} else if value.Kind != Invalid && !value.IsComparable() {
		c.report(stmt.Value.GetSpan(), fmt.Sprintf("value switch expression type %s is not comparable", value.String()))
	}
	defaultSeen := false
	constantCases := map[string]source.Span{}
	entryFlow := c.snapshotNullableFlow()
	continuing := make([]nullableFlowSnapshot, 0, len(stmt.Cases)+1)
	var fallthroughFlow *nullableFlowSnapshot
	c.breakFlowContexts = append(c.breakFlowContexts, breakFlowContext{})
	for index := range stmt.Cases {
		clause := &stmt.Cases[index]
		clause.FallsThrough = false
		if clause.Body != nil && len(clause.Body.Statements) != 0 {
			if branch, ok := clause.Body.Statements[len(clause.Body.Statements)-1].(*ast.BranchStmt); ok && branch.Kind == ast.FallthroughBranch && index+1 < len(stmt.Cases) {
				clause.FallsThrough = true
				c.validFallthrough[branch] = true
			}
		}
		caseEntry := entryFlow
		if fallthroughFlow != nil {
			caseEntry = c.mergeNullableFlow(entryFlow, entryFlow, *fallthroughFlow)
		}
		c.restoreNullableFlow(caseEntry)
		if clause.Default {
			if defaultSeen {
				c.report(clause.Span, "value switch may contain at most one default case")
			}
			defaultSeen = true
		}
		for _, expression := range clause.Values {
			caseType := c.singleValue(c.checkExpressionExpected(expression, value), expression.GetSpan())
			c.requireAssignable(value, caseType, expression.GetSpan())
			if caseType.Kind != Invalid && caseType.Kind != Nil && caseType.Kind != Null && !caseType.IsComparable() {
				c.report(expression.GetSpan(), fmt.Sprintf("value switch case type %s is not comparable", caseType.String()))
			}
			if integer, known := c.resolvedIntegerConstantValue(expression); known {
				if caseType.Kind != Invalid && value.Kind != UntypedInt && value.IsInteger() && !integerConstantFitsFixedType(integer, value) {
					c.report(expression.GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), value.String()))
				}
				key := "number:" + integer.String()
				if _, duplicate := constantCases[key]; duplicate {
					c.report(expression.GetSpan(), fmt.Sprintf("duplicate value switch case %s", integer.String()))
				} else {
					constantCases[key] = expression.GetSpan()
				}
			} else if key, display, known := c.switchLiteralKey(expression); known {
				if _, duplicate := constantCases[key]; duplicate {
					c.report(expression.GetSpan(), "duplicate value switch case "+display)
				} else {
					constantCases[key] = expression.GetSpan()
				}
			}
		}
		c.breakableDepth++
		c.checkBlock(clause.Body, true)
		c.breakableDepth--
		caseFlow := c.snapshotNullableFlow()
		if clause.FallsThrough && valueSwitchCaseFallthroughReachable(clause) {
			fallthroughFlow = &caseFlow
		} else {
			fallthroughFlow = nil
		}
		if !clause.FallsThrough && !definitelyReturns(clause.Body) {
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

func (c *Checker) switchLiteralKey(expression ast.Expression) (string, string, bool) {
	literal, ok := expression.(*ast.LiteralExpr)
	if !ok {
		identifier, identifierOK := expression.(*ast.IdentifierExpr)
		if !identifierOK {
			return "", "", false
		}
		symbol, found := c.lookupSymbol(identifier.Name, identifier.Span)
		if !found || !symbol.constant || symbol.declaration == nil {
			return "", "", false
		}
		literal, ok = symbol.declaration.Value.(*ast.LiteralExpr)
		if !ok {
			return "", "", false
		}
	}
	switch literal.Kind {
	case ast.StringLiteral:
		value, err := strconv.Unquote(literal.Text)
		if err != nil {
			return "", "", false
		}
		return "string:" + value, strconv.Quote(value), true
	case ast.BooleanLiteral:
		return "boolean:" + literal.Text, literal.Text, true
	case ast.NilLiteral:
		return "nil", "nil", true
	case ast.NullLiteral:
		return "null", "null", true
	case ast.FloatLiteral:
		value := constant.MakeFromLiteral(literal.Text, gotoken.FLOAT, 0)
		if value.Kind() == constant.Unknown {
			return "", "", false
		}
		return "number:" + value.ExactString(), value.ExactString(), true
	default:
		return "", "", false
	}
}
