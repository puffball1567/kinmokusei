package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateReferenceRead(value goast.Expr, ref ast.TypeRef) goast.Expr {
	message := stringLiteral(nonNullMessage(ref.Span, "non-null reference read"))
	name := ""
	if !ref.TypeParameter {
		switch ref.NonNullKinds {
		case 1:
			name = "__kinmokuseiReadPointer"
		case 4:
			name = "__kinmokuseiReadInterface"
		}
	}
	if name != "" {
		return &goast.CallExpr{Fun: goast.NewIdent(name), Args: []goast.Expr{value, message}}
	}
	return &goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiValidateNonNullKinds"), Args: []goast.Expr{value, &goast.BasicLit{Kind: token.INT, Value: fmt.Sprint(ref.NonNullKinds)}, message, goast.NewIdent("true")}}
}

func referenceReadRuntimeDeclarations(file *goast.File) ([]goast.Decl, error) {
	needed := false
	goast.Inspect(file, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok && (name.Name == "__kinmokuseiReadPointer" || name.Name == "__kinmokuseiReadInterface") {
			needed = true
		}
		return true
	})
	if !needed {
		return nil, nil
	}
	return parseDeferSupport(`package runtime
func __kinmokuseiReadPointer[P ~*E,E any](value P,message string)P{if value==nil{panic(message)};return value}
func __kinmokuseiReadInterface[I any](value I,message string)I{if interface{}(value)==nil{panic(message)};return value}
`)
}
