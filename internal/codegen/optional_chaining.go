package codegen

import (
	"fmt"
	"github.com/puffball1567/kinmokusei/internal/ast"
	goast "go/ast"
	"go/token"
	"path/filepath"
)

func generateOptionalChain(expr *ast.OptionalChainExpr) (goast.Expr, error) {
	object, err := generateExpression(expr.Object)
	if err != nil {
		return nil, err
	}
	value, err := generateExpression(expr.Value)
	if err != nil {
		return nil, err
	}
	if expr.Receiver == nil {
		return nil, fmt.Errorf("optional chain has no receiver")
	}
	resultType := goType(expr.ResultType)
	void := expr.ResultType.Name == "void"
	results := &goast.FieldList{}
	if !void {
		results.List = append(results.List, &goast.Field{Type: resultType})
	}
	if expr.Checked {
		results.List = append(results.List, &goast.Field{Type: goast.NewIdent("bool")})
	}
	used := map[string]bool{}
	reserve := func(node goast.Node) bool {
		if id, ok := node.(*goast.Ident); ok {
			used[id.Name] = true
		}
		return true
	}
	goast.Inspect(value, reserve)
	goast.Inspect(results, reserve)
	name := "__kinmokuseiOptionalReceiver"
	for used[name] {
		name += "_"
	}
	zero := "__kinmokuseiOptionalZero"
	for used[zero] {
		zero += "_"
	}
	goast.Inspect(value, func(node goast.Node) bool {
		if id, ok := node.(*goast.Ident); ok && id.Name == expr.Receiver.Marker {
			id.Name = name
		}
		return true
	})
	var absent, present []goast.Stmt
	if void {
		absent = []goast.Stmt{&goast.ReturnStmt{}}
		present = []goast.Stmt{&goast.ExprStmt{X: value}}
	} else if expr.Checked {
		absent = []goast.Stmt{
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{Names: []*goast.Ident{goast.NewIdent(zero)}, Type: resultType}}}},
			&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent(zero), goast.NewIdent("false")}},
		}
		values := []goast.Expr{value}
		if !expr.ForwardChecked {
			values = append(values, goast.NewIdent("true"))
		}
		present = []goast.Stmt{&goast.ReturnStmt{Results: values}}
	} else {
		absent = []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("nil")}}}
		present = []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{value}}}
	}
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(name)}, Tok: token.DEFINE, Rhs: []goast.Expr{object}},
		&goast.IfStmt{Cond: &goast.BinaryExpr{X: goast.NewIdent(name), Op: token.EQL, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: absent}},
	}}
	body.List = append(body.List, present...)
	return &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}, Results: results}, Body: body}}, nil
}

func generateOptionalAssertion(expr *ast.UnaryExpr, operand goast.Expr) goast.Expr {
	value, present := goast.NewIdent("value"), goast.NewIdent("present")
	location := expr.AssertionSpan
	message := fmt.Sprintf("non-null assertion failed at %s:%d:%d", filepath.Base(location.Path), location.Start.Line, location.Start.Column)
	return &goast.CallExpr{Fun: &goast.FuncLit{
		Type: &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{
			{Names: []*goast.Ident{value}, Type: goType(expr.AssertionType)},
			{Names: []*goast.Ident{present}, Type: goast.NewIdent("bool")},
		}}, Results: &goast.FieldList{List: []*goast.Field{{Type: goType(expr.AssertionType)}}}},
		Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.IfStmt{Cond: &goast.UnaryExpr{Op: token.NOT, X: present}, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{stringLiteral(message)}}},
			}}},
			&goast.ReturnStmt{Results: []goast.Expr{value}},
		}},
	}, Args: []goast.Expr{operand}}
}
