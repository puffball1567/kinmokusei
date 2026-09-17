package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateLocalArrowStorage(variable *ast.VariableDecl) (goast.Stmt, error) {
	if _, ok := variable.Value.(*ast.ArrowExpr); !ok || !variable.ResolvedType.IsSpecified() {
		return nil, fmt.Errorf("recursive arrow %q is missing a checked function initializer or type", variable.Name)
	}
	return &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
		Names: []*goast.Ident{goast.NewIdent(goName(variable.Name))}, Type: goType(variable.ResolvedType),
	}}}}, nil
}
