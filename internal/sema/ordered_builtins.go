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
		var ok bool
		operandName := fmt.Sprintf("arg%d", index+1)
		if value.Kind == UntypedInt || isUntypedGoNumeric(value) {
			arguments[index] = c.orderedUntypedExpression(pkg, operandName, argument)
		}
		if arguments[index] != nil {
			ok = true
		} else {
			arguments[index], ok = c.numericOperand(pkg, operandName, argument, value)
		}
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

// Preserve untyped nonconstant shifts until the other min/max operands provide
// their context. Replacing 1<<n with an int variable would wrongly reject a
// uint8 peer, while replacing it with a uint8 variable would miss 300<<n's
// overflowing left operand. Counts were already checked as integers; model an
// unknown count as a variable without inspecting or executing its expression.
func (c *Checker) orderedUntypedExpression(pkg *gotypes.Package, name string, expression ast.Expression) goast.Expr {
	if value, known := c.scalarConstant(expression); known {
		pkg.Scope().Insert(gotypes.NewConst(0, pkg, name, value.Type, value.Value))
		return goast.NewIdent(name)
	}
	switch expr := expression.(type) {
	case *ast.UnaryExpr:
		if operand := c.orderedUntypedExpression(pkg, name+"_value", expr.Operand); operand != nil {
			return &goast.UnaryExpr{Op: numericOperator(expr.Operator), X: operand}
		}
	case *ast.BinaryExpr:
		left := c.orderedUntypedExpression(pkg, name+"_left", expr.Left)
		if left == nil {
			return nil
		}
		var right goast.Expr
		if expr.Operator == "<<" || expr.Operator == ">>" {
			if value, known := c.scalarConstant(expr.Right); known {
				pkg.Scope().Insert(gotypes.NewConst(0, pkg, name+"_count", value.Type, value.Value))
			} else {
				pkg.Scope().Insert(gotypes.NewVar(0, pkg, name+"_count", gotypes.Typ[gotypes.Uint]))
			}
			right = goast.NewIdent(name + "_count")
		} else {
			right = c.orderedUntypedExpression(pkg, name+"_right", expr.Right)
		}
		if right != nil {
			return &goast.BinaryExpr{X: left, Op: numericOperator(expr.Operator), Y: right}
		}
	}
	return nil
}
