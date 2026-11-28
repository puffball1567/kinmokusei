package sema

import (
	"fmt"
	goast "go/ast"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Go's export-data readers install bounds in declaration order. A comparable
// intersection can query the storage of a later parameter before its bound
// exists, even though the source checker can resolve the declarations.
func (c *Checker) checkGoExportBoundOrder(program *ast.Program) {
	exportedBodies := c.goExportBodyDependencies(program)
	for _, declaration := range program.Declarations {
		name, _ := ast.DeclarationBinding(declaration)
		class, isClass := declaration.(*ast.ClassDecl)
		constructorExported := isClass && !class.Abstract
		if !constructorExported && !goast.IsExported(generatedIdentifier(name)) && !exportedBodies[name] {
			continue
		}
		var parameters []ast.TypeParameter
		var scope map[string]Type
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			parameters, scope = declaration.TypeParameters, c.functionTypeParameters[declaration]
		case *ast.StructDecl:
			parameters, scope = declaration.TypeParameters, c.structs[name].typeParamScope
		case *ast.InterfaceDecl:
			parameters, scope = declaration.TypeParameters, c.interfaces[name].typeParamScope
		case *ast.ClassDecl:
			parameters, scope = declaration.TypeParameters, c.classes[name].typeParamScope
		case *ast.TypeDecl:
			if !declaration.Alias {
				parameters, scope = declaration.TypeParameters, c.nativeTypes[name].typeParamScope
			}
		}
		if len(parameters) < 2 {
			continue
		}
		pending := make(map[gotypes.Type]bool, len(parameters))
		for _, parameter := range parameters {
			pending[scope[parameter.Name].GoType] = true
		}
		for _, parameter := range parameters {
			identity, ok := scope[parameter.Name].GoType.(*gotypes.TypeParam)
			if !ok {
				continue
			}
			delete(pending, identity)
			if parameter.Constraint != nil && len(pending) != 0 && comparableBoundUsesPendingStorage(identity.Constraint(), pending) {
				c.report(parameter.Constraint.Span, fmt.Sprintf(
					"Go export data for %q would evaluate a comparable constraint for %q before a later type parameter is ready. Declare the element type parameters before %q",
					name, parameter.Name, parameter.Name))
			}
		}
	}
}

// Export data can include inlinable bodies that refer to private functions.
// Reuse the checked references that the initialization analysis collects.
func (c *Checker) goExportBodyDependencies(program *ast.Program) map[string]bool {
	reachable := map[string]bool{}
	var mark func(string)
	mark = func(name string) {
		if reachable[name] {
			return
		}
		reachable[name] = true
		for child := range c.globalDependencies[name] {
			mark(child)
		}
	}
	methods := func(owner string, declarations []*ast.MethodDecl) {
		for _, method := range declarations {
			if !goast.IsExported(method.GoName) {
				continue
			}
			if method.Static {
				mark(staticMemberDependency(owner, method.GoName))
			} else {
				mark(nativeMethodDependency(method.NameSpan))
			}
		}
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			if goast.IsExported(generatedIdentifier(declaration.Name)) {
				mark(declaration.Name)
			}
		case *ast.VariableDecl:
			if declaration.FunctionBinding && goast.IsExported(generatedIdentifier(declaration.Name)) {
				mark(declaration.Name)
			}
		case *ast.ClassDecl:
			if !declaration.Abstract {
				mark(classConstructionDependency(declaration.Name))
			}
			methods(declaration.Name, declaration.Methods)
		case *ast.StructDecl:
			methods(declaration.Name, declaration.Methods)
		case *ast.MethodDecl:
			if goast.IsExported(declaration.GoName) {
				mark(nativeMethodDependency(declaration.NameSpan))
			}
		}
	}
	return reachable
}

func comparableBoundUsesPendingStorage(bound gotypes.Type, pending map[gotypes.Type]bool) bool {
	comparableType := gotypes.Universe.Lookup("comparable").Type()
	var comparable, dependent bool
	visited := map[gotypes.Type]bool{}
	var scan func(gotypes.Type)
	scan = func(value gotypes.Type) {
		value = gotypes.Unalias(value)
		if value == comparableType {
			comparable = true
			return
		}
		if visited[value] || comparable && dependent {
			return
		}
		visited[value] = true
		switch shape := value.Underlying().(type) {
		case *gotypes.Interface:
			for i := 0; i < shape.NumEmbeddeds(); i++ {
				scan(shape.EmbeddedType(i))
			}
		case *gotypes.Union:
			for i := 0; i < shape.Len(); i++ {
				scan(shape.Term(i).Type())
			}
		default:
			dependent = dependent || comparableDependsOnPending(value, nil, pending, map[gotypes.Type]bool{})
		}
	}
	scan(bound)
	return comparable && dependent
}
