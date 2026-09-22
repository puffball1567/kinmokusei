package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Channel operations share source-aware storage checks, but retain separate
// send, receive and close rules for direction, element types and diagnostics.
func (c *Checker) checkChannelSend(stmt *ast.ChannelSendStmt) {
	channelType := c.singleValue(c.checkExpression(stmt.Channel), stmt.Channel.GetSpan())
	if channelType.Kind == Nullable {
		c.report(stmt.Channel.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before sending", channelType.String()))
		if channelType.Element == nil {
			return
		}
		channelType = *channelType.Element
	}
	goType, ok := c.goTypeForNativeStorage(channelType)
	if !ok {
		c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel send requires a Go channel, got %s", channelType.String()))
		c.checkExpression(stmt.Value)
		return
	}
	if parameter, ok := gotypes.Unalias(goType).(*gotypes.TypeParam); ok {
		element, valid := c.genericChannelElement(parameter, true)
		if !valid {
			c.report(stmt.Channel.GetSpan(), "channel send type parameter requires only send-capable channels with identical element types and nullability")
			c.checkExpression(stmt.Value)
			return
		}
		value := c.checkExpressionExpectedSlot(&stmt.Value, element)
		c.requireAssignable(element, value, stmt.Value.GetSpan())
		return
	}
	channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
	if !ok {
		c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel send requires a Go channel, got %s", channelType.String()))
		c.checkExpression(stmt.Value)
		return
	}
	if channel.Dir() == gotypes.RecvOnly {
		c.report(stmt.Channel.GetSpan(), fmt.Sprintf("cannot send to receive-only channel %s", channelType.String()))
	}
	element, err := kinmokuseiTypeFromGo(channel.Elem())
	if err != nil {
		c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel element type is not supported: %v", err))
		c.checkExpression(stmt.Value)
		return
	}
	if channelType.Element != nil {
		element = *channelType.Element
	}
	value := c.checkExpressionExpectedSlot(&stmt.Value, element)
	c.requireAssignable(element, value, stmt.Value.GetSpan())
}

func (c *Checker) checkChannelReceive(expr *ast.UnaryExpr, checked bool) Type {
	operand := c.singleValue(c.checkExpression(expr.Operand), expr.Operand.GetSpan())
	if operand.Kind == Nullable {
		c.report(expr.Operand.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before receiving", operand.String()))
		if operand.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		operand = *operand.Element
	}
	if operand.Kind == Invalid {
		return operand
	}
	goType, ok := c.goTypeForNativeStorage(operand)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("operator <- requires a Go channel operand, got %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if parameter, ok := gotypes.Unalias(goType).(*gotypes.TypeParam); ok {
		element, valid := c.genericChannelElement(parameter, false)
		if !valid {
			c.report(expr.Span, "channel receive type parameter requires only receive-capable channels with identical element types and nullability")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if checked {
			return Type{Kind: MultiValue, Name: "checked channel receive", Results: []Type{element, builtins["boolean"]}}
		}
		return element
	}
	channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("operator <- requires a Go channel operand, got %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if channel.Dir() == gotypes.SendOnly {
		c.report(expr.Span, fmt.Sprintf("cannot receive from send-only channel %s", operand.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element, err := kinmokuseiTypeFromGo(channel.Elem())
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("channel element type is not supported: %v", err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if operand.Element != nil {
		element = *operand.Element
	}
	if checked {
		return Type{Kind: MultiValue, Name: "checked channel receive", Results: []Type{element, builtins["boolean"]}}
	}
	return element
}

func (c *Checker) checkGoChannelClose(expr *ast.CallExpr) Type {
	expr.Builtin = ast.CloseGoChannelCall
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, "closeGoChannel does not accept type arguments")
	}
	if expr.Expanded {
		c.report(expr.Span, "closeGoChannel does not accept spread arguments")
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("closeGoChannel expects one channel argument, got %d", len(expr.Arguments)))
	}
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind == Nullable {
			c.report(argument.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before closing", value.String()))
			if value.Element == nil {
				continue
			}
			value = *value.Element
		}
		goType, ok := c.goTypeForNativeStorage(value)
		if !ok {
			c.report(argument.GetSpan(), fmt.Sprintf("closeGoChannel requires a Go channel, got %s", value.String()))
			continue
		}
		if _, parameter := gotypes.Unalias(goType).(*gotypes.TypeParam); parameter {
			// Closing does not inspect elements: unlike send/receive or range,
			// its type set may contain channels with different element types.
			if !genericCollectionOperation("close", goType) {
				c.report(argument.GetSpan(), "closeGoChannel type parameter requires only send-capable channel types")
			}
			continue
		}
		channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
		if !ok {
			c.report(argument.GetSpan(), fmt.Sprintf("closeGoChannel requires a Go channel, got %s", value.String()))
			continue
		}
		if channel.Dir() == gotypes.RecvOnly {
			c.report(argument.GetSpan(), fmt.Sprintf("cannot close receive-only channel %s", value.String()))
		}
	}
	return builtins["void"]
}
