package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"strconv"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func generateTryStatement(stmt *kinmokuseiAST.TryStmt) (goast.Stmt, error) {
	tryBody, err := generateBlock(stmt.Body)
	if err != nil {
		return nil, err
	}
	outerBody := &goast.BlockStmt{}
	if stmt.FinallyBody != nil {
		finallyBody, err := generateBlock(stmt.FinallyBody)
		if err != nil {
			return nil, err
		}
		outerBody.List = append(outerBody.List, &goast.DeferStmt{Call: &goast.CallExpr{Fun: &goast.FuncLit{
			Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: finallyBody,
		}}})
	}
	if len(stmt.Catches) == 0 {
		outerBody.List = append(outerBody.List, tryBody.List...)
	} else {
		offset := stmt.Span.Start.Offset
		caughtName := fmt.Sprintf("__kinmokusei_caught_%d", offset)
		handledName := fmt.Sprintf("__kinmokusei_handled_%d", offset)
		errorName := fmt.Sprintf("__kinmokusei_caught_error_%d", offset)
		recoveredName := fmt.Sprintf("__kinmokusei_recovered_%d", offset)
		thrownName := fmt.Sprintf("__kinmokusei_thrown_%d", offset)
		candidateName := fmt.Sprintf("__kinmokusei_candidate_%d", offset)
		okName := fmt.Sprintf("__kinmokusei_thrown_ok_%d", offset)

		outerBody.List = append(outerBody.List,
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
				Names: []*goast.Ident{goast.NewIdent(caughtName)}, Type: goast.NewIdent("bool"),
			}}}},
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
				Names: []*goast.Ident{goast.NewIdent(errorName)}, Type: goast.NewIdent("error"),
			}}}},
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
				Names: []*goast.Ident{goast.NewIdent(thrownName)}, Type: goast.NewIdent("__kinmokuseiException"),
			}}}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent(errorName)}},
		)

		recoveryBody := &goast.BlockStmt{List: []goast.Stmt{
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(recoveredName)}, Tok: token.DEFINE, Rhs: []goast.Expr{
				&goast.CallExpr{Fun: goast.NewIdent("recover")},
			}},
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: goast.NewIdent(recoveredName), Op: token.EQL, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ReturnStmt{},
			}}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(candidateName), goast.NewIdent(okName)}, Tok: token.DEFINE, Rhs: []goast.Expr{
				&goast.TypeAssertExpr{X: goast.NewIdent(recoveredName), Type: goast.NewIdent("__kinmokuseiException")},
			}},
			&goast.IfStmt{Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent(okName)}, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{goast.NewIdent(recoveredName)}}},
			}}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(thrownName)}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent(candidateName)}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(caughtName)}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent("true")}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(errorName)}, Tok: token.ASSIGN, Rhs: []goast.Expr{
				&goast.CallExpr{Fun: &goast.SelectorExpr{X: goast.NewIdent(candidateName), Sel: goast.NewIdent("KinmokuseiExceptionError")}},
			}},
		}}
		tryBody.List = append([]goast.Stmt{&goast.DeferStmt{Call: &goast.CallExpr{Fun: &goast.FuncLit{
			Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: recoveryBody,
		}}}}, tryBody.List...)
		outerBody.List = append(outerBody.List, &goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{
			Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: tryBody,
		}}})

		dispatchBody := &goast.BlockStmt{List: []goast.Stmt{
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
				Names: []*goast.Ident{goast.NewIdent(handledName)}, Type: goast.NewIdent("bool"),
			}}}},
		}}
		for index, clause := range stmt.Catches {
			if clause.Type.Qualifier == "" && clause.Type.Name == "error" {
				catchBody, err := generateTypedCatchBody(clause, handledName, goast.NewIdent("error"), goast.NewIdent(errorName))
				if err != nil {
					return nil, err
				}
				dispatchBody.List = append(dispatchBody.List, &goast.IfStmt{
					Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent(handledName)}, Body: catchBody,
				})
				continue
			}
			if len(clause.MatchingClasses) != 0 {
				for matchIndex, matchingClass := range clause.MatchingClasses {
					typedName := fmt.Sprintf("__kinmokusei_typed_catch_%d_%d_%d", offset, index, matchIndex)
					typedOKName := fmt.Sprintf("__kinmokusei_typed_catch_ok_%d_%d_%d", offset, index, matchIndex)
					typedTarget := goast.Expr(goast.NewIdent(typedName))
					if clause.Name == "_" || !clause.Used {
						typedTarget = goast.NewIdent("_")
					}
					bindingValue := goast.Expr(goast.NewIdent(typedName))
					if matchingClass != clause.Type.Name {
						bindingValue = &goast.CallExpr{Fun: goast.NewIdent(upcastName(matchingClass, clause.Type.Name)), Args: []goast.Expr{goast.NewIdent(typedName)}}
					}
					catchBody, err := generateTypedCatchBody(clause, handledName, goType(clause.Type), bindingValue)
					if err != nil {
						return nil, err
					}
					typeMatch := &goast.IfStmt{
						Init: &goast.AssignStmt{Lhs: []goast.Expr{typedTarget, goast.NewIdent(typedOKName)}, Tok: token.DEFINE, Rhs: []goast.Expr{
							&goast.TypeAssertExpr{X: goast.NewIdent(errorName), Type: &goast.StarExpr{X: goast.NewIdent(matchingClass)}},
						}},
						Cond: goast.NewIdent(typedOKName), Body: catchBody,
					}
					dispatchBody.List = append(dispatchBody.List, &goast.IfStmt{
						Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent(handledName)}, Body: &goast.BlockStmt{List: []goast.Stmt{typeMatch}},
					})
				}
				if clause.Type.Name == "Exception" {
					fallback := &goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiExceptionFromError"), Args: []goast.Expr{goast.NewIdent(errorName)}}
					catchBody, err := generateTypedCatchBody(clause, handledName, goType(clause.Type), fallback)
					if err != nil {
						return nil, err
					}
					dispatchBody.List = append(dispatchBody.List, &goast.IfStmt{
						Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent(handledName)}, Body: catchBody,
					})
				}
				continue
			}

			typedName := fmt.Sprintf("__kinmokusei_typed_catch_%d_%d", offset, index)
			typedOKName := fmt.Sprintf("__kinmokusei_typed_catch_ok_%d_%d", offset, index)
			typedTarget := goast.Expr(goast.NewIdent(typedName))
			if clause.Name == "_" || !clause.Used {
				typedTarget = goast.NewIdent("_")
			}
			catchBody, err := generateTypedCatchBody(clause, handledName, goType(clause.Type), goast.NewIdent(typedName))
			if err != nil {
				return nil, err
			}
			typeMatch := &goast.IfStmt{
				Init: &goast.AssignStmt{Lhs: []goast.Expr{typedTarget, goast.NewIdent(typedOKName)}, Tok: token.DEFINE, Rhs: []goast.Expr{
					&goast.TypeAssertExpr{X: goast.NewIdent(errorName), Type: goType(clause.Type)},
				}},
				Cond: goast.NewIdent(typedOKName), Body: catchBody,
			}
			dispatchBody.List = append(dispatchBody.List, &goast.IfStmt{
				Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent(handledName)}, Body: &goast.BlockStmt{List: []goast.Stmt{typeMatch}},
			})
		}
		dispatchBody.List = append(dispatchBody.List, &goast.IfStmt{
			Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent(handledName)}, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{goast.NewIdent(thrownName)}}},
			}},
		})
		outerBody.List = append(outerBody.List, &goast.IfStmt{Cond: goast.NewIdent(caughtName), Body: dispatchBody})
	}

	call := goast.Stmt(&goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: outerBody}}})
	if stmt.HandlesReturn {
		offset := stmt.Span.Start.Offset
		returningName := fmt.Sprintf("__kinmokusei_returning_%d", offset)
		controlName := fmt.Sprintf("__kinmokusei_return_%d", offset)
		recoveredName := fmt.Sprintf("__kinmokusei_return_recovered_%d", offset)
		candidateName := fmt.Sprintf("__kinmokusei_return_candidate_%d", offset)
		okName := fmt.Sprintf("__kinmokusei_return_ok_%d", offset)
		returnRecoveryBody := &goast.BlockStmt{List: []goast.Stmt{
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(recoveredName)}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.CallExpr{Fun: goast.NewIdent("recover")}}},
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: goast.NewIdent(recoveredName), Op: token.EQL, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{}}}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(candidateName), goast.NewIdent(okName)}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.TypeAssertExpr{X: goast.NewIdent(recoveredName), Type: goast.NewIdent("__kinmokuseiReturn")}}},
			&goast.IfStmt{Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent(okName)}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{goast.NewIdent(recoveredName)}}}}}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(controlName)}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent(candidateName)}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(returningName)}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent("true")}},
		}}
		wrapperBody := &goast.BlockStmt{List: []goast.Stmt{
			&goast.DeferStmt{Call: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: returnRecoveryBody}}},
			call,
		}}
		returnStatement := exceptionControlReturn(stmt.ReturnType, controlName)
		call = &goast.BlockStmt{List: []goast.Stmt{
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{Names: []*goast.Ident{goast.NewIdent(returningName)}, Type: goast.NewIdent("bool")}}}},
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{Names: []*goast.Ident{goast.NewIdent(controlName)}, Type: goast.NewIdent("__kinmokuseiReturn")}}}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent(controlName)}},
			&goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: wrapperBody}}},
			&goast.IfStmt{Cond: goast.NewIdent(returningName), Body: &goast.BlockStmt{List: []goast.Stmt{returnStatement}}},
		}}
	}
	if !stmt.Terminal {
		return call, nil
	}
	return &goast.BlockStmt{List: []goast.Stmt{call, &goast.ExprStmt{X: &goast.CallExpr{
		Fun: goast.NewIdent("panic"), Args: []goast.Expr{&goast.BasicLit{Kind: token.STRING, Value: strconv.Quote("unreachable after terminal Kinmokusei try")}},
	}}}}, nil
}

func generateTypedCatchBody(clause *kinmokuseiAST.CatchClause, handledName string, bindingType, bindingValue goast.Expr) (*goast.BlockStmt, error) {
	catchBody, err := generateBlock(clause.Body)
	if err != nil {
		return nil, err
	}
	catchBody.List = append([]goast.Stmt{&goast.AssignStmt{
		Lhs: []goast.Expr{goast.NewIdent(handledName)}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent("true")},
	}}, catchBody.List...)
	if clause.Name != "_" && clause.Used {
		binding := &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
			Names: []*goast.Ident{goast.NewIdent(goName(clause.Name))}, Type: bindingType, Values: []goast.Expr{bindingValue},
		}}}}
		catchBody.List = append([]goast.Stmt{binding}, catchBody.List...)
	}
	return catchBody, nil
}

func exceptionControlReturn(returnType kinmokuseiAST.TypeRef, controlName string) goast.Stmt {
	control := func(field string) goast.Expr {
		return &goast.SelectorExpr{X: goast.NewIdent(controlName), Sel: goast.NewIdent(field)}
	}
	if returnType.Name == "void" {
		return &goast.ReturnStmt{}
	}
	if returnType.Name == "Result" && len(returnType.GenericArguments) == 1 {
		element := returnType.GenericArguments[0]
		if element.Name == "void" {
			return &goast.ReturnStmt{Results: []goast.Expr{control("err")}}
		}
		return &goast.ReturnStmt{Results: []goast.Expr{exceptionReturnValue(element, control("value")), control("err")}}
	}
	return &goast.ReturnStmt{Results: []goast.Expr{exceptionReturnValue(returnType, control("value"))}}
}

func exceptionReturnValue(valueType kinmokuseiAST.TypeRef, value goast.Expr) goast.Expr {
	return &goast.CallExpr{Fun: &goast.IndexExpr{X: goast.NewIdent("__kinmokuseiReturnValue"), Index: goType(valueType)}, Args: []goast.Expr{value}}
}
