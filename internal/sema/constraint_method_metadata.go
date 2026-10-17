package sema

import (
	gotypes "go/types"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) recordTypeParameterMethods(program *ast.Program) {
	program.TypeParameterMethods = map[source.Span][]ast.ObjectTypeField{}
	record := func(parameters []ast.TypeParameter, scope map[string]Type) {
		for _, declaration := range parameters {
			parameter, ok := scope[declaration.Name].GoType.(*gotypes.TypeParam)
			if !ok {
				continue
			}
			if bound, native := c.nativeInterfaceBounds[parameter]; native {
				methods, bindings, _, _ := c.nativeConstraintMethods(bound)
				properties := map[string]bool{}
				for _, name := range sortedNativeConstraintMethodNames(methods) {
					method := methods[name]
					callable := substituteNativeTypeParameters(method.typeInfo, bindings)
					if strings.HasPrefix(name, "get ") || strings.HasPrefix(name, "set ") {
						property := name[4:]
						if properties[property] {
							continue
						}
						properties[property] = true
						value := callable.Result
						if strings.HasPrefix(name, "set ") && len(callable.Parameters) == 1 {
							value = &callable.Parameters[0]
						}
						if value != nil {
							program.TypeParameterMethods[declaration.NameSpan] = append(program.TypeParameterMethods[declaration.NameSpan], ast.ObjectTypeField{Name: property, Type: typeRefFromType(*value, method.declarationSpan), Span: method.declarationSpan})
						}
						continue
					}
					program.TypeParameterMethods[declaration.NameSpan] = append(program.TypeParameterMethods[declaration.NameSpan], ast.ObjectTypeField{Name: name, ParameterNames: callable.ParameterNames, Type: typeRefFromType(callable, method.declarationSpan), Span: method.declarationSpan})
				}
				continue
			}
			contract := underlyingGoInterface(parameter.Constraint())
			if contract == nil {
				continue
			}
			for _, method := range constraintMethods(contract) {
				if !method.Exported() || !c.allowUnsafeGo && goTypeContainsUnsafePointer(method.Type(), nil) {
					continue
				}
				callable, err := kinmokuseiFunctionFromGo(method.Type().(*gotypes.Signature))
				if err != nil {
					continue
				}
				program.TypeParameterMethods[declaration.NameSpan] = append(program.TypeParameterMethods[declaration.NameSpan], ast.ObjectTypeField{
					Name: method.Name(), Type: typeRefFromType(callable, declaration.NameSpan),
				})
			}
		}
	}
	for declaration, scope := range c.functionTypeParameters {
		record(declaration.TypeParameters, scope)
	}
	for declaration, scope := range c.receiverTypeParameters {
		record(declaration.TypeParameters, scope)
	}
	for declaration, scope := range c.methodTypeParameters {
		record(declaration.TypeParameters, scope)
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			if symbol := c.classes[declaration.Name]; symbol != nil {
				record(declaration.TypeParameters, symbol.typeParamScope)
			}
		case *ast.StructDecl:
			if symbol := c.structs[declaration.Name]; symbol != nil {
				record(declaration.TypeParameters, symbol.typeParamScope)
			}
		case *ast.InterfaceDecl:
			if symbol := c.interfaces[declaration.Name]; symbol != nil {
				record(declaration.TypeParameters, symbol.typeParamScope)
			}
		case *ast.TypeDecl:
			if symbol := c.nativeTypes[declaration.Name]; symbol != nil {
				record(declaration.TypeParameters, symbol.typeParamScope)
			}
		}
	}
}
