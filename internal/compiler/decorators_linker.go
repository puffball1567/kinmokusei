package compiler

import "github.com/puffball1567/kinmokusei/internal/ast"

// Decorator factories are evaluated in module scope. A decorated parameter or
// member with the same spelling must not shadow an imported factory.
func (linker *sourceLinker) linkClassDecorators(class *ast.ClassDecl, visible moduleNames) {
	link := func(list []*ast.Decorator) {
		for _, application := range list {
			linker.linkExpression(application.Expression, visible, nil)
		}
	}
	link(class.Decorators)
	for _, field := range class.Fields {
		link(field.Decorators)
	}
	if class.Constructor != nil {
		link(class.Constructor.Decorators)
		for _, parameter := range class.Constructor.Parameters {
			link(parameter.Decorators)
		}
	}
	for _, method := range class.Methods {
		link(method.Decorators)
		for _, parameter := range method.Parameters {
			link(parameter.Decorators)
		}
	}
}
