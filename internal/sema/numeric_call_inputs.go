package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Retain original expressions for constant operands. Expanded call results are
// runtime values; their shared producer is kept only for source diagnostics.
func (c *Checker) checkNumericCallInputs(call *ast.CallExpr) ([]Type, []ast.Expression) {
	values := make([]Type, len(call.Arguments))
	for i, argument := range call.Arguments {
		value := c.checkExpression(argument)
		if _, producer := argument.(*ast.CallExpr); producer && len(call.Arguments) == 1 && !call.Expanded && value.Kind == MultiValue {
			call.MultipleArgumentCount = len(value.Results)
			origins := make([]ast.Expression, len(value.Results))
			for j := range origins {
				origins[j] = argument
			}
			return value.Results, origins
		}
		values[i] = c.singleValue(value, argument.GetSpan())
	}
	return values, call.Arguments
}
