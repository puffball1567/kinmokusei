package sema

import (
	"go/constant"
	gotypes "go/types"
	"math/big"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Type declarations precede ordinary initializer checking. Lengths need both
// scalar constants and annotated array types, without moving runtime evaluation.
func (c *Checker) predeclareArrayLengthBindings(program *ast.Program) {
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.VariableDecl)
		if !ok || decl.FunctionBinding {
			continue
		}
		if _, exists := c.globals[decl.Name]; !exists {
			c.globals[decl.Name] = valueSymbol{typeInfo: Type{Kind: Invalid, Name: "<inferred>"}, constant: decl.Constant, declarationSpan: decl.NameSpan, declaration: decl}
		}
	}
}

func (c *Checker) markGoImportUsed(declaration *ast.ImportDecl) {
	if c.inArrayLength {
		declaration.UsedByArrayLength = true
	} else {
		declaration.Used = true
	}
}

func (c *Checker) resolveArrayLength(ref ast.TypeRef) bool {
	expr := ref.FixedLengthExpression
	if expr == nil {
		return true
	}
	if c.arrayLengthChecks[expr] {
		c.report(expr.GetSpan(), "array length has a declaration cycle")
		return false
	}
	c.arrayLengthChecks[expr] = true
	defer delete(c.arrayLengthChecks, expr)
	// A checked type-level constant is not a runtime initializer dependency.
	previous := c.globalDependencyOwner
	previousArrayLength := c.inArrayLength
	c.globalDependencyOwner = ""
	c.inArrayLength = true
	defer func() { c.globalDependencyOwner, c.inArrayLength = previous, previousArrayLength }()
	actual := c.checkExpression(expr)
	if actual.Kind == Invalid {
		return false
	}
	info, known := c.scalarConstant(expr)
	if !known || info.Value == nil || !initializerEmitsConstant(expr) {
		c.report(expr.GetSpan(), "array length must be a compile-time integer constant")
		return false
	}
	// Integral untyped floating/complex constants are permitted, but typed
	// floats, fractions, strings and booleans must not acquire an integer type.
	integer := constant.ToInt(info.Value)
	basic, basicType := gotypes.Unalias(info.Type).Underlying().(*gotypes.Basic)
	if integer.Kind() != constant.Int || !basicType || basic.Info()&(gotypes.IsInteger|gotypes.IsUntyped) == 0 {
		c.report(expr.GetSpan(), "array length must be a compile-time integer constant")
		return false
	}
	value, ok := new(big.Int).SetString(integer.ExactString(), 10)
	if !ok || value.Sign() < 0 {
		c.report(expr.GetSpan(), "array length must not be negative")
		return false
	}
	if !value.IsInt64() || !c.integerConstantFitsFixedType(value, builtins["int"]) {
		c.report(expr.GetSpan(), "array length is out of range for int")
		return false
	}
	*ref.FixedLength = value.Int64()
	return true
}
