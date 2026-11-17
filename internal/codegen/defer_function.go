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
	original := &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}, Results: signature.Results}, Body: &goast.BlockStmt{List: body.List}}}
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
