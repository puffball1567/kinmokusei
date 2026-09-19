package codegen

import (
	goast "go/ast"
	"go/token"
	"strings"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func goIntegerConstant(value string) goast.Expr {
	if strings.HasPrefix(value, "-") {
		return &goast.UnaryExpr{Op: token.SUB, X: &goast.BasicLit{Kind: token.INT, Value: strings.TrimPrefix(value, "-")}}
	}
	return &goast.BasicLit{Kind: token.INT, Value: value}
}

func goToken(operator string) token.Token {
	switch operator {
	case "+":
		return token.ADD
	case "-":
		return token.SUB
	case "*":
		return token.MUL
	case "/":
		return token.QUO
	case "%":
		return token.REM
	case "!":
		return token.NOT
	case "&":
		return token.AND
	case "|":
		return token.OR
	case "^":
		return token.XOR
	case "<<":
		return token.SHL
	case ">>":
		return token.SHR
	case "&^":
		return token.AND_NOT
	case "<-":
		return token.ARROW
	case "==", "===":
		return token.EQL
	case "!=", "!==":
		return token.NEQ
	case "<":
		return token.LSS
	case "<=":
		return token.LEQ
	case ">":
		return token.GTR
	case ">=":
		return token.GEQ
	case "&&":
		return token.LAND
	case "||":
		return token.LOR
	default:
		return token.ILLEGAL
	}
}

func goAssignmentToken(operator string) token.Token {
	switch operator {
	case "", "=":
		return token.ASSIGN
	case "+=":
		return token.ADD_ASSIGN
	case "-=":
		return token.SUB_ASSIGN
	case "*=":
		return token.MUL_ASSIGN
	case "/=":
		return token.QUO_ASSIGN
	case "%=":
		return token.REM_ASSIGN
	case "&=":
		return token.AND_ASSIGN
	case "|=":
		return token.OR_ASSIGN
	case "^=":
		return token.XOR_ASSIGN
	case "&^=":
		return token.AND_NOT_ASSIGN
	case "<<=":
		return token.SHL_ASSIGN
	case ">>=":
		return token.SHR_ASSIGN
	default:
		return token.ILLEGAL
	}
}

func isGoConstant(expr kinmokuseiAST.Expression) bool {
	switch expr := expr.(type) {
	case *kinmokuseiAST.IdentifierExpr:
		return expr.GoConstant || expr.GoMember != nil && expr.GoMember.Constant
	case *kinmokuseiAST.LiteralExpr:
		return expr.Kind != kinmokuseiAST.NilLiteral && expr.Kind != kinmokuseiAST.NullLiteral
	case *kinmokuseiAST.UnaryExpr:
		return isGoConstant(expr.Operand)
	case *kinmokuseiAST.BinaryExpr:
		return isGoConstant(expr.Left) && isGoConstant(expr.Right)
	case *kinmokuseiAST.CallExpr:
		if expr.GoConstant {
			return true
		}
		name, ok := expr.Callee.(*kinmokuseiAST.IdentifierExpr)
		if !ok || !isBuiltinConversion(name.Name) {
			return false
		}
		return !expr.Expanded && len(expr.Arguments) == 1 && isGoConstant(expr.Arguments[0])
	case *kinmokuseiAST.MemberExpr:
		return expr.Constant
	default:
		return false
	}
}

func isBuiltinConversion(name string) bool {
	switch name {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float", "number", "float64", "byte":
		return true
	default:
		return false
	}
}
