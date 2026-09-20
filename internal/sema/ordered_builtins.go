package sema

import (
	"fmt"
	goast "go/ast"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Use Go's ordered-operand rules, preserving exact constants until their
// common type is known. Runtime operands stay variables, not folded values.
func (c *Checker) checkOrderedBuiltin(expr *ast.CallExpr, name string) Type {
	if name == "min" {
		expr.Builtin = ast.MinCall
	} else {
		expr.Builtin = ast.MaxCall
	}
	valid := true
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, fmt.Sprintf("%s does not accept type arguments", name))
		valid = false
	}
	if expr.Expanded {
		c.report(expr.Span, fmt.Sprintf("%s does not accept spread arguments", name))
		valid = false
	}
	if len(expr.Arguments) == 0 {
		c.report(expr.Span, fmt.Sprintf("%s expects at least 1 argument, got 0", name))
		valid = false
	}
	pkg := gotypes.NewPackage("kinmokusei.synthetic/ordered", "ordered")
	arguments := make([]goast.Expr, len(expr.Arguments))
	for index, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind == Invalid {
			valid = false
			continue
		}
		if !value.IsOrdered() {
			c.report(argument.GetSpan(), fmt.Sprintf("%s requires ordered operands, got %s", name, value.String()))
			valid = false
			continue
		}
		operandName := fmt.Sprintf("arg%d", index+1)
		var ok bool
		arguments[index], ok = c.numericOperand(pkg, operandName, argument, value)
		if !ok {
			c.report(argument.GetSpan(), fmt.Sprintf("%s cannot use operand of type %s", name, value.String()))
			valid = false
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return c.finishNumeric(expr, pkg, &goast.CallExpr{Fun: goast.NewIdent(name), Args: arguments})
}
