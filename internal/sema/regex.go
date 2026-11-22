package sema

import (
	"fmt"
	"go/token"
	gotypes "go/types"
	"regexp"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Model an opaque, non-comparable value for Go generic type checking. The
// emitted wrapper keeps the implementation pointer private.
var regexGoType = gotypes.NewNamed(
	gotypes.NewTypeName(token.NoPos, nil, ast.RegexTypeName, nil),
	gotypes.NewStruct([]*gotypes.Var{
		gotypes.NewField(token.NoPos, nil, "_", gotypes.NewArray(gotypes.NewSignatureType(nil, nil, nil, nil, nil, false), 0), false),
		gotypes.NewField(token.NoPos, nil, "compiled", gotypes.NewPointer(gotypes.NewStruct(nil, nil)), false),
	}, nil),
	nil,
)

func (c *Checker) checkRegexLiteral(expr *ast.RegexExpr) Type {
	if _, err := regexp.Compile(expr.Pattern); err != nil {
		c.report(expr.Span, fmt.Sprintf("invalid regular expression: %v", err))
		return Type{Kind: Invalid}
	}
	return builtins[ast.RegexTypeName]
}

func (c *Checker) checkRegexMember(expr *ast.MemberExpr, write bool) Type {
	for _, method := range ast.RegexMethods() {
		if expr.Name != method.Name {
			continue
		}
		if write {
			c.report(expr.NameSpan, "Regex methods cannot be assigned")
			return Type{Kind: Invalid}
		}
		signature := method.Signature
		parameters := make([]Type, len(signature.ParameterTypes))
		for i, name := range signature.ParameterTypes {
			parameters[i] = builtins[name]
		}
		result := builtins[signature.Result]
		if signature.Result == "string[]" {
			text := builtins["string"]
			result = Type{Kind: Array, Name: "array", Element: &text}
		}
		expr.ResolvedName = expr.Name
		expr.BoundMethod = true
		return Type{Kind: Function, Name: "function", Parameters: parameters, ParameterNames: signature.ParameterNames, Result: &result}
	}
	c.report(expr.NameSpan, fmt.Sprintf("Regex has no member %q", expr.Name))
	return Type{Kind: Invalid}
}
