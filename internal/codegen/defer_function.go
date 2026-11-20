package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
)

func wrapDeferredCallable(signature *goast.FuncType, body *goast.BlockStmt) {
	// A supervisor distinguishes panic(nil) (including panicnil=1) from normal
	// completion. A separate outer defer handles Goexit, which never resumes
	// after the supervisor; Goexit must not be converted into a nil panic.
	wrapper, err := parseDeferSupport(`package p
func wrapper() {
 var __kinmokuseiDefers __kinmokuseiDeferFrame
 __kinmokuseiDeferResumed := false
 defer func() {
  if !__kinmokuseiDeferResumed {
   __kinmokuseiDefers.panicked = false
   __kinmokuseiDefers.run()
  }
 }()
 func() {
  __kinmokuseiDeferCompleted := false
  defer func() {
   __kinmokuseiDefers.value = recover()
   __kinmokuseiDefers.panicked = !__kinmokuseiDeferCompleted
  }()
  BODY()
  __kinmokuseiDeferCompleted = true
 }()
 __kinmokuseiDeferResumed = true
 __kinmokuseiDefers.run()
}
`)
	if err != nil {
		panic(err) // Static compiler-owned syntax, also parsed by runtime tests.
	}
	outer := wrapper[0].(*goast.FuncDecl).Body
	supervisor := outer.List[3].(*goast.ExprStmt).X.(*goast.CallExpr).Fun.(*goast.FuncLit).Body
	originalBody := &goast.BlockStmt{List: body.List}
	originalType := &goast.FuncType{Params: &goast.FieldList{}, Results: signature.Results}
	original := &goast.CallExpr{Fun: &goast.FuncLit{Type: originalType, Body: originalBody}}
	if signature.Results != nil && len(signature.Results.List) != 0 && len(signature.Results.List[0].Names) != 0 {
		originalType.Results = nil
		slots := lowerNamedDeferredReturns(signature.Results, originalBody)
		supervisor.List[2] = &goast.ExprStmt{X: original}
		body.List = append(slots, outer.List...)
		body.List = append(body.List, &goast.ReturnStmt{})
		return
	}
	var results []goast.Expr
	if signature.Results != nil {
		for _, field := range signature.Results.List {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for i := 0; i < count; i++ {
				name := goast.NewIdent(fmt.Sprintf("__kinmokuseiDeferResult%d", len(results)))
				results = append(results, name)
				outer.List = append([]goast.Stmt{&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{
					&goast.ValueSpec{Names: []*goast.Ident{name}, Type: field.Type},
				}}}}, outer.List...)
			}
		}
	}
	if len(results) == 0 {
		supervisor.List[2] = &goast.ExprStmt{X: original}
	} else {
		supervisor.List[2] = &goast.AssignStmt{Lhs: results, Tok: token.ASSIGN, Rhs: []goast.Expr{original}}
		outer.List = append(outer.List, &goast.ReturnStmt{Results: results})
	}
	body.List = outer.List
}

// Write to the outer result variables. They remain accessible to deferred
// closures even if the protected call panics. Infer the pointer types here so
// a result name that shadows a Go type does not break the inner signature.
func lowerNamedDeferredReturns(fields *goast.FieldList, body *goast.BlockStmt) []goast.Stmt {
	var captures []goast.Stmt
	var slots []goast.Expr
	for _, field := range fields.List {
		for _, name := range field.Names {
			pointer := goast.NewIdent(fmt.Sprintf("__kinmokuseiDeferSlot%d", len(slots)))
			captures = append(captures, &goast.AssignStmt{Lhs: []goast.Expr{pointer}, Tok: token.DEFINE,
				Rhs: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: goast.NewIdent(name.Name)}}})
			slots = append(slots, &goast.StarExpr{X: pointer})
		}
	}
	used := false
	replaceGoStatements(body, false, func(stmt goast.Stmt) goast.Stmt {
		returned, ok := stmt.(*goast.ReturnStmt)
		if !ok || len(returned.Results) == 0 {
			return stmt
		}
		used = true
		return &goast.BlockStmt{List: []goast.Stmt{
			&goast.AssignStmt{Lhs: slots, Tok: token.ASSIGN, Rhs: returned.Results},
			&goast.ReturnStmt{Return: returned.Return},
		}}
	})
	if !used {
		return nil
	}
	return captures
}
