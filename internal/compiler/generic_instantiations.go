package compiler

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/codegen"
)

func (r *Result) checkInstantiationCycles() error {
	if len(r.Diagnostics) != 0 || !hasGenericDeclarations(r.Program) {
		return nil
	}
	diagnostics, err := codegen.CheckInstantiationCycles(r.Program, r.goImporter, r.goSizes)
	r.Diagnostics = append(r.Diagnostics, diagnostics...)
	return err
}

func hasGenericDeclarations(program *ast.Program) bool {
	for _, declaration := range program.Declarations {
		var parameters []ast.TypeParameter
		var methods []*ast.MethodDecl
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			parameters = declaration.TypeParameters
		case *ast.MethodDecl:
			parameters = declaration.TypeParameters
		case *ast.ClassDecl:
			parameters, methods = declaration.TypeParameters, declaration.Methods
		case *ast.StructDecl:
			parameters, methods = declaration.TypeParameters, declaration.Methods
		case *ast.InterfaceDecl:
			parameters = declaration.TypeParameters
		case *ast.TypeDecl:
			parameters = declaration.TypeParameters
		}
		if len(parameters) != 0 {
			return true
		}
		for _, method := range methods {
			if len(method.TypeParameters) != 0 {
				return true
			}
		}
	}
	return false
}
