package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"strconv"
)

// Capture constants as constants, not default-typed variables: an untyped
// integer may need a uint64 context, and nil needs its original call context.
// Inferred temporaries retain even Go types that cannot be named by clients.
func captureSourceDefer(block *goast.BlockStmt, contexts map[string]*goast.FuncDecl) {
	marker := sourceMarker(block.List[0], sourceDeferName)
	mask, _ := strconv.Unquote(marker.Args[0].(*goast.BasicLit).Value)
	capture := marker.Args[1].(*goast.Ident).Name == "true"
	tuple, _ := strconv.Atoi(marker.Args[2].(*goast.BasicLit).Value)
	call := block.List[1].(*goast.DeferStmt).Call
	block.List = nil
	if signature, ok := marker.Args[3].(*goast.FuncType); ok && !capture && tuple == 0 {
		// Let Go infer a generic function value together with all arguments.
		// This preserves private inferred types and contextual runtime shifts.
		block.List = []goast.Stmt{appendDeferredThunk(genericDeferThunk(signature, call, contexts))}
		return
	}
	index := 0
	fresh := func() *goast.Ident {
		name := goast.NewIdent(fmt.Sprintf("__kinmokuseiDeferCapture%d", index))
		index++
		return name
	}
	if capture {
		name := fresh()
		block.List = append(block.List, &goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{call.Fun}})
		call.Fun = name
	}
	if tuple > 0 {
		values := make([]goast.Expr, tuple)
		for i := range values {
			values[i] = fresh()
		}
		block.List = append(block.List, &goast.AssignStmt{Lhs: values, Tok: token.DEFINE, Rhs: call.Args})
		call.Args = values
	} else {
		for i, argument := range call.Args {
			if name, ok := argument.(*goast.Ident); ok && name.Name == "nil" {
				continue
			}
			name := fresh()
			if builtin, ok := call.Fun.(*goast.Ident); ok && builtin.Name == "delete" && i == 1 && mask[i] != 'c' {
				argument = &goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiDeferMapKey"), Args: []goast.Expr{call.Args[0], argument}}
			}
			context := marker.Args[4+i]
			if identifier, nilContext := context.(*goast.Ident); !nilContext || identifier.Name != "nil" {
				if signature, ok := marker.Args[3].(*goast.FuncType); ok && capture {
					argument = contextualDeferArgument(signature, len(call.Args), i, call.Ellipsis.IsValid(), call.Fun, argument, contexts)
					block.List = append(block.List, &goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{argument}})
				} else {
					block.List = append(block.List, &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{
						&goast.ValueSpec{Names: []*goast.Ident{name}, Type: context, Values: []goast.Expr{argument}},
					}}})
				}
			} else if mask[i] == 'c' {
				block.List = append(block.List, &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.CONST, Specs: []goast.Spec{
					&goast.ValueSpec{Names: []*goast.Ident{name}, Values: []goast.Expr{argument}},
				}}})
			} else {
				block.List = append(block.List, &goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{argument}})
			}
			call.Args[i] = name
		}
	}
	next := goast.NewIdent("__kinmokuseiDeferNext")
	thunk := &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{{
		Names: []*goast.Ident{next}, Type: &goast.FuncType{Params: &goast.FieldList{}},
	}}}}, Body: &goast.BlockStmt{List: []goast.Stmt{
		&goast.DeferStmt{Call: call}, &goast.ExprStmt{X: &goast.CallExpr{Fun: next}},
	}}}
	block.List = append(block.List, appendDeferredThunk(thunk))
}

func appendDeferredThunk(thunk goast.Expr) goast.Stmt {
	calls := &goast.SelectorExpr{X: goast.NewIdent("__kinmokuseiDefers"), Sel: goast.NewIdent("calls")}
	return &goast.AssignStmt{Lhs: []goast.Expr{calls}, Tok: token.ASSIGN,
		Rhs: []goast.Expr{&goast.CallExpr{Fun: goast.NewIdent("append"), Args: []goast.Expr{calls, thunk}}}}
}
