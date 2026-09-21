package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func captureMultipleTaskCall(source *ast.CallExpr) (*goast.BlockStmt, *goast.CallExpr, error) {
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
	if !source.MultipleArgumentGeneric || method {
		name := fresh()
		body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{call.Fun}})
		call.Fun = name
	}
	if !method {
		values := make([]goast.Expr, source.MultipleArgumentCount)
		for i := range values {
			values[i] = fresh()
		}
		body.List = append(body.List, &goast.AssignStmt{Lhs: values, Tok: token.DEFINE, Rhs: call.Args})
		call.Args = values
	}
	return body, call, nil
}
