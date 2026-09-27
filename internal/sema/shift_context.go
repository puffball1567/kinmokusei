package sema

import (
	goast "go/ast"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// A runtime shift of an untyped constant acquires its type from its context.
// Preserve that expression until a destination or typed peer is available:
// modeling 300<<n as a byte variable would hide the overflowing left operand.
func (c *Checker) hasDeferredShift(expression ast.Expression) bool {
	if _, constant := c.scalarConstant(expression); constant {
		return false
	}
	switch expr := expression.(type) {
	case *ast.UnaryExpr:
		return c.hasDeferredShift(expr.Operand)
	case *ast.BinaryExpr:
		if expr.Operator == "<<" || expr.Operator == ">>" {
			if value, known := c.scalarConstant(expr.Left); known {
				basic, ok := value.Type.(*gotypes.Basic)
				return ok && basic.Info()&gotypes.IsUntyped != 0
			}
		}
		return c.hasDeferredShift(expr.Left) || c.hasDeferredShift(expr.Right)
	}
	return false
}

// Counts were already checked by the ordinary expression checker. Rebuilding
// only constants and operators never evaluates or rechecks user expressions.
func (c *Checker) checkShiftAssignment(expr ast.Expression, target gotypes.Type) error {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/shift", "shift")
	tree := c.contextualNumericExpression(pkg, "value", expr)
	if tree == nil {
		return nil // A typed runtime peer has already fixed the expression's type.
	}
	signature := gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(gotypes.NewVar(0, pkg, "", target)), nil, false)
	pkg.Scope().Insert(gotypes.NewFunc(0, pkg, "accept", signature))
	pkg.MarkComplete()
	_, err := c.evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent("accept"), Args: []goast.Expr{tree}})
	return err
}

func (c *Checker) contextualNumericExpression(pkg *gotypes.Package, name string, expression ast.Expression) goast.Expr {
	if value, known := c.scalarConstant(expression); known {
		pkg.Scope().Insert(gotypes.NewConst(0, pkg, name, value.Type, value.Value))
		return goast.NewIdent(name)
	}
	switch expr := expression.(type) {
	case *ast.UnaryExpr:
		if operand := c.contextualNumericExpression(pkg, name+"_value", expr.Operand); operand != nil {
			return &goast.UnaryExpr{Op: numericOperator(expr.Operator), X: operand}
		}
	case *ast.BinaryExpr:
		left := c.contextualNumericExpression(pkg, name+"_left", expr.Left)
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
			right = c.contextualNumericExpression(pkg, name+"_right", expr.Right)
		}
		if right != nil {
			return &goast.BinaryExpr{X: left, Op: numericOperator(expr.Operator), Y: right}
		}
	}
	return nil
}
