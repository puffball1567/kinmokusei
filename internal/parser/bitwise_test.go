package parser

import (
	"testing"

	"github.com/puffball1567/onsentamago/internal/ast"
)

func TestBitwiseAndShiftPrecedenceMatchesGoGroups(t *testing.T) {
	program, diagnostics := parseSource(t, `function value(): int { return 1 | 2 ^ 3 + 4 << 5 &^ 6 * 7; }`)
	if diagnostics != 0 {
		t.Fatalf("diagnostics = %d", diagnostics)
	}
	returned := program.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt)
	root := requireBinaryOperator(t, returned.Value, "+")
	xor := requireBinaryOperator(t, root.Left, "^")
	requireBinaryOperator(t, xor.Left, "|")
	multiply := requireBinaryOperator(t, root.Right, "*")
	andNot := requireBinaryOperator(t, multiply.Left, "&^")
	requireBinaryOperator(t, andNot.Left, "<<")
}

func TestBitwiseLogicalAndUnaryBoundaries(t *testing.T) {
	program, diagnostics := parseSource(t, `function value(input: int): boolean { return ^input & 7 === 3 && false || true; }`)
	if diagnostics != 0 {
		t.Fatalf("diagnostics = %d", diagnostics)
	}
	returned := program.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt)
	logicalOr := requireBinaryOperator(t, returned.Value, "||")
	logicalAnd := requireBinaryOperator(t, logicalOr.Left, "&&")
	equal := requireBinaryOperator(t, logicalAnd.Left, "===")
	bitwiseAnd := requireBinaryOperator(t, equal.Left, "&")
	complement, ok := bitwiseAnd.Left.(*ast.UnaryExpr)
	if !ok || complement.Operator != "^" {
		t.Fatalf("left operand = %#v, want unary ^", bitwiseAnd.Left)
	}
}

func TestShiftRightAndNestedGenericClosersAreUnambiguous(t *testing.T) {
	program, diagnostics := parseSource(t, `function shift(value: int): int { return value >> 2; } function nested(value: Outer<Middle<Inner<int>>>): void {}`)
	if diagnostics != 0 {
		t.Fatalf("diagnostics = %d", diagnostics)
	}
	shift := program.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt)
	requireBinaryOperator(t, shift.Value, ">>")
	parameter := program.Declarations[1].(*ast.FunctionDecl).Parameters[0].Type
	if parameter.Name != "Outer" || len(parameter.GenericArguments) != 1 || parameter.GenericArguments[0].Name != "Middle" || len(parameter.GenericArguments[0].GenericArguments) != 1 || parameter.GenericArguments[0].GenericArguments[0].Name != "Inner" {
		t.Fatalf("nested type = %#v", parameter)
	}
}

func requireBinaryOperator(t *testing.T, expression ast.Expression, operator string) *ast.BinaryExpr {
	t.Helper()
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || binary.Operator != operator {
		t.Fatalf("expression = %#v, want binary %s", expression, operator)
	}
	return binary
}
