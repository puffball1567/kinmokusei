package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func decoratorInvokeUnavailableReason(target *kinmokuseiAST.DecoratorTarget) string {
	if target.Invocable {
		return ""
	}
	if target.InvokeUnavailableReason != "" {
		return target.InvokeUnavailableReason
	}
	return "decorator target is not an invocable method"
}

func decoratorInvokeAdapter(target *kinmokuseiAST.DecoratorTarget) goast.Expr {
	receiver := goast.NewIdent("receiver")
	arguments := goast.NewIdent("arguments")
	valueType := goast.NewIdent("__kinmokuseiDecoratorValue")
	functionType := &goast.FuncType{
		Params: &goast.FieldList{List: []*goast.Field{
			{Names: []*goast.Ident{receiver}, Type: valueType},
			{Names: []*goast.Ident{arguments}, Type: &goast.ArrayType{Elt: valueType}},
		}},
		Results: &goast.FieldList{List: []*goast.Field{{Type: valueType}, {Type: goast.NewIdent("error")}}},
	}
	failure := func(message string) *goast.ReturnStmt {
		return &goast.ReturnStmt{Results: []goast.Expr{
			&goast.CompositeLit{Type: valueType},
			&goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiDecoratorAdapterError"), Args: []goast.Expr{stringLiteral(message)}},
		}}
	}
	body := &goast.BlockStmt{}
	if !target.Invocable {
		body.List = append(body.List, failure(decoratorInvokeUnavailableReason(target)))
		return &goast.FuncLit{Type: functionType, Body: body}
	}
	label := fmt.Sprintf("decorator method %s.%s", target.ClassName, target.MemberName)
	actualReceiver := goast.NewIdent("typedReceiver")
	receiverOK := goast.NewIdent("receiverOK")
	body.List = append(body.List,
		&goast.AssignStmt{Lhs: []goast.Expr{actualReceiver, receiverOK}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.TypeAssertExpr{
			X:    &goast.SelectorExpr{X: receiver, Sel: goast.NewIdent("value")},
			Type: &goast.StarExpr{X: goast.NewIdent(target.RuntimeClassName)},
		}}},
		&goast.IfStmt{Cond: &goast.BinaryExpr{
			X: &goast.UnaryExpr{Op: token.NOT, X: receiverOK}, Op: token.LOR,
			Y: &goast.BinaryExpr{X: actualReceiver, Op: token.EQL, Y: goast.NewIdent("nil")},
		}, Body: &goast.BlockStmt{List: []goast.Stmt{failure(label + " expects a non-null " + target.ClassName + " receiver")}}},
	)
	checks, callArguments := decoratorCheckedArguments(label, target.MethodParameters, target.MethodVariadic, arguments, failure)
	body.List = append(body.List, checks...)
	call := &goast.CallExpr{Fun: &goast.SelectorExpr{X: actualReceiver, Sel: goast.NewIdent(goName(target.RuntimeMethodName))}, Args: callArguments}
	if target.MethodVariadic {
		call.Ellipsis = token.Pos(1)
	}
	resultType := *target.MethodResult
	fallible := resultType.Name == "Result" && len(resultType.GenericArguments) == 1
	if fallible {
		resultType = resultType.GenericArguments[0]
	}
	void := resultType.Name == "void"
	result := goast.NewIdent("result")
	if fallible {
		err := goast.NewIdent("callError")
		if void {
			body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{err}, Tok: token.DEFINE, Rhs: []goast.Expr{call}})
		} else {
			body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{result, err}, Tok: token.DEFINE, Rhs: []goast.Expr{call}})
		}
		body.List = append(body.List, &goast.IfStmt{Cond: &goast.BinaryExpr{X: err, Op: token.NEQ, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.ReturnStmt{Results: []goast.Expr{&goast.CompositeLit{Type: valueType}, err}},
		}}})
	} else if void {
		body.List = append(body.List, &goast.ExprStmt{X: call})
	} else {
		body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{result}, Tok: token.DEFINE, Rhs: []goast.Expr{call}})
	}
	wrapped := &goast.CompositeLit{Type: valueType, Elts: []goast.Expr{
		&goast.KeyValueExpr{Key: goast.NewIdent("TypeIdentity"), Value: stringLiteral(target.MethodResultIdentity)},
	}}
	if !void {
		wrapped.Elts = append(wrapped.Elts, &goast.KeyValueExpr{Key: goast.NewIdent("value"), Value: result})
	}
	body.List = append(body.List, &goast.ReturnStmt{Results: []goast.Expr{wrapped, goast.NewIdent("nil")}})
	return &goast.FuncLit{Type: functionType, Body: body}
}
