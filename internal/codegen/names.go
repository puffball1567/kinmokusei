package codegen

import (
	"strings"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/goname"
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
	return goname.Identifier(name)
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
