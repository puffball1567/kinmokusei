package sema

import (
	"fmt"
	goast "go/ast"
	"go/constant"
	gotoken "go/token"
	gotypes "go/types"
	"math/big"

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
	if op == "!" {
		return gotoken.NOT
	}
	return map[string]gotoken.Token{"+": gotoken.ADD, "-": gotoken.SUB, "*": gotoken.MUL, "/": gotoken.QUO, "%": gotoken.REM, "^": gotoken.XOR, "&": gotoken.AND, "|": gotoken.OR, "&^": gotoken.AND_NOT, "<<": gotoken.SHL, ">>": gotoken.SHR, "==": gotoken.EQL, "===": gotoken.EQL, "!=": gotoken.NEQ, "!==": gotoken.NEQ, "<": gotoken.LSS, "<=": gotoken.LEQ, ">": gotoken.GTR, ">=": gotoken.GEQ, "&&": gotoken.LAND, "||": gotoken.LOR}[op]
}

// Only literal scalar trees are rebuilt. Calls, variables, and imported
// objects are represented by checked types/values, never evaluated here.
func scalarLiteralTree(expr ast.Expression) goast.Expr {
	switch e := expr.(type) {
	case *ast.LiteralExpr:
		if e.Kind == ast.BooleanLiteral {
			return goast.NewIdent(e.Text)
		}
		kind := gotoken.INT
		if e.Kind == ast.FloatLiteral {
			kind = gotoken.FLOAT
		} else if e.Kind == ast.ImaginaryLiteral {
			kind = gotoken.IMAG
		} else if e.Kind == ast.StringLiteral {
			kind = gotoken.STRING
		} else if e.Kind != ast.IntegerLiteral {
			return nil
		}
		return &goast.BasicLit{Kind: kind, Value: e.Text}
	case *ast.UnaryExpr:
		if e.Operator != "+" && e.Operator != "-" && e.Operator != "^" && e.Operator != "!" {
			return nil
		}
		if operand := scalarLiteralTree(e.Operand); operand != nil {
			return &goast.UnaryExpr{Op: numericOperator(e.Operator), X: operand}
		}
	case *ast.BinaryExpr:
		left, right := scalarLiteralTree(e.Left), scalarLiteralTree(e.Right)
		if left != nil && right != nil && numericOperator(e.Operator) != gotoken.ILLEGAL {
			return &goast.BinaryExpr{X: left, Op: numericOperator(e.Operator), Y: right}
		}
	case *ast.CallExpr:
		name, ok := e.Callee.(*ast.IdentifierExpr)
		if !ok || !e.Conversion || len(e.Arguments) != 1 || e.Expanded {
			return nil
		}
		t, ok := LookupType(name.Name)
		if !ok || !isScalarConstantType(t) {
			return nil
		}
		if arg := scalarLiteralTree(e.Arguments[0]); arg != nil {
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

func (c *Checker) scalarConstant(expr ast.Expression) (gotypes.TypeAndValue, bool) {
	if identifier, ok := expr.(*ast.IdentifierExpr); ok {
		if value := c.namedGoConstant(identifier); value != nil {
			return gotypes.TypeAndValue{Type: value.Type(), Value: value.Val()}, true
		}
	}
	if value, ok := c.constantValues[expr]; ok && value.Value != nil {
		return value, true
	}
	if member, ok := expr.(*ast.MemberExpr); ok && member.Constant {
		if id, ok := member.Object.(*ast.IdentifierExpr); ok {
			if enumeration := c.enums[id.Name]; enumeration != nil {
				if value := enumeration.members[member.Name]; value != nil && value.ResolvedValue != "" {
					if native := c.nativeTypes[id.Name]; native != nil {
						typeInfo := c.resolveNativeType(native)
						if goType, ok := goTypeOf(typeInfo); ok {
							if parsed, ok := new(big.Int).SetString(value.ResolvedValue, 10); ok {
								return gotypes.TypeAndValue{Type: goType, Value: constant.Make(parsed)}, true
							}
						}
					}
				}
			}
			if imported := c.lookupGoPackage(id.Span.Path, id.Name); imported != nil {
				if object, ok := imported.packageInfo.Scope().Lookup(member.Name).(*gotypes.Const); ok {
					return gotypes.TypeAndValue{Type: object.Type(), Value: object.Val()}, true
				}
			}
		}
	}
	if tree := scalarLiteralTree(expr); tree != nil {
		value, err := c.evalNumericGo(nil, tree)
		return value, err == nil && value.Value != nil
	}
	return gotypes.TypeAndValue{}, false
}

// Validate when an untyped numeric expression is materialized into storage or
// an expected parameter/result type, without prematurely rounding its children.
func (c *Checker) checkNumericMaterialization(expr ast.Expression, target Type) bool {
	info, known := c.constantValues[expr]
	if !known {
		info, known = c.scalarConstant(expr)
		if gt, ok := goTypeOf(target); known && ok {
			known = gotypes.AssignableTo(info.Type, gt)
		} else {
			known = false
		}
	}
	if known && info.Value != nil && (target.IsNumeric() || underlyingGoInterface(target.GoType) != nil) {
		if gt, ok := goTypeOf(target); ok {
			if err := c.checkNumericConstantAssignment(info, gt); err != nil {
				c.report(expr.GetSpan(), err.Error())
				return false
			}
		}
	}
	if (target.IsNumeric() || underlyingGoInterface(target.GoType) != nil) && c.hasDeferredShift(expr) {
		if gt, ok := goTypeOf(target); ok {
			if err := c.checkShiftAssignment(expr, gt); err != nil {
				c.report(expr.GetSpan(), err.Error())
				return false
			}
		}
	}
	return true
}

// Only propagate values from scalar initializers that emit Go constants.
// Identifiers carry the checked binding fact, not merely source immutability.
func initializerEmitsConstant(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.IdentifierExpr:
		return e.GoConstant || e.GoMember != nil && e.GoMember.Constant
	case *ast.LiteralExpr:
		return e.Kind == ast.IntegerLiteral || e.Kind == ast.FloatLiteral || e.Kind == ast.ImaginaryLiteral || e.Kind == ast.StringLiteral || e.Kind == ast.BooleanLiteral
	case *ast.UnaryExpr:
		return initializerEmitsConstant(e.Operand)
	case *ast.BinaryExpr:
		return initializerEmitsConstant(e.Left) && initializerEmitsConstant(e.Right)
	case *ast.CallExpr:
		return e.GoConstant || scalarLiteralTree(e) != nil
	case *ast.MemberExpr:
		return e.Constant
	}
	return false
}

func (c *Checker) convertNumericConstant(info gotypes.TypeAndValue, target gotypes.Type) (gotypes.TypeAndValue, error) {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/constant", "constant")
	pkg.Scope().Insert(gotypes.NewConst(0, pkg, "value", info.Type, info.Value))
	pkg.Scope().Insert(gotypes.NewTypeName(0, pkg, "Target", target))
	pkg.MarkComplete()
	return c.evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent("Target"), Args: []goast.Expr{goast.NewIdent("value")}})
}

func (c *Checker) numericOperand(pkg *gotypes.Package, name string, expr ast.Expression, actual Type) (goast.Expr, bool) {
	gt, ok := goTypeOf(actual)
	if !ok || actual.Kind == Nullable || actual.Kind == Invalid {
		return nil, false
	}
	var value constant.Value
	if info, known := c.scalarConstant(expr); known {
		gt, value = info.Type, info.Value
	}
	if id, ok := expr.(*ast.IdentifierExpr); ok {
		if object := c.namedGoConstant(id); object != nil {
			gt, value = object.Type(), object.Val()
		}
		if symbol, found := c.lookupSymbol(id.Name, id.Span); found && symbol.declaration != nil && symbol.declaration.GoConstant {
			if info, known := c.scalarConstant(symbol.declaration.Value); known {
				value = info.Value
				if !symbol.declaration.Type.IsSpecified() {
					gt = info.Type
				} else if rounded, err := c.convertNumericConstant(info, gt); err == nil {
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
		if actual.Kind == UntypedInt || isUntypedGoNumeric(actual) {
			if tree := c.contextualNumericExpression(pkg, name, expr); tree != nil {
				return tree, true
			}
		}
		// Runtime bindings remain typed variables, even if immutable.
		gt = gotypes.Default(gt)
		pkg.Scope().Insert(gotypes.NewVar(0, pkg, name, gt))
	}
	return goast.NewIdent(name), true
}

func (c *Checker) finishNumeric(expr ast.Expression, pkg *gotypes.Package, node goast.Expr) Type {
	pkg.MarkComplete()
	info, err := c.evalNumericGo(pkg, node)
	if err != nil {
		c.report(expr.GetSpan(), err.Error())
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if c.constantValues == nil {
		c.constantValues = map[ast.Expression]gotypes.TypeAndValue{}
	}
	c.constantValues[expr] = info
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
	return preserveUntypedScalar(result, info.Type)
}

func (c *Checker) checkComplexBuiltin(expr *ast.CallExpr, name string) Type {
	count := 1
	expr.Builtin = ast.RealCall
	if name == "complex" {
		count, expr.Builtin = 2, ast.ComplexCall
	} else if name == "imag" {
		expr.Builtin = ast.ImagCall
	}
	values, origins := c.checkNumericCallInputs(expr)
	shape := *expr
	shape.Arguments = origins
	c.checkBuiltinCallShape(&shape, name, count, count, 0)
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	args := make([]goast.Expr, len(values))
	parameterTypes := make([]string, len(args))
	valid := len(values) == count && len(expr.TypeArguments) == 0 && !expr.Expanded
	for i, argument := range origins {
		actual := values[i]
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
	if result.Kind != Invalid {
		c.recordBuiltinMultipleResult(expr, result)
	}
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

func (c *Checker) checkGoBinary(expr *ast.BinaryExpr, left, right Type) Type {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	x, xok := c.numericOperand(pkg, "left", expr.Left, left)
	y, yok := c.numericOperand(pkg, "right", expr.Right, right)
	if !xok || !yok {
		c.report(expr.Span, "operator "+expr.Operator+" requires compatible numeric operands")
		return Type{Kind: Invalid}
	}
	return c.finishNumeric(expr, pkg, &goast.BinaryExpr{X: x, Op: numericOperator(expr.Operator), Y: y})
}

func (c *Checker) checkGoUnary(expr *ast.UnaryExpr, operand Type) Type {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	x, ok := c.numericOperand(pkg, "value", expr.Operand, operand)
	if !ok {
		return Type{Kind: Invalid}
	}
	return c.finishNumeric(expr, pkg, &goast.UnaryExpr{Op: numericOperator(expr.Operator), X: x})
}

func (c *Checker) checkNumericConstantAssignment(info gotypes.TypeAndValue, target gotypes.Type) error {
	pkg := gotypes.NewPackage("kinmokusei.synthetic/numeric", "numeric")
	pkg.Scope().Insert(gotypes.NewConst(0, pkg, "value", info.Type, info.Value))
	if _, constrained := gotypes.Unalias(target).(*gotypes.TypeParam); constrained {
		// A foreign type parameter embedded directly into a synthetic function
		// signature does not make go/types validate the constant against every
		// term in its type set. Naming the target and checking the corresponding
		// conversion exercises the same representability rule as generated Go.
		pkg.Scope().Insert(gotypes.NewTypeName(0, pkg, "Target", target))
		pkg.MarkComplete()
		_, err := c.evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent("Target"), Args: []goast.Expr{goast.NewIdent("value")}})
		return err
	}
	signature := gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(gotypes.NewVar(0, pkg, "", target)), nil, false)
	pkg.Scope().Insert(gotypes.NewFunc(0, pkg, "accept", signature))
	pkg.MarkComplete()
	_, err := c.evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent("accept"), Args: []goast.Expr{goast.NewIdent("value")}})
	return err
}
