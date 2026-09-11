package sema

import (
	"go/constant"
	gotypes "go/types"
	"math/big"
	"strconv"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// go/types rejects constant shifts above this implementation bound. Diagnose
// the same restriction while the Kinmokusei source span is still available.
const maximumGoConstantShift = 1074

func expressionAlwaysTrue(expression ast.Expression) bool {
	value, known := booleanConstantValue(expression)
	return known && value
}

func (c *Checker) expressionAlwaysTrue(expression ast.Expression) bool {
	value, known := c.resolvedBooleanConstantValue(expression, map[source.Span]bool{})
	return known && value
}

func (c *Checker) resolvedBooleanConstantValue(expression ast.Expression, seen map[source.Span]bool) (bool, bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		if value := c.namedGoConstantValue(expression); value != nil && value.Kind() == constant.Bool {
			return constant.BoolVal(value), true
		}
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declarationSpan] {
			return false, false
		}
		seen[symbol.declarationSpan] = true
		value, known := c.resolvedBooleanConstantValue(symbol.declaration.Value, seen)
		delete(seen, symbol.declarationSpan)
		return value, known
	case *ast.LiteralExpr:
		if expression.Kind != ast.BooleanLiteral {
			return false, false
		}
		return expression.Text == "true", expression.Text == "true" || expression.Text == "false"
	case *ast.UnaryExpr:
		if expression.Operator != "!" {
			return false, false
		}
		value, known := c.resolvedBooleanConstantValue(expression.Operand, seen)
		return !value, known
	case *ast.BinaryExpr:
		switch expression.Operator {
		case "&&", "||":
			left, leftKnown := c.resolvedBooleanConstantValue(expression.Left, seen)
			right, rightKnown := c.resolvedBooleanConstantValue(expression.Right, seen)
			if !leftKnown || !rightKnown {
				return false, false
			}
			if expression.Operator == "&&" {
				return left && right, true
			}
			return left || right, true
		case "==", "===", "!=", "!==":
			if left, leftKnown := c.resolvedBooleanConstantValue(expression.Left, seen); leftKnown {
				if right, rightKnown := c.resolvedBooleanConstantValue(expression.Right, seen); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
			if left, leftKnown := c.resolvedIntegerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := c.resolvedIntegerConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left.Cmp(right) == 0, expression.Operator), true
				}
			}
			if left, leftKnown := c.resolvedStringConstantValue(expression.Left, seen); leftKnown {
				if right, rightKnown := c.resolvedStringConstantValue(expression.Right, seen); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
		case "<", "<=", ">", ">=":
			if left, leftKnown := c.resolvedIntegerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := c.resolvedIntegerConstantValue(expression.Right); rightKnown {
					return compareConstantOrdering(left.Cmp(right), expression.Operator), true
				}
			}
			if left, leftKnown := c.resolvedStringConstantValue(expression.Left, seen); leftKnown {
				if right, rightKnown := c.resolvedStringConstantValue(expression.Right, seen); rightKnown {
					return compareConstantOrdering(strings.Compare(left, right), expression.Operator), true
				}
			}
		}
	}
	return false, false
}

func (c *Checker) resolvedStringConstantValue(expression ast.Expression, seen map[source.Span]bool) (string, bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		if value := c.namedGoConstantValue(expression); value != nil && value.Kind() == constant.String {
			return constant.StringVal(value), true
		}
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declarationSpan] {
			return "", false
		}
		seen[symbol.declarationSpan] = true
		value, known := c.resolvedStringConstantValue(symbol.declaration.Value, seen)
		delete(seen, symbol.declarationSpan)
		return value, known
	case *ast.LiteralExpr:
		if expression.Kind != ast.StringLiteral {
			return "", false
		}
		value, err := strconv.Unquote(expression.Text)
		return value, err == nil
	case *ast.BinaryExpr:
		if expression.Operator != "+" {
			return "", false
		}
		left, leftKnown := c.resolvedStringConstantValue(expression.Left, seen)
		right, rightKnown := c.resolvedStringConstantValue(expression.Right, seen)
		if !leftKnown || !rightKnown {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

func (c *Checker) rangeExpressionGuaranteedNonEmpty(expression ast.Expression) bool {
	return c.resolvedRangeExpressionGuaranteedNonEmpty(expression, map[source.Span]bool{})
}

func (c *Checker) resolvedRangeExpressionGuaranteedNonEmpty(expression ast.Expression, seen map[source.Span]bool) bool {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declarationSpan] {
			return false
		}
		seen[symbol.declarationSpan] = true
		guaranteed := c.resolvedRangeExpressionGuaranteedNonEmpty(symbol.declaration.Value, seen)
		delete(seen, symbol.declarationSpan)
		return guaranteed
	case *ast.ArrayLiteralExpr:
		return len(expression.Elements) != 0
	case *ast.CallExpr:
		switch expression.Builtin {
		case ast.AppendCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			if c.resolvedRangeExpressionGuaranteedNonEmpty(expression.Arguments[0], seen) {
				return true
			}
			if !expression.Expanded {
				return len(expression.Arguments) > 1
			}
			return len(expression.Arguments) == 2 && c.resolvedRangeExpressionGuaranteedNonEmpty(expression.Arguments[1], seen)
		case ast.MakeSliceCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			length, known := c.resolvedIntegerConstantValue(expression.Arguments[0])
			return known && length.Sign() > 0
		default:
			return false
		}
	default:
		value, known := c.resolvedStringConstantValue(expression, seen)
		return known && value != ""
	}
}

func rangeExpressionGuaranteedNonEmpty(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.ArrayLiteralExpr:
		return len(expression.Elements) != 0
	case *ast.CallExpr:
		switch expression.Builtin {
		case ast.AppendCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			if rangeExpressionGuaranteedNonEmpty(expression.Arguments[0]) {
				return true
			}
			if !expression.Expanded {
				return len(expression.Arguments) > 1
			}
			return len(expression.Arguments) == 2 && rangeExpressionGuaranteedNonEmpty(expression.Arguments[1])
		case ast.MakeSliceCall:
			if len(expression.Arguments) == 0 {
				return false
			}
			length, known := integerConstantValue(expression.Arguments[0])
			return known && length.Sign() > 0
		default:
			return false
		}
	default:
		value, known := stringConstantValue(expression)
		return known && value != ""
	}
}

func booleanConstantValue(expression ast.Expression) (bool, bool) {
	switch expression := expression.(type) {
	case *ast.LiteralExpr:
		if expression.Kind != ast.BooleanLiteral {
			return false, false
		}
		return expression.Text == "true", expression.Text == "true" || expression.Text == "false"
	case *ast.UnaryExpr:
		if expression.Operator != "!" {
			return false, false
		}
		value, known := booleanConstantValue(expression.Operand)
		return !value, known
	case *ast.BinaryExpr:
		switch expression.Operator {
		case "&&", "||":
			left, leftKnown := booleanConstantValue(expression.Left)
			right, rightKnown := booleanConstantValue(expression.Right)
			if !leftKnown || !rightKnown {
				return false, false
			}
			if expression.Operator == "&&" {
				return left && right, true
			}
			return left || right, true
		case "==", "===", "!=", "!==":
			if left, leftKnown := booleanConstantValue(expression.Left); leftKnown {
				if right, rightKnown := booleanConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
			if left, leftKnown := integerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := integerConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left.Cmp(right) == 0, expression.Operator), true
				}
			}
			if left, leftKnown := stringConstantValue(expression.Left); leftKnown {
				if right, rightKnown := stringConstantValue(expression.Right); rightKnown {
					return compareConstantEquality(left == right, expression.Operator), true
				}
			}
		case "<", "<=", ">", ">=":
			if left, leftKnown := integerConstantValue(expression.Left); leftKnown {
				if right, rightKnown := integerConstantValue(expression.Right); rightKnown {
					return compareConstantOrdering(left.Cmp(right), expression.Operator), true
				}
			}
			if left, leftKnown := stringConstantValue(expression.Left); leftKnown {
				if right, rightKnown := stringConstantValue(expression.Right); rightKnown {
					return compareConstantOrdering(strings.Compare(left, right), expression.Operator), true
				}
			}
		}
	}
	return false, false
}

func compareConstantEquality(equal bool, operator string) bool {
	if operator == "!=" || operator == "!==" {
		return !equal
	}
	return equal
}

func compareConstantOrdering(comparison int, operator string) bool {
	switch operator {
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case ">":
		return comparison > 0
	case ">=":
		return comparison >= 0
	default:
		return false
	}
}

func stringConstantValue(expression ast.Expression) (string, bool) {
	switch expression := expression.(type) {
	case *ast.LiteralExpr:
		if expression.Kind != ast.StringLiteral {
			return "", false
		}
		value, err := strconv.Unquote(expression.Text)
		return value, err == nil
	case *ast.BinaryExpr:
		if expression.Operator != "+" {
			return "", false
		}
		left, leftKnown := stringConstantValue(expression.Left)
		right, rightKnown := stringConstantValue(expression.Right)
		if !leftKnown || !rightKnown {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

func integerConstantValue(expression ast.Expression) (*big.Int, bool) {
	return integerConstantValueWithResolver(expression, nil)
}

func integerConstantValueWithResolver(expression ast.Expression, resolve func(*ast.IdentifierExpr) (*big.Int, bool)) (*big.Int, bool) {
	switch expression := expression.(type) {
	case *ast.LiteralExpr:
		if expression.Kind != ast.IntegerLiteral {
			return nil, false
		}
		value, ok := new(big.Int).SetString(expression.Text, 0)
		return value, ok
	case *ast.UnaryExpr:
		value, ok := integerConstantValueWithResolver(expression.Operand, resolve)
		if !ok {
			return nil, false
		}
		switch expression.Operator {
		case "+":
			return value, true
		case "-":
			return new(big.Int).Neg(value), true
		case "^":
			return new(big.Int).Not(value), true
		default:
			return nil, false
		}
	case *ast.BinaryExpr:
		left, leftOK := integerConstantValueWithResolver(expression.Left, resolve)
		right, rightOK := integerConstantValueWithResolver(expression.Right, resolve)
		if !leftOK || !rightOK {
			return nil, false
		}
		switch expression.Operator {
		case "+":
			return new(big.Int).Add(left, right), true
		case "-":
			return new(big.Int).Sub(left, right), true
		case "*":
			return new(big.Int).Mul(left, right), true
		case "/":
			if right.Sign() != 0 {
				return new(big.Int).Quo(left, right), true
			}
		case "%":
			if right.Sign() != 0 {
				return new(big.Int).Rem(left, right), true
			}
		case "&":
			return new(big.Int).And(left, right), true
		case "|":
			return new(big.Int).Or(left, right), true
		case "^":
			return new(big.Int).Xor(left, right), true
		case "&^":
			return new(big.Int).AndNot(left, right), true
		case "<<":
			if right.Sign() >= 0 && right.IsUint64() {
				return new(big.Int).Lsh(left, uint(right.Uint64())), true
			}
		case ">>":
			if right.Sign() >= 0 && right.IsUint64() {
				return new(big.Int).Rsh(left, uint(right.Uint64())), true
			}
		}
	case *ast.CallExpr:
		name, ok := expression.Callee.(*ast.IdentifierExpr)
		if !ok || expression.Expanded || len(expression.Arguments) != 1 {
			return nil, false
		}
		target, ok := LookupType(name.Name)
		if !ok || !target.IsInteger() {
			return nil, false
		}
		return integerConstantValueWithResolver(expression.Arguments[0], resolve)
	case *ast.IdentifierExpr:
		if resolve != nil {
			return resolve(expression)
		}
	}
	return nil, false
}

func (c *Checker) resolvedIntegerConstantValue(expression ast.Expression) (*big.Int, bool) {
	seen := map[*ast.VariableDecl]bool{}
	var resolve func(*ast.IdentifierExpr) (*big.Int, bool)
	resolve = func(identifier *ast.IdentifierExpr) (*big.Int, bool) {
		if value := c.namedGoConstantValue(identifier); value != nil && value.Kind() == constant.Int {
			return new(big.Int).SetString(value.ExactString(), 10)
		}
		symbol, ok := c.lookupSymbol(identifier.Name, identifier.Span)
		if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declaration] {
			return nil, false
		}
		seen[symbol.declaration] = true
		value, known := integerConstantValueWithResolver(symbol.declaration.Value, resolve)
		delete(seen, symbol.declaration)
		return value, known
	}
	return integerConstantValueWithResolver(expression, resolve)
}

func (c *Checker) integerExpressionIsCompileTimeConstant(expression ast.Expression) bool {
	seen := map[*ast.VariableDecl]bool{}
	var check func(ast.Expression) bool
	check = func(expression ast.Expression) bool {
		switch expression := expression.(type) {
		case *ast.IdentifierExpr:
			if c.namedGoConstant(expression) != nil {
				return true
			}
			symbol, ok := c.lookupSymbol(expression.Name, expression.Span)
			if !ok || !symbol.constant || symbol.declaration == nil || seen[symbol.declaration] {
				return false
			}
			seen[symbol.declaration] = true
			constant := check(symbol.declaration.Value)
			delete(seen, symbol.declaration)
			return constant
		case *ast.LiteralExpr:
			return expression.Kind == ast.IntegerLiteral
		case *ast.UnaryExpr:
			return check(expression.Operand)
		case *ast.BinaryExpr:
			return check(expression.Left) && check(expression.Right)
		case *ast.CallExpr:
			name, ok := expression.Callee.(*ast.IdentifierExpr)
			if !ok || expression.Expanded || len(expression.Arguments) != 1 {
				return false
			}
			target, ok := LookupType(name.Name)
			return ok && target.IsInteger() && check(expression.Arguments[0])
		case *ast.MemberExpr:
			return expression.Constant
		default:
			return false
		}
	}
	return check(expression)
}

func integerConstantFitsFixedType(value *big.Int, target Type) bool {
	goType, ok := goTypeOf(target)
	if !ok {
		return true
	}
	basic, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Basic)
	if !ok {
		return true
	}
	var bits uint
	var signed bool
	switch basic.Kind() {
	case gotypes.Int8:
		bits, signed = 8, true
	case gotypes.Uint8:
		bits = 8
	case gotypes.Uint:
		return value.Sign() >= 0
	case gotypes.Int16:
		bits, signed = 16, true
	case gotypes.Uint16:
		bits = 16
	case gotypes.Int32:
		bits, signed = 32, true
	case gotypes.Uint32:
		bits = 32
	case gotypes.Int64:
		bits, signed = 64, true
	case gotypes.Uint64:
		bits = 64
	default:
		// int, uint, and uintptr depend on the selected build target. Generated
		// Go validation remains authoritative until target sizes are part of sema.
		return true
	}
	if signed {
		limit := new(big.Int).Lsh(big.NewInt(1), bits-1)
		minimum := new(big.Int).Neg(new(big.Int).Set(limit))
		maximum := new(big.Int).Sub(limit, big.NewInt(1))
		return value.Cmp(minimum) >= 0 && value.Cmp(maximum) <= 0
	}
	if value.Sign() < 0 {
		return false
	}
	limit := new(big.Int).Lsh(big.NewInt(1), bits)
	return value.Cmp(limit) < 0
}
