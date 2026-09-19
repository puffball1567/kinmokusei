package codegen

import (
	"go/token"
	"strings"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func memberName(name string, visibility kinmokuseiAST.Visibility) string {
	if visibility != kinmokuseiAST.Public || name == "" {
		return name
	}
	first := []rune(name)
	first[0] = []rune(strings.ToUpper(string(first[0])))[0]
	return string(first)
}

func staticMethodName(className, methodName string, visibility kinmokuseiAST.Visibility) string {
	if visibility == kinmokuseiAST.Public {
		return goName(className + methodName)
	}
	return goName("__kinmokuseiStatic" + className + methodName)
}

func goName(name string) string {
	if token.Lookup(name).IsKeyword() || isGoPredeclaredName(name) {
		return name + "_"
	}
	return name
}

func isGoPredeclaredName(name string) bool {
	switch name {
	case "append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new", "panic", "print", "println", "real", "recover":
		return true
	default:
		return false
	}
}

func goTypeName(name string) string {
	switch name {
	case "boolean":
		return "bool"
	case "float", "number", "float64":
		return "float64"
	default:
		return name
	}
}

func enumMemberGoName(enumName, memberNameValue string) string {
	return enumName + memberName(memberNameValue, kinmokuseiAST.Public)
}
