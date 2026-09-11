package sema

import (
	"fmt"
	goast "go/ast"
	"go/constant"
	gotoken "go/token"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func isComplexType(t Type) bool {
	gt, ok := goTypeOf(t)
	if !ok {
		return false
	}
	if basic, ok := gotypes.Unalias(gt).Underlying().(*gotypes.Basic); ok {
		return basic.Info()&gotypes.IsComplex != 0
	}
	return goTypeSetSupports(gt, gotypes.IsComplex)
}

func isUntypedGoNumeric(t Type) bool {
	basic, ok := t.GoType.(*gotypes.Basic)
	return ok && basic.Info()&gotypes.IsUntyped != 0 && basic.Info()&gotypes.IsNumeric != 0
}

func numericOperator(op string) gotoken.Token {
	return map[string]gotoken.Token{"+": gotoken.ADD, "-": gotoken.SUB, "*": gotoken.MUL, "/": gotoken.QUO, "%": gotoken.REM, "^": gotoken.XOR, "&": gotoken.AND, "|": gotoken.OR, "&^": gotoken.AND_NOT, "<<": gotoken.SHL, ">>": gotoken.SHR, "==": gotoken.EQL, "===": gotoken.EQL, "!=": gotoken.NEQ, "!==": gotoken.NEQ, "<": gotoken.LSS, "<=": gotoken.LEQ, ">": gotoken.GTR, ">=": gotoken.GEQ, "&&": gotoken.LAND, "||": gotoken.LOR}[op]
}

// Only literal numeric trees are rebuilt. Calls, variables, and imported
// objects are represented by checked types/values, never evaluated here.
func numericLiteralTree(expr ast.Expression) goast.Expr {
	switch e := expr.(type) {
	case *ast.LiteralExpr:
		kind := gotoken.INT
		if e.Kind == ast.FloatLiteral {
			kind = gotoken.FLOAT
		} else if e.Kind == ast.ImaginaryLiteral {
			kind = gotoken.IMAG
		} else if e.Kind != ast.IntegerLiteral {
			return nil
		}
		return &goast.BasicLit{Kind: kind, Value: e.Text}
	case *ast.UnaryExpr:
		if e.Operator != "+" && e.Operator != "-" && e.Operator != "^" {
			return nil
		}
		if operand := numericLiteralTree(e.Operand); operand != nil {
			return &goast.UnaryExpr{Op: numericOperator(e.Operator), X: operand}
		}
	case *ast.BinaryExpr:
		left, right := numericLiteralTree(e.Left), numericLiteralTree(e.Right)
		if left != nil && right != nil && numericOperator(e.Operator) != gotoken.ILLEGAL {
			return &goast.BinaryExpr{X: left, Op: numericOperator(e.Operator), Y: right}
		}
	case *ast.CallExpr:
		name, ok := e.Callee.(*ast.IdentifierExpr)
		if !ok || !e.Conversion || len(e.Arguments) != 1 || e.Expanded {
			return nil
		}
		t, ok := LookupType(name.Name)
		if !ok || !t.IsNumeric() {
			return nil
		}
		if arg := numericLiteralTree(e.Arguments[0]); arg != nil {
			goType, ok := goTypeOf(t)
			if !ok {
				return nil
			}
			return &goast.CallExpr{Fun: goast.NewIdent(goType.String()), Args: []goast.Expr{arg}}
		}
	}
	return nil
}

func evalNumericGo(pkg *gotypes.Package, expr goast.Expr) (gotypes.TypeAndValue, error) {
	info := &gotypes.Info{Types: map[goast.Expr]gotypes.TypeAndValue{}}
	err := gotypes.CheckExpr(gotoken.NewFileSet(), pkg, gotoken.NoPos, expr, info)
	return info.Types[expr], err
}

func (c *Checker) numericConstant(expr ast.Expression) (gotypes.TypeAndValue, bool) {
	if identifier, ok := expr.(*ast.IdentifierExpr); ok {
		if value := c.namedGoConstant(identifier); value != nil {
			return gotypes.TypeAndValue{Type: value.Type(), Value: value.Val()}, true
		}
	}
	if value, ok := c.numericValues[expr]; ok && value.Value != nil {
		return value, true
	}
	if member, ok := expr.(*ast.MemberExpr); ok && member.Constant {
		if id, ok := member.Object.(*ast.IdentifierExpr); ok {
			if imported := c.lookupGoPackage(id.Span.Path, id.Name); imported != nil {
				if object, ok := imported.packageInfo.Scope().Lookup(member.Name).(*gotypes.Const); ok {
					return gotypes.TypeAndValue{Type: object.Type(), Value: object.Val()}, true
				}
			}
		}
	}
	if tree := numericLiteralTree(expr); tree != nil {
		value, err := evalNumericGo(nil, tree)
		return value, err == nil && value.Value != nil
	}
	return gotypes.TypeAndValue{}, false
}

// Validate when an untyped numeric expression is materialized into storage or
// an expected parameter/result type, without prematurely rounding its children.
func (c *Checker) checkNumericMaterialization(expr ast.Expression, target Type) bool {
	if info, ok := c.numericValues[expr]; ok && info.Value != nil && target.IsNumeric() {
		if gt, ok := goTypeOf(target); ok {
			if err := checkNumericConstantAssignment(info, gt); err != nil {
				c.report(expr.GetSpan(), err.Error())
				return false
			}
		}
	}
	return true
}

// A source const can lower to a Go variable (for example an initializer that
// refers to another local). Only propagate values from actual Go constants.
func numericInitializerEmitsConstant(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.IdentifierExpr:
		return e.GoMember != nil && e.GoMember.Constant
	case *ast.LiteralExpr:
		return e.Kind == ast.IntegerLiteral || e.Kind == ast.FloatLiteral || e.Kind == ast.ImaginaryLiteral
	case *ast.UnaryExpr:
		return numericInitializerEmitsConstant(e.Operand)
	case *ast.BinaryExpr:
		return numericInitializerEmitsConstant(e.Left) && numericInitializerEmitsConstant(e.Right)
	case *ast.CallExpr:
		return e.GoConstant || numericLiteralTree(e) != nil
	case *ast.MemberExpr:
		return e.Constant
	}
	return false
}

func convertNumericConstant(info gotypes.TypeAndValue, target gotypes.Type) (gotypes.TypeAndValue, error) {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/constant", "constant")
	pkg.Scope().Insert(gotypes.NewConst(0, pkg, "value", info.Type, info.Value))
	pkg.Scope().Insert(gotypes.NewTypeName(0, pkg, "Target", target))
	pkg.MarkComplete()
	return evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent("Target"), Args: []goast.Expr{goast.NewIdent("value")}})
}

func (c *Checker) numericOperand(pkg *gotypes.Package, name string, expr ast.Expression, actual Type) (goast.Expr, bool) {
	gt, ok := goTypeOf(actual)
	if !ok || actual.Kind == Nullable || actual.Kind == Invalid {
		return nil, false
	}
	var value constant.Value
	if info, known := c.numericConstant(expr); known {
		gt, value = info.Type, info.Value
	}
	if id, ok := expr.(*ast.IdentifierExpr); ok {
		if object := c.namedGoConstant(id); object != nil {
			gt, value = object.Type(), object.Val()
		}
		if symbol, found := c.lookupSymbol(id.Name, id.Span); found && symbol.constant && symbol.declaration != nil && numericInitializerEmitsConstant(symbol.declaration.Value) {
			if info, known := c.numericConstant(symbol.declaration.Value); known {
				value = info.Value
				if !symbol.declaration.Type.IsSpecified() {
					gt = info.Type
				} else if rounded, err := convertNumericConstant(info, gt); err == nil {
					value = rounded.Value
				} else {
					return nil, false
				}
			}
		}
	}
	if member, ok := expr.(*ast.MemberExpr); ok && member.Constant {
		if id, ok := member.Object.(*ast.IdentifierExpr); ok {
			if imported := c.lookupGoPackage(id.Span.Path, id.Name); imported != nil {
				if object, ok := imported.packageInfo.Scope().Lookup(member.Name).(*gotypes.Const); ok {
					gt, value = object.Type(), object.Val()
				}
			}
		}
	}
	if value != nil {
		pkg.Scope().Insert(gotypes.NewConst(0, pkg, name, gt, value))
	} else {
		// A nonconstant untyped shift cannot acquire a floating/complex type.
		// Replacing its AST with an untyped synthetic variable would lose Go's
		// deferred shift check and incorrectly accept complex(1 << n, 0).
		gt = gotypes.Default(gt)
		pkg.Scope().Insert(gotypes.NewVar(0, pkg, name, gt))
	}
	return goast.NewIdent(name), true
}

func (c *Checker) finishNumeric(expr ast.Expression, pkg *gotypes.Package, node goast.Expr) Type {
	pkg.MarkComplete()
	info, err := evalNumericGo(pkg, node)
	if err != nil {
		c.report(expr.GetSpan(), err.Error())
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if c.numericValues == nil {
		c.numericValues = map[ast.Expression]gotypes.TypeAndValue{}
	}
	c.numericValues[expr] = info
	if call, ok := expr.(*ast.CallExpr); ok {
		call.GoConstant = info.Value != nil
	}
	if basic, ok := info.Type.(*gotypes.Basic); ok && basic.Info()&gotypes.IsUntyped != 0 && basic.Info()&gotypes.IsNumeric != 0 {
		return Type{Kind: GoBasic, Name: basic.Name(), GoType: basic}
	}
	result, err := kinmokuseiTypeFromGo(info.Type)
	if err != nil {
		c.report(expr.GetSpan(), err.Error())
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return result
}

func (c *Checker) checkComplexBuiltin(expr *ast.CallExpr, name string) Type {
	count := 1
	expr.Builtin = ast.RealCall
	if name == "complex" {
		count, expr.Builtin = 2, ast.ComplexCall
	} else if name == "imag" {
		expr.Builtin = ast.ImagCall
	}
	c.checkBuiltinCallShape(expr, name, count, count, 0)
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	args := make([]goast.Expr, len(expr.Arguments))
	parameterTypes := make([]string, len(args))
	valid := len(expr.Arguments) == count && len(expr.TypeArguments) == 0 && !expr.Expanded
	for i, argument := range expr.Arguments {
		actual := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		parameterTypes[i] = actual.String()
		var ok bool
		args[i], ok = c.numericOperand(pkg, fmt.Sprintf("arg%d", i), argument, actual)
		if !ok {
			if actual.Kind != Invalid {
				c.report(argument.GetSpan(), name+" requires numeric operands")
			}
			valid = false
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	result := c.finishNumeric(expr, pkg, &goast.CallExpr{Fun: goast.NewIdent(name), Args: args})
	expr.Signature = &ast.CallableSignature{Result: result.String(), ParameterTypes: parameterTypes}
	for i := range args {
		expr.Signature.ParameterNames = append(expr.Signature.ParameterNames, fmt.Sprintf("arg%d", i+1))
	}
	return result
}

func (c *Checker) checkComplexConversion(expr *ast.CallExpr, target, actual Type) Type {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	gt, ok := goTypeOf(target)
	if !ok {
		c.report(expr.Span, "conversion target has no Go representation")
		return Type{Kind: Invalid}
	}
	pkg.Scope().Insert(gotypes.NewTypeName(0, pkg, "Target", gt))
	arg, ok := c.numericOperand(pkg, "value", expr.Arguments[0], actual)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("cannot convert %s to %s", actual.String(), target.String()))
		return Type{Kind: Invalid}
	}
	result := c.finishNumeric(expr, pkg, &goast.CallExpr{Fun: goast.NewIdent("Target"), Args: []goast.Expr{arg}})
	if result.Kind == Invalid {
		return result
	}
	return target
}

func (c *Checker) checkComplexBinary(expr *ast.BinaryExpr, left, right Type) Type {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	x, xok := c.numericOperand(pkg, "left", expr.Left, left)
	y, yok := c.numericOperand(pkg, "right", expr.Right, right)
	if !xok || !yok {
		c.report(expr.Span, "operator "+expr.Operator+" requires compatible numeric operands")
		return Type{Kind: Invalid}
	}
	return c.finishNumeric(expr, pkg, &goast.BinaryExpr{X: x, Op: numericOperator(expr.Operator), Y: y})
}

func (c *Checker) checkComplexUnary(expr *ast.UnaryExpr, operand Type) Type {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	x, ok := c.numericOperand(pkg, "value", expr.Operand, operand)
	if !ok {
		return Type{Kind: Invalid}
	}
	return c.finishNumeric(expr, pkg, &goast.UnaryExpr{Op: numericOperator(expr.Operator), X: x})
}

func checkNumericConstantAssignment(info gotypes.TypeAndValue, target gotypes.Type) error {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	pkg.Scope().Insert(gotypes.NewConst(0, pkg, "value", info.Type, info.Value))
	signature := gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(gotypes.NewVar(0, pkg, "", target)), nil, false)
	pkg.Scope().Insert(gotypes.NewFunc(0, pkg, "accept", signature))
	pkg.MarkComplete()
	_, err := evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent("accept"), Args: []goast.Expr{goast.NewIdent("value")}})
	return err
}
