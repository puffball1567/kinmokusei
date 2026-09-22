package sema

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Applications are module-level expressions, not method-body expressions.
// This phase checks factories without executing them. The runtime context ABI
// and lowering are still gated; recording a target does not enable emission.
func (c *Checker) checkDecorators(program *ast.Program) {
	if len(program.Decorators) == 0 {
		return
	}
	for _, application := range program.Decorators {
		application.Target = nil
	}
	attach := func(list []*ast.Decorator, target ast.DecoratorTarget) {
		for _, application := range list {
			copy := target
			application.Target = &copy
			before := len(c.diagnostics)
			value := c.callableType(c.checkExpression(application.Expression))
			if len(c.diagnostics) == before && value.Kind != Invalid && value.Kind != Function {
				c.report(application.Span, "decorator application must resolve to a function; factories must return a function")
			}
		}
	}
	parameters := func(parameters []ast.Parameter, owner source.Span, static bool, visibility ast.Visibility) {
		for i := range parameters {
			parameter := &parameters[i]
			attach(parameter.Decorators, ast.DecoratorTarget{Kind: "parameter", Name: parameter.Name, Owner: owner, Declaration: parameter.Span, ParameterIndex: i, Static: static, Visibility: visibility, ValueType: &parameter.Type})
		}
	}
	signature := func(parameters []ast.Parameter, result *ast.TypeRef) *ast.TypeRef {
		ref := &ast.TypeRef{Return: result}
		for _, parameter := range parameters {
			ref.Parameters = append(ref.Parameters, parameter.Type)
			ref.Variadic = parameter.Variadic
		}
		return ref
	}
	for _, declaration := range program.Declarations {
		class, ok := declaration.(*ast.ClassDecl)
		if !ok {
			continue
		}
		classType := &ast.TypeRef{Name: class.Name, Span: class.NameSpan}
		for _, parameter := range class.TypeParameters {
			classType.GenericArguments = append(classType.GenericArguments, ast.TypeRef{Name: parameter.Name, TypeParameter: true, Span: parameter.Span})
		}
		attach(class.Decorators, ast.DecoratorTarget{Kind: "class", Name: class.Name, Declaration: class.NameSpan, ParameterIndex: -1, ValueType: classType})
		for i := range class.Fields {
			field := &class.Fields[i]
			attach(field.Decorators, ast.DecoratorTarget{Kind: "field", Name: field.Name, Owner: class.NameSpan, Declaration: field.NameSpan, ParameterIndex: -1, Static: field.Static, Visibility: field.Visibility, ValueType: &field.Type})
		}
		if constructor := class.Constructor; constructor != nil {
			attach(constructor.Decorators, ast.DecoratorTarget{Kind: "constructor", Name: "constructor", Owner: class.NameSpan, Declaration: constructor.Span, ParameterIndex: -1, ValueType: signature(constructor.Parameters, classType)})
			parameters(constructor.Parameters, constructor.Span, false, ast.Public)
		}
		for _, method := range class.Methods {
			kind := "method"
			if method.Accessor != "" {
				kind = method.Accessor
			}
			attach(method.Decorators, ast.DecoratorTarget{Kind: kind, Name: method.Name, Owner: class.NameSpan, Declaration: method.NameSpan, ParameterIndex: -1, Static: method.Static, Visibility: method.Visibility, ValueType: signature(method.Parameters, &method.ReturnType)})
			parameters(method.Parameters, method.NameSpan, method.Static, method.Visibility)
		}
	}
	for _, application := range program.Decorators {
		if application.Target == nil {
			c.report(application.Span, "decorator target must be a class, class member, or constructor/method parameter")
		}
		c.report(application.Span, "decorator execution and metadata generation are not implemented yet")
	}
}
