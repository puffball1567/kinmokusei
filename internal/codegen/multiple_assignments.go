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
	for _, boundary := range stmt.NonNullResults {
		if boundary != nil {
			goast.Inspect(goType(*boundary), reserve)
		}
	}
	bindings := make([]goast.Expr, len(targets))
	values := make([]goast.Expr, len(targets))
	keepFlag := hasReferenceBoundaries(stmt.NonNullResults) && stmt.NonNullCondition != ast.NonNullAlways
	for i, target := range targets {
		if id, ok := target.(*goast.Ident); ok && id.Name == "_" {
			if !keepFlag || i != len(targets)-1 {
				bindings[i], values[i] = goast.NewIdent("_"), nil
				continue
			}
		}
		name := fmt.Sprintf("__assignmentValue%d", i)
		for n := 0; used[name]; n++ {
			name = fmt.Sprintf("__assignmentValue%d_%d", i, n)
		}
		used[name] = true
		bindings[i] = goast.NewIdent(name)
		values[i] = bindings[i]
		if i < len(stmt.Upcasts) && stmt.Upcasts[i] != nil {
			upcast := stmt.Upcasts[i]
			copy := *upcast
			copy.Value = &ast.IdentifierExpr{Name: name, Span: stmt.Span}
			var err error
			values[i], err = generateExpression(&copy)
			if err != nil {
				return nil, err
			}
		}
	}
	// Conditions use the captured producer flag, including when its destination
	// is blank. Evaluate all validation expressions before the parallel writes.
	for i, boundary := range stmt.NonNullResults {
		if boundary != nil {
			success := referenceSuccess(stmt.NonNullCondition, bindings[len(bindings)-1])
			values[i] = generateNonNullReferenceWhen(values[i], *boundary, boundary.Span, "non-null storage boundary", success)
		}
	}
	var lhs, rhs []goast.Expr
	for i, value := range values {
		if id, ok := targets[i].(*goast.Ident); ok && id.Name == "_" {
			continue
		}
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
