package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Capture from the fully lowered call, preserving helper receivers and generic
// instantiations as well as the contextual types of ordinary arguments.
func captureTaskCall(source *ast.CallExpr) (*goast.BlockStmt, *goast.CallExpr, error) {
	generated, err := generateExpression(source)
	if err != nil {
		return nil, nil, err
	}
	call := generated.(*goast.CallExpr)
	used := map[string]bool{"task": true}
	goast.Inspect(call, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			used[name.Name] = true
		}
		return true
	})
	for _, ref := range source.CaptureArgumentTypes {
		goast.Inspect(goType(ref), func(node goast.Node) bool {
			if name, ok := node.(*goast.Ident); ok {
				used[name.Name] = true
			}
			return true
		})
	}
	fresh := func() goast.Expr {
		for i := 0; ; i++ {
			name := fmt.Sprintf("__taskCapture%d", i)
			if !used[name] {
				used[name] = true
				return goast.NewIdent(name)
			}
		}
	}
	body := &goast.BlockStmt{}
	// Generic methods already lower to a capture factory followed by invocation.
	// Capture that factory's result now; run only its invocation in the worker.
	method := source.MultipleArgumentResult != nil
	if !source.GenericCall || method {
		name := fresh()
		body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{call.Fun}})
		call.Fun = name
	}
	if source.MultipleArgumentCount > 0 && !method {
		values := make([]goast.Expr, source.MultipleArgumentCount)
		for i := range values {
			values[i] = fresh()
		}
		body.List = append(body.List, &goast.AssignStmt{Lhs: values, Tok: token.DEFINE, Rhs: call.Args})
		call.Args = values
	} else if !method {
		receiverOffset := len(call.Args) - len(source.Arguments)
		for i, argument := range call.Args {
			name := fresh()
			index := i - receiverOffset
			if index >= 0 && index < len(source.CaptureArgumentTypes) {
				// Preserve the call's contextual type for nil and untyped constants.
				body.List = append(body.List, &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
					Names: []*goast.Ident{name.(*goast.Ident)}, Type: goType(source.CaptureArgumentTypes[index]), Values: []goast.Expr{argument},
				}}}})
			} else {
				body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{argument}})
			}
			call.Args[i] = name
		}
	}
	return body, call, nil
}
