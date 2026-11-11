package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Capture the producer once with inferred Go types, including private types.
// Typed checks perform interface boxing before testing nil, just like a scalar
// storage boundary. No result is published if any reference check fails.
func generateNonNullResults(expr *ast.UnaryExpr, producer goast.Expr) (goast.Expr, error) {
	return generateNonNullResultsWhen(expr, producer, ast.NonNullAlways)
}

func generateNonNullResultsWhen(expr *ast.UnaryExpr, producer goast.Expr, condition ast.NonNullCondition) (goast.Expr, error) {
	if len(expr.AssertionType.GoResults) != len(expr.NonNullResults) {
		return nil, fmt.Errorf("non-null result checks do not match the result list")
	}
	fnType := &goast.FuncType{Params: &goast.FieldList{}, Results: functionResults(expr.AssertionType)}
	used := map[string]bool{}
	reserve := func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			used[name.Name] = true
		}
		return true
	}
	goast.Inspect(producer, reserve)
	goast.Inspect(fnType, reserve)
	bindings := make([]goast.Expr, len(expr.NonNullResults))
	values := make([]goast.Expr, len(bindings))
	for i := range expr.NonNullResults {
		name := fmt.Sprintf("__kinmokuseiResult%d", i)
		for used[name] {
			name += "_"
		}
		used[name] = true
		bindings[i] = goast.NewIdent(name)
		values[i] = bindings[i]
	}
	// A success condition uses the final capture even when its source binding
	// is blank. Populate every capture before constructing the checks.
	for i, checked := range expr.NonNullResults {
		if checked {
			success := referenceSuccess(condition, bindings[len(bindings)-1])
			values[i] = generateNonNullReferenceWhen(values[i], expr.AssertionType.GoResults[i], expr.AssertionSpan, fmt.Sprintf("non-null result %d", i+1), success)
		}
	}
	return &goast.CallExpr{Fun: &goast.FuncLit{Type: fnType, Body: &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: bindings, Tok: token.DEFINE, Rhs: []goast.Expr{producer}},
		&goast.ReturnStmt{Results: values},
	}}}}, nil
}
