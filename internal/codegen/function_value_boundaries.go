package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Capture the bound value once. Its concrete wrapper enforces source contracts
// even when Go invokes it without a source call site or mutates its providers.
func generateFunctionValueBoundary(signature ast.TypeRef, boundary *ast.FunctionValueBoundary, span source.Span, value goast.Expr) (goast.Expr, error) {
	if boundary == nil {
		return value, nil
	}
	functionType, ok := goType(signature).(*goast.FuncType)
	if !ok || len(functionType.Params.List) != len(boundary.Parameters) {
		return nil, fmt.Errorf("function value boundary does not match the function signature")
	}
	function := goast.NewIdent("__kinmokuseiBoundFunction")
	arguments := make([]goast.Expr, len(boundary.Parameters))
	for i, parameter := range boundary.Parameters {
		name := goast.NewIdent(parameter.Name)
		functionType.Params.List[i].Names = []*goast.Ident{name}
		arguments[i] = name
	}
	call := &goast.CallExpr{Fun: function, Args: arguments}
	if len(boundary.Parameters) != 0 && boundary.Parameters[len(boundary.Parameters)-1].Variadic {
		call.Ellipsis = token.Pos(1)
	}
	body := &goast.BlockStmt{}
	if len(boundary.Results) == 0 {
		body.List = []goast.Stmt{&goast.ExprStmt{X: call}, &goast.ReturnStmt{}}
	} else {
		results := make([]goast.Expr, len(boundary.Results))
		for i := range results {
			results[i] = goast.NewIdent(fmt.Sprintf("__kinmokuseiValueResult%d", i))
		}
		body.List = append(body.List, &goast.AssignStmt{Lhs: results, Tok: token.DEFINE, Rhs: []goast.Expr{call}})
		for i, target := range boundary.Results {
			if target != nil {
				success := referenceSuccess(boundary.Condition, results[len(results)-1])
				body.List = append(body.List, generateTypedNonNullCheck(results[i], *target, span, fmt.Sprintf("non-null function result %d", i+1), success))
			}
		}
		body.List = append(body.List, &goast.ReturnStmt{Results: results})
	}
	prependParameterChecks(body, boundary.Parameters)
	// The capture's parameter type needs no names and must remain independent
	// of the inner function's parameter declarations.
	capture := &goast.FuncLit{
		Type: &goast.FuncType{
			Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{function}, Type: goType(signature)}}},
			Results: &goast.FieldList{List: []*goast.Field{{Type: goType(signature)}}},
		},
		Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{&goast.FuncLit{Type: functionType, Body: body}}}}},
	}
	return &goast.CallExpr{Fun: capture, Args: []goast.Expr{value}}, nil
}
