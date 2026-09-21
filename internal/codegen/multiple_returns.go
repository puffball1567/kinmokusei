package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// The typed payload preserves nil interfaces, narrow numeric types and generic
// identities while the existing return control unwinds through finally blocks.
func multipleReturnPayloadType(result ast.TypeRef) *goast.StructType {
	fields := make([]*goast.Field, len(result.GoResults))
	for i, value := range result.GoResults {
		fields[i] = &goast.Field{Names: []*goast.Ident{goast.NewIdent(fmt.Sprintf("v%d", i))}, Type: goType(value)}
	}
	return &goast.StructType{Fields: &goast.FieldList{List: fields}}
}

func generateMultipleExceptionReturn(stmt *ast.ReturnStmt) (goast.Stmt, error) {
	// Use a typed return context before packaging. This performs ordinary Go
	// conversions and evaluates an explicit list or forwarded call exactly once.
	plain := *stmt
	plain.CrossesTry = false
	returned, err := generateStatement(&plain)
	if err != nil {
		return nil, err
	}
	call := &goast.CallExpr{Fun: &goast.FuncLit{
		Type: &goast.FuncType{Params: &goast.FieldList{}, Results: functionResults(stmt.ResultType)},
		Body: &goast.BlockStmt{List: []goast.Stmt{returned}},
	}}
	names := make([]goast.Expr, len(stmt.ResultType.GoResults))
	for i := range names {
		names[i] = goast.NewIdent(fmt.Sprintf("__kinmokusei_result_%d", i))
	}
	payload := &goast.CompositeLit{Type: multipleReturnPayloadType(stmt.ResultType), Elts: names}
	control := &goast.CompositeLit{Type: goast.NewIdent("__kinmokuseiReturn"), Elts: []goast.Expr{
		&goast.KeyValueExpr{Key: goast.NewIdent("value"), Value: payload},
	}}
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: names, Tok: token.DEFINE, Rhs: []goast.Expr{call}},
		&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{control}}},
	}}
	return &goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: body}}}, nil
}
