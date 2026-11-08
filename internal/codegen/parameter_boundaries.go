package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"path/filepath"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Check existing parameter variables directly: no helper signature needs to
// spell a private Go type and no argument expression is evaluated twice.
func prependParameterChecks(body *goast.BlockStmt, parameters []ast.Parameter) {
	var checks []goast.Stmt
	for _, parameter := range parameters {
		if !parameter.NonNull {
			continue
		}
		name := goast.NewIdent(goName(parameter.Name))
		value := name
		if parameter.Variadic {
			value = goast.NewIdent("value")
		}
		message := fmt.Sprintf("non-null parameter %q failed at %s:%d:%d", parameter.Name,
			filepath.Base(parameter.Span.Path), parameter.Span.Start.Line, parameter.Span.Start.Column)
		var check goast.Stmt = &goast.IfStmt{
			Cond: &goast.BinaryExpr{X: value, Op: token.EQL, Y: goast.NewIdent("nil")},
			Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ExprStmt{X: &goast.CallExpr{
				Fun: goast.NewIdent("panic"), Args: []goast.Expr{stringLiteral(message)},
			}}}},
		}
		if parameter.Variadic {
			check = &goast.RangeStmt{Key: goast.NewIdent("_"), Value: value, Tok: token.DEFINE,
				X: name, Body: &goast.BlockStmt{List: []goast.Stmt{check}}}
		}
		checks = append(checks, check)
	}
	body.List = append(checks, body.List...)
}

func generateCallableBlock(block *ast.BlockStmt, parameters []ast.Parameter) (*goast.BlockStmt, error) {
	body, err := generateBlock(block)
	if err != nil {
		return nil, err
	}
	prependParameterChecks(body, parameters)
	return body, nil
}
