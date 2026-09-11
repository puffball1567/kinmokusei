package sema

import (
	"math/big"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type constructorRangeProofs map[source.Span]struct{}

func (proofs constructorRangeProofs) contains(declaration source.Span) bool {
	_, ok := proofs[declaration]
	return ok
}

func constructorRangeProof(declaration source.Span) constructorRangeProofs {
	if declaration.Path == "" {
		return nil
	}
	return constructorRangeProofs{declaration: {}}
}

func unionConstructorRangeProofs(left, right constructorRangeProofs) constructorRangeProofs {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	combined := make(constructorRangeProofs, len(left)+len(right))
	for declaration := range left {
		combined[declaration] = struct{}{}
	}
	for declaration := range right {
		combined[declaration] = struct{}{}
	}
	return combined
}

func intersectConstructorRangeProofs(left, right constructorRangeProofs) constructorRangeProofs {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	common := make(constructorRangeProofs)
	for declaration := range left {
		if right.contains(declaration) {
			common[declaration] = struct{}{}
		}
	}
	if len(common) == 0 {
		return nil
	}
	return common
}

func constructorNonEmptyRangeGuard(expression ast.Expression) (constructorRangeProofs, constructorRangeProofs) {
	if unary, ok := expression.(*ast.UnaryExpr); ok && unary.Operator == "!" {
		whenTrue, whenFalse := constructorNonEmptyRangeGuard(unary.Operand)
		return whenFalse, whenTrue
	}
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok {
		return nil, nil
	}
	if binary.Operator == "&&" || binary.Operator == "||" {
		// Carry proofs through compound guards only when neither side can mutate
		// a collection between its length check and the guarded range.
		if !constructorRangeGuardStable(binary.Left) || !constructorRangeGuardStable(binary.Right) {
			return nil, nil
		}
		leftTrue, leftFalse := constructorNonEmptyRangeGuard(binary.Left)
		rightTrue, rightFalse := constructorNonEmptyRangeGuard(binary.Right)
		if binary.Operator == "&&" {
			return unionConstructorRangeProofs(leftTrue, rightTrue), intersectConstructorRangeProofs(leftFalse, rightFalse)
		}
		return intersectConstructorRangeProofs(leftTrue, rightTrue), unionConstructorRangeProofs(leftFalse, rightFalse)
	}
	declaration, constant, operator, ok := constructorLengthComparison(binary.Left, binary.Right, binary.Operator)
	if !ok {
		declaration, constant, operator, ok = constructorLengthComparison(binary.Right, binary.Left, reverseComparisonOperator(binary.Operator))
	}
	if !ok {
		return nil, nil
	}
	trueNonEmpty, falseNonEmpty := lengthComparisonProvesNonEmpty(operator, constant)
	var whenTrue, whenFalse constructorRangeProofs
	if trueNonEmpty {
		whenTrue = constructorRangeProof(declaration)
	}
	if falseNonEmpty {
		whenFalse = constructorRangeProof(declaration)
	}
	return whenTrue, whenFalse
}

// constructorNonEmptyRangeSwitch proves branch-local facts for a value switch
// whose subject is len(collection). A case is non-empty only when every value
// in that clause is a known positive integer. The default is non-empty when a
// case explicitly covers zero, because len cannot be negative. As with guarded
// branches, the fact can cross side-effect-free local declarations before the
// first range or nested guard in the selected body.
func constructorNonEmptyRangeSwitch(statement *ast.ValueSwitchStmt) []constructorRangeProofs {
	proofs := make([]constructorRangeProofs, len(statement.Cases))
	call, ok := statement.Value.(*ast.CallExpr)
	if !ok || call.Builtin != ast.LenCall || len(call.Arguments) != 1 {
		return proofs
	}
	declaration := constructorRangeSourceDeclaration(call.Arguments[0])
	if declaration.Path == "" {
		return proofs
	}

	zeroCovered := false
	stableCases := true
	for index := range statement.Cases {
		clause := &statement.Cases[index]
		if clause.Default || len(clause.Values) == 0 {
			continue
		}
		allPositive := true
		for _, value := range clause.Values {
			stableCases = stableCases && constructorRangeGuardStable(value)
			constant, known := integerConstantValue(value)
			if !known || constant.Sign() <= 0 {
				allPositive = false
			}
			if known && constant.Sign() == 0 {
				zeroCovered = true
			}
		}
		if allPositive {
			proofs[index] = constructorRangeProof(declaration)
		}
	}
	if !stableCases {
		return make([]constructorRangeProofs, len(statement.Cases))
	}
	if zeroCovered {
		for index := range statement.Cases {
			if statement.Cases[index].Default {
				proofs[index] = constructorRangeProof(declaration)
			}
		}
	}
	return proofs
}

func constructorRangeGuardStable(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr, *ast.LiteralExpr:
		return true
	case *ast.UnaryExpr:
		return constructorRangeGuardStable(expression.Operand)
	case *ast.BinaryExpr:
		return constructorRangeGuardStable(expression.Left) && constructorRangeGuardStable(expression.Right)
	case *ast.CallExpr:
		return expression.Builtin == ast.LenCall && len(expression.Arguments) == 1 && constructorRangeSourceDeclaration(expression.Arguments[0]).Path != ""
	default:
		return false
	}
}

func constructorLengthComparison(left, right ast.Expression, operator string) (source.Span, *big.Int, string, bool) {
	call, ok := left.(*ast.CallExpr)
	if !ok || call.Builtin != ast.LenCall || len(call.Arguments) != 1 {
		return source.Span{}, nil, "", false
	}
	declaration := constructorRangeSourceDeclaration(call.Arguments[0])
	if declaration.Path == "" {
		return source.Span{}, nil, "", false
	}
	constant, known := integerConstantValue(right)
	if !known {
		return source.Span{}, nil, "", false
	}
	return declaration, constant, operator, true
}

func constructorRangeSourceDeclaration(expression ast.Expression) source.Span {
	identifier, ok := expression.(*ast.IdentifierExpr)
	if !ok {
		return source.Span{}
	}
	return identifier.ResolvedDeclaration
}

func reverseComparisonOperator(operator string) string {
	switch operator {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	default:
		return operator
	}
}

func lengthComparisonProvesNonEmpty(operator string, constant *big.Int) (bool, bool) {
	zero := big.NewInt(0)
	positive := constant.Sign() > 0
	nonNegative := constant.Sign() >= 0
	switch operator {
	case ">":
		return nonNegative, false
	case ">=":
		return positive, false
	case "<":
		return false, positive
	case "<=":
		return false, nonNegative
	case "==", "===":
		return positive, constant.Cmp(zero) == 0
	case "!=", "!==":
		return constant.Cmp(zero) == 0, positive
	default:
		return false, false
	}
}
