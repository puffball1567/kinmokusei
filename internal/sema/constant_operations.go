package sema

import (
	goast "go/ast"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Retain checked integer constant operations as well as float/complex ones.
// Evaluate from the children's resolved values in their original lexical
// context, never by looking up initializer names again at a later use site.
func (c *Checker) checkConstantOperation(expr ast.Expression, result Type) Type {
	if !result.IsNumeric() {
		return result
	}
	if _, checked := c.numericValues[expr]; checked {
		return result
	}
	var pkg *gotypes.Package
	operand := func(name string, expression ast.Expression) (goast.Expr, bool) {
		info, known := c.numericConstant(expression)
		if !known {
			return nil, false
		}
		if pkg == nil {
			pkg = gotypes.NewPackage("kinmokusei.synthetic/constant", "constant")
		}
		pkg.Scope().Insert(gotypes.NewConst(0, pkg, name, info.Type, info.Value))
		return goast.NewIdent(name), true
	}
	var node goast.Expr
	switch expr := expr.(type) {
	case *ast.BinaryExpr:
		left, leftOK := operand("left", expr.Left)
		right, rightOK := operand("right", expr.Right)
		if !leftOK || !rightOK {
			return result
		}
		node = &goast.BinaryExpr{X: left, Op: numericOperator(expr.Operator), Y: right}
	case *ast.UnaryExpr:
		if expr.Operator != "+" && expr.Operator != "-" && expr.Operator != "^" {
			return result
		}
		value, known := operand("value", expr.Operand)
		if !known {
			return result
		}
		node = &goast.UnaryExpr{Op: numericOperator(expr.Operator), X: value}
	case *ast.CallExpr:
		if !expr.Conversion || expr.Expanded || len(expr.Arguments) != 1 {
			return result
		}
		target, ok := goTypeOf(result)
		if !ok {
			return result
		}
		// Conversion to a type parameter is a runtime value even for literal input.
		if _, basic := target.Underlying().(*gotypes.Basic); !basic {
			return result
		}
		value, known := operand("value", expr.Arguments[0])
		if !known {
			return result
		}
		pkg.Scope().Insert(gotypes.NewTypeName(0, pkg, "Target", target))
		node = &goast.CallExpr{Fun: goast.NewIdent("Target"), Args: []goast.Expr{value}}
	default:
		return result
	}
	if checked := c.finishNumeric(expr, pkg, node); checked.Kind == Invalid {
		return checked
	}
	return result
}
