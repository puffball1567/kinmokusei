package sema

import (
	"fmt"
	"go/constant"
	gotypes "go/types"
	"math/big"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Use only constants that really survive lowering as Go constants. A source
// const initialized from another binding may instead lower to a Go variable.
func (c *Checker) checkedNumericConstant(expr ast.Expression, actual Type) (gotypes.TypeAndValue, bool) {
	if id, ok := expr.(*ast.IdentifierExpr); ok {
		if symbol, found := c.lookupSymbol(id.Name, id.Span); found {
			actual = symbol.typeInfo
		}
	}
	pkg := gotypes.NewPackage("kinmokusei.synthetic/context", "context")
	if _, ok := c.numericOperand(pkg, "value", expr, actual); !ok {
		return gotypes.TypeAndValue{}, false
	}
	value, ok := pkg.Scope().Lookup("value").(*gotypes.Const)
	if !ok {
		return gotypes.TypeAndValue{}, false
	}
	return gotypes.TypeAndValue{Type: value.Type(), Value: value.Val()}, true
}

func (c *Checker) constantStringLength(expr ast.Expression) int64 {
	if id, ok := expr.(*ast.IdentifierExpr); ok {
		if symbol, found := c.lookupSymbol(id.Name, id.Span); found && symbol.constant && symbol.declaration != nil {
			expr = symbol.declaration.Value
		}
	}
	if literal, ok := expr.(*ast.LiteralExpr); ok && literal.Kind == ast.StringLiteral {
		if value, err := strconv.Unquote(literal.Text); err == nil {
			return int64(len(value))
		}
	}
	return -1
}

func (c *Checker) integerContextValue(expr ast.Expression) (*big.Int, bool) {
	info, ok := c.checkedNumericConstant(expr, builtins["int"])
	if !ok {
		return nil, false
	}
	value := constant.ToInt(info.Value)
	if value.Kind() != constant.Int {
		return nil, false
	}
	return new(big.Int).SetString(value.ExactString(), 10)
}

func (c *Checker) isIntegerContext(expr ast.Expression, actual Type) bool {
	if actual.IsInteger() {
		return true
	}
	info, ok := c.checkedNumericConstant(expr, actual)
	if !ok {
		return false
	}
	basic, ok := info.Type.Underlying().(*gotypes.Basic)
	return ok && basic.Info()&gotypes.IsUntyped != 0 && constant.ToInt(info.Value).Kind() == constant.Int
}

func (c *Checker) checkSequenceIndex(expr ast.Expression, actual Type, length int64, kind string) {
	if actual.Kind == Invalid {
		return
	}
	if !c.isIntegerContext(expr, actual) {
		c.report(expr.GetSpan(), kind+" index must be an integer")
		return
	}
	if value, ok := c.integerContextValue(expr); ok {
		// Sema does not yet receive target sizes. Reject values exceeding every
		// supported int width; generated Go checks the selected architecture.
		switch {
		case value.Sign() < 0:
			c.report(expr.GetSpan(), kind+" index cannot be negative")
		case !value.IsInt64():
			c.report(expr.GetSpan(), kind+" index is out of range for int")
		case length >= 0 && value.Cmp(big.NewInt(length)) >= 0:
			c.report(expr.GetSpan(), fmt.Sprintf("%s index %s is out of bounds for length %d", kind, value, length))
		}
	}
}
