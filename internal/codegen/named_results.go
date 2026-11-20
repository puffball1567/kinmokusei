package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func namedResultIdentifier(binding ast.ResultBinding, index int) *goast.Ident {
	if binding.Name == "_" {
		return goast.NewIdent("__kinmokuseiBlankResult" + strconv.Itoa(index))
	}
	return goast.NewIdent(goName(binding.Name))
}

func namedResultSlot(index int) *goast.Ident {
	return goast.NewIdent("__kinmokuseiResultSlot" + strconv.Itoa(index))
}

// A try closure may contain a local with the same source name as a result.
// Capture the actual result variables before entering any nested scope.
func prependNamedResultSlots(body *goast.BlockStmt, result ast.TypeRef) {
	if !result.NamedResultsThroughTry {
		return
	}
	var slots []goast.Stmt
	for i, binding := range result.NamedResults {
		slots = append(slots, &goast.AssignStmt{Lhs: []goast.Expr{namedResultSlot(i)}, Tok: token.DEFINE,
			Rhs: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: namedResultIdentifier(binding, i)}}})
	}
	body.List = append(slots, body.List...)
}

func namedResultSlots(count int) []goast.Expr {
	slots := make([]goast.Expr, count)
	for i := range slots {
		slots[i] = &goast.StarExpr{X: namedResultSlot(i)}
	}
	return slots
}

func generateNamedExceptionReturn(stmt *ast.ReturnStmt) (goast.Stmt, error) {
	if stmt.Value == nil {
		return exceptionReturnPanic(nil, nil), nil
	}
	plain := *stmt
	plain.CrossesTry = false
	generated, err := generateStatement(&plain)
	if err != nil {
		return nil, err
	}
	return &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: namedResultSlots(stmt.NamedResultCount), Tok: token.ASSIGN, Rhs: generated.(*goast.ReturnStmt).Results},
		exceptionReturnPanic(nil, nil),
	}}, nil
}
