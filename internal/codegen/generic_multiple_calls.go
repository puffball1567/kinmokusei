package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// A lowered generic method has an extra receiver argument, so Go cannot expand
// the source call directly. Capture the receiver before evaluating the producer.
func generateGenericMultipleCall(source *ast.CallExpr, call *goast.CallExpr) goast.Expr {
	return generateCapturedMultipleCall(source, call, true)
}

// Explicit bindings also avoid a Go 1.26+ vet panic when collection built-ins
// receive a tuple directly. The capture factory preserves go/defer timing.
func generateCapturedMultipleCall(source *ast.CallExpr, call *goast.CallExpr, hasReceiver bool) goast.Expr {
	used := map[string]bool{}
	results := functionResults(*source.MultipleArgumentResult)
	reserve := func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			used[name.Name] = true
		}
		return true
	}
	goast.Inspect(call, reserve)
	if results != nil {
		goast.Inspect(results, reserve)
	}
	fresh := func(base string) goast.Expr {
		name := base
		for i := 1; used[name]; i++ {
			name = base + strconv.Itoa(i)
		}
		used[name] = true
		return goast.NewIdent(name)
	}
	var body []goast.Stmt
	var arguments []goast.Expr
	producer := call.Args[0]
	if hasReceiver {
		receiver := fresh("__multipleReceiver")
		body = append(body, &goast.AssignStmt{Lhs: []goast.Expr{receiver}, Tok: token.DEFINE, Rhs: []goast.Expr{call.Args[0]}})
		arguments = append(arguments, receiver)
		producer = call.Args[1]
	}
	values := make([]goast.Expr, source.MultipleArgumentCount)
	for i := range values {
		values[i] = fresh("__multipleValue")
	}
	body = append(body, &goast.AssignStmt{Lhs: values, Tok: token.DEFINE, Rhs: []goast.Expr{producer}})
	call.Args = append(arguments, values...)
	var invoke goast.Stmt
	if results == nil || len(results.List) == 0 {
		invoke = &goast.ExprStmt{X: call}
	} else {
		invoke = &goast.ReturnStmt{Results: []goast.Expr{call}}
	}
	// Return a captured invocation so go/defer evaluate the receiver and source
	// arguments immediately, while only the method body is delayed.
	invokeType := &goast.FuncType{Params: &goast.FieldList{}, Results: results}
	body = append(body, &goast.ReturnStmt{Results: []goast.Expr{&goast.FuncLit{
		Type: invokeType, Body: &goast.BlockStmt{List: []goast.Stmt{invoke}},
	}}})
	return &goast.CallExpr{Fun: &goast.CallExpr{Fun: &goast.FuncLit{
		Type: &goast.FuncType{Params: &goast.FieldList{}, Results: &goast.FieldList{List: []*goast.Field{{Type: invokeType}}}},
		Body: &goast.BlockStmt{List: body},
	}}}
}
