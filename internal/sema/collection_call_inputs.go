package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Keep the source argument intact: Go performs the expansion and evaluates the
// producer once. All expanded slots refer back to that producer for diagnostics.
func (c *Checker) checkCollectionCallInputs(expr *ast.CallExpr, name string, count int) ([]Type, []source.Span) {
	var checked *Type
	if len(expr.Arguments) == 1 && !expr.Expanded {
		if _, call := expr.Arguments[0].(*ast.CallExpr); call {
			value := c.checkExpression(expr.Arguments[0])
			if value.Kind == MultiValue {
				if len(expr.TypeArguments) != 0 {
					c.report(expr.Span, name+" does not accept type arguments")
				}
				if len(value.Results) != count {
					c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d results", name, count, len(value.Results)))
				}
				spans := make([]source.Span, len(value.Results))
				for i := range spans {
					spans[i] = expr.Arguments[0].GetSpan()
				}
				return value.Results, spans
			}
			checked = &value
		}
	}
	c.checkBuiltinCallShape(expr, name, count, count, 0)
	values := make([]Type, len(expr.Arguments))
	spans := make([]source.Span, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		spans[i] = argument.GetSpan()
		if checked != nil {
			values[i] = c.singleValue(*checked, spans[i])
		} else {
			values[i] = c.singleValue(c.checkExpression(argument), spans[i])
		}
	}
	return values, spans
}
