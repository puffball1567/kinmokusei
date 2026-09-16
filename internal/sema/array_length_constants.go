package sema

import (
	goast "go/ast"
	"go/constant"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Delegate type-set validation to Go: every term must support the operation,
// but unlike range, len/cap do not require one common collection shape.
func typeParameterAcceptsLenOrCap(value gotypes.Type, allowLenOnly bool) bool {
	name := "cap"
	if allowLenOnly {
		name = "len"
	}
	pkg := gotypes.NewPackage("kinmokusei.synthetic/length", "length")
	pkg.Scope().Insert(gotypes.NewVar(0, pkg, "value", value))
	pkg.MarkComplete()
	_, err := evalNumericGo(pkg, &goast.CallExpr{Fun: goast.NewIdent(name), Args: []goast.Expr{goast.NewIdent("value")}})
	return err == nil
}

// len/cap of an array (or array pointer) is a typed int constant when Go
// does not evaluate the operand. An immutable binding is not required.
func (c *Checker) checkArrayLengthConstant(call *ast.CallExpr, value Type) {
	if len(call.Arguments) != 1 || len(call.TypeArguments) != 0 || call.Expanded || value.Kind == Invalid || value.Kind == Nullable || value.Kind == TypeParameter {
		return
	}
	length := int64(-1)
	if value.Kind == FixedArray {
		length = value.Length
	} else if target, ok := goTypeOf(value); ok {
		underlying := gotypes.Unalias(target).Underlying()
		if pointer, ok := underlying.(*gotypes.Pointer); ok {
			underlying = gotypes.Unalias(pointer.Elem()).Underlying()
		}
		if array, ok := underlying.(*gotypes.Array); ok {
			length = array.Len()
		}
	}
	if length < 0 || !arrayLengthOperandUnevaluated(call.Arguments[0]) {
		return
	}
	if c.constantValues == nil {
		c.constantValues = map[ast.Expression]gotypes.TypeAndValue{}
	}
	c.constantValues[call] = gotypes.TypeAndValue{Type: gotypes.Typ[gotypes.Int], Value: constant.MakeInt64(length)}
	call.GoConstant = true
}

// Follow expression syntax, not runtime purity: even a side-effect-free call
// prevents constant len/cap. Conversions are not calls. Function literal bodies
// are not evaluated; checked constant calls may themselves hide unevaluated
// operands. Source operations lowered to runtime helpers stay nonconstant.
func arrayLengthOperandUnevaluated(expression ast.Expression) bool {
	switch expr := expression.(type) {
	case nil, *ast.IdentifierExpr, *ast.LiteralExpr, *ast.ArrowExpr:
		return true
	case *ast.UnaryExpr:
		return expr.Operator != "<-" && arrayLengthOperandUnevaluated(expr.Operand)
	case *ast.BinaryExpr:
		return arrayLengthOperandUnevaluated(expr.Left) && arrayLengthOperandUnevaluated(expr.Right)
	case *ast.MemberExpr:
		return arrayLengthOperandUnevaluated(expr.Object)
	case *ast.IndexExpr:
		return arrayLengthOperandUnevaluated(expr.Object) && arrayLengthOperandUnevaluated(expr.Index)
	case *ast.SliceExpr:
		return arrayLengthOperandUnevaluated(expr.Object) && arrayLengthOperandUnevaluated(expr.Low) && arrayLengthOperandUnevaluated(expr.High) && arrayLengthOperandUnevaluated(expr.Max)
	case *ast.GoTypeAssertionExpr:
		return !expr.ClassDowncast && arrayLengthOperandUnevaluated(expr.Value)
	case *ast.CallExpr:
		if expr.GoConstant {
			return true
		}
		if !expr.Conversion && expr.Builtin != ast.CopyArrayCall && expr.Builtin != ast.ViewArrayCall {
			return false
		}
		for _, argument := range expr.Arguments {
			if !arrayLengthOperandUnevaluated(argument) {
				return false
			}
		}
		return true
	case *ast.ArrayLiteralExpr:
		for _, element := range expr.Elements {
			if !arrayLengthOperandUnevaluated(element) {
				return false
			}
		}
		return true
	case *ast.ObjectLiteralExpr:
		return arrayLengthFieldsUnevaluated(expr.Fields)
	case *ast.GoCompositeLiteralExpr:
		return arrayLengthFieldsUnevaluated(expr.Fields)
	default:
		return false
	}
}

func arrayLengthFieldsUnevaluated(fields []ast.ObjectField) bool {
	for _, field := range fields {
		if !arrayLengthOperandUnevaluated(field.Value) {
			return false
		}
	}
	return true
}
