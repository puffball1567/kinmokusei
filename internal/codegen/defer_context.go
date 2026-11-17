package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
)

// Infer the context from the saved callee, instead of spelling its parameter
// types. This also works when a Go API accepts an unexported numeric type.
func contextualDeferArgument(signature *goast.FuncType, arguments, index int, expanded bool, callee, value goast.Expr, helpers map[string]*goast.FuncDecl) goast.Expr {
	parameters := len(signature.Params.List)
	variadic := false
	if parameters != 0 {
		_, variadic = signature.Params.List[parameters-1].Type.(*goast.Ellipsis)
	}
	// Hidden native contracts/receivers precede ordinary source arguments.
	if !variadic {
		parameters = arguments
	}
	results := 0
	if signature.Results != nil {
		results = len(signature.Results.List)
	}
	name := fmt.Sprintf("__kinmokuseiDeferContext_%d_%d_%d_%t_%t", parameters, results, index, variadic, expanded)
	if helpers[name] == nil {
		var typeParameters []*goast.Field
		var params, returns []*goast.Field
		for i := 0; i < parameters+results; i++ {
			typeName := goast.NewIdent(fmt.Sprintf("T%d", i))
			typeParameters = append(typeParameters, &goast.Field{Names: []*goast.Ident{typeName}, Type: goast.NewIdent("any")})
			if i < parameters {
				var parameterType goast.Expr = typeName
				if variadic && i == parameters-1 {
					parameterType = &goast.Ellipsis{Elt: typeName}
				}
				params = append(params, &goast.Field{Type: parameterType})
			} else {
				returns = append(returns, &goast.Field{Type: typeName})
			}
		}
		parameterIndex := index
		if variadic && parameterIndex >= parameters-1 {
			parameterIndex = parameters - 1
		}
		var valueType goast.Expr = goast.NewIdent(fmt.Sprintf("T%d", parameterIndex))
		if variadic && expanded && index == arguments-1 {
			valueType = &goast.ArrayType{Elt: valueType}
		}
		fnType := &goast.FuncType{Params: &goast.FieldList{List: params}}
		if results != 0 {
			fnType.Results = &goast.FieldList{List: returns}
		}
		helpers[name] = &goast.FuncDecl{Name: goast.NewIdent(name), Type: &goast.FuncType{
			TypeParams: &goast.FieldList{List: typeParameters}, Params: &goast.FieldList{List: []*goast.Field{
				{Names: []*goast.Ident{goast.NewIdent("_")}, Type: fnType},
				{Names: []*goast.Ident{goast.NewIdent("value")}, Type: valueType},
			}}, Results: &goast.FieldList{List: []*goast.Field{{Type: valueType}}},
		}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("value")}}}}}
	}
	return &goast.CallExpr{Fun: goast.NewIdent(name), Args: []goast.Expr{callee, value}}
}

func genericDeferThunk(signature *goast.FuncType, call *goast.CallExpr, helpers map[string]*goast.FuncDecl) goast.Expr {
	parameters, results := len(signature.Params.List), 0
	if signature.Results != nil {
		results = len(signature.Results.List)
	}
	variadic := false
	if parameters > 0 {
		_, variadic = signature.Params.List[parameters-1].Type.(*goast.Ellipsis)
	}
	name := fmt.Sprintf("__kinmokuseiDeferGeneric_%d_%d_%t", parameters, results, variadic)
	if helpers[name] == nil {
		var typeParameters, functionParameters, functionResults, helperParameters []*goast.Field
		var arguments []goast.Expr
		for i := 0; i < parameters+results; i++ {
			typeName := goast.NewIdent(fmt.Sprintf("T%d", i))
			typeParameters = append(typeParameters, &goast.Field{Names: []*goast.Ident{typeName}, Type: goast.NewIdent("any")})
			if i < parameters {
				var parameterType goast.Expr = typeName
				if variadic && i == parameters-1 {
					parameterType = &goast.Ellipsis{Elt: typeName}
				}
				argument := goast.NewIdent(fmt.Sprintf("arg%d", i))
				functionParameters = append(functionParameters, &goast.Field{Type: parameterType})
				helperParameters = append(helperParameters, &goast.Field{Names: []*goast.Ident{argument}, Type: parameterType})
				arguments = append(arguments, argument)
			} else {
				functionResults = append(functionResults, &goast.Field{Type: typeName})
			}
		}
		fnType := &goast.FuncType{Params: &goast.FieldList{List: functionParameters}}
		if results != 0 {
			fnType.Results = &goast.FieldList{List: functionResults}
		}
		thunkType := &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{{Type: &goast.FuncType{Params: &goast.FieldList{}}}}}}
		next := goast.NewIdent("next")
		thunkSignature := &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{next}, Type: &goast.FuncType{Params: &goast.FieldList{}}}}}}
		invocation := &goast.CallExpr{Fun: goast.NewIdent("fn"), Args: arguments}
		if variadic {
			invocation.Ellipsis = token.Pos(1)
		}
		thunk := &goast.FuncLit{Type: thunkSignature, Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.DeferStmt{Call: invocation}, &goast.ExprStmt{X: &goast.CallExpr{Fun: next}},
		}}}
		helpers[name] = &goast.FuncDecl{Name: goast.NewIdent(name), Type: &goast.FuncType{
			TypeParams: &goast.FieldList{List: typeParameters},
			Params:     &goast.FieldList{List: append([]*goast.Field{{Names: []*goast.Ident{goast.NewIdent("fn")}, Type: fnType}}, helperParameters...)},
			Results:    &goast.FieldList{List: []*goast.Field{{Type: thunkType}}},
		}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{thunk}}}}}
		if len(typeParameters) == 0 {
			helpers[name].Type.TypeParams = nil
		}
	}
	return &goast.CallExpr{Fun: goast.NewIdent(name), Args: append([]goast.Expr{call.Fun}, call.Args...), Ellipsis: call.Ellipsis}
}
