package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateCoercedMultiAssignment(stmt *ast.MultiAssignmentStmt, targets []goast.Expr, producer goast.Expr) (goast.Stmt, error) {
	used := map[string]bool{}
	reserve := func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			used[name.Name] = true
		}
		return true
	}
	goast.Inspect(producer, reserve)
	for _, target := range targets {
		goast.Inspect(target, reserve)
	}
	// Reserve the names used by conversion helpers and their type arguments too.
	for _, upcast := range stmt.Upcasts {
		if upcast != nil {
			generated, err := generateExpression(upcast)
			if err != nil {
				return nil, err
			}
			goast.Inspect(generated, reserve)
		}
	}
	bindings := make([]goast.Expr, len(targets))
	values := make([]goast.Expr, len(targets))
	for i, target := range targets {
		if id, ok := target.(*goast.Ident); ok && id.Name == "_" {
			bindings[i], values[i] = goast.NewIdent("_"), nil
			continue
		}
		name := fmt.Sprintf("__assignmentValue%d", i)
		for n := 0; used[name]; n++ {
			name = fmt.Sprintf("__assignmentValue%d_%d", i, n)
		}
		used[name] = true
		bindings[i] = goast.NewIdent(name)
		values[i] = bindings[i]
		if upcast := stmt.Upcasts[i]; upcast != nil {
			copy := *upcast
			copy.Value = &ast.IdentifierExpr{Name: name, Span: stmt.Span}
			var err error
			values[i], err = generateExpression(&copy)
			if err != nil {
				return nil, err
			}
		}
	}
	var lhs, rhs []goast.Expr
	for i, value := range values {
		if value != nil {
			lhs = append(lhs, targets[i])
			rhs = append(rhs, value)
		}
	}
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: bindings, Tok: token.DEFINE, Rhs: []goast.Expr{producer}},
		&goast.AssignStmt{Lhs: lhs, Tok: token.ASSIGN, Rhs: rhs},
	}}
	// A closure also works as a for-post statement, where a block is invalid.
	return &goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: body}}}, nil
}
