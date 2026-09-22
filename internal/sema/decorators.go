package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Applications are module-level expressions, not method-body expressions.
// Factories are checked here and executed later by the generated Go init.
func (c *Checker) checkDecorators(program *ast.Program) {
	if len(program.Decorators) == 0 {
		return
	}
	c.usesDecoratorContext = true
	for _, application := range program.Decorators {
		application.Target = nil
	}
	attach := func(list []*ast.Decorator, target ast.DecoratorTarget) {
		if target.ValueIdentity == "" {
			target.ValueIdentity = c.decoratorValueIdentity(target.ValueType)
		}
		for _, application := range list {
			copy := target
			application.Target = &copy
			before := len(c.diagnostics)
			value := c.callableType(c.checkExpression(application.Expression))
			if len(c.diagnostics) != before || value.Kind == Invalid {
				continue
			}
			if value.Kind != Function {
				c.report(application.Span, "decorator application must resolve to a function; factories must return a function")
				continue
			}
			if !isDecoratorCallback(value) {
				c.report(application.Span, "decorator callback must have type (context: DecoratorContext) => void")
			}
		}
	}
	parameters := func(parameters []ast.Parameter, className, memberName, ownerID string, owner source.Span, static bool, visibility ast.Visibility) {
		for i := range parameters {
			parameter := &parameters[i]
			attach(parameter.Decorators, ast.DecoratorTarget{
				Kind: "parameter", Name: parameter.Name, Identity: fmt.Sprintf("%s|parameter|%d", ownerID, i),
				ClassName: className, MemberName: memberName, ParameterName: parameter.Name,
				Owner: owner, Declaration: parameter.Span, ParameterIndex: i, Static: static, Visibility: visibility, ValueType: &parameter.Type,
			})
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
		className := class.SourceName
		if className == "" {
			className = class.Name
		}
		classID := "type|" + class.Name
		classType := &ast.TypeRef{Name: className, Span: class.NameSpan}
		for _, parameter := range class.TypeParameters {
			classType.GenericArguments = append(classType.GenericArguments, ast.TypeRef{Name: parameter.Name, TypeParameter: true, Span: parameter.Span})
		}
		attach(class.Decorators, ast.DecoratorTarget{
			Kind: "class", Name: className, Identity: classID, ClassName: className, ValueIdentity: classID,
			Declaration: class.NameSpan, ParameterIndex: -1, Visibility: ast.Public, ValueType: classType,
		})
		for i := range class.Fields {
			field := &class.Fields[i]
			attach(field.Decorators, ast.DecoratorTarget{
				Kind: "field", Name: field.Name, Identity: classID + "|field|" + field.Name,
				ClassName: className, MemberName: field.Name,
				Owner: class.NameSpan, Declaration: field.NameSpan, ParameterIndex: -1, Static: field.Static, Visibility: field.Visibility, ValueType: &field.Type,
			})
		}
		if constructor := class.Constructor; constructor != nil {
			constructorID := classID + "|constructor"
			attach(constructor.Decorators, ast.DecoratorTarget{
				Kind: "constructor", Name: "constructor", Identity: constructorID,
				ClassName: className, MemberName: "constructor",
				Owner: class.NameSpan, Declaration: constructor.Span, ParameterIndex: -1, Visibility: ast.Public, ValueType: signature(constructor.Parameters, classType),
			})
			parameters(constructor.Parameters, className, "constructor", constructorID, constructor.Span, false, ast.Public)
		}
		for _, method := range class.Methods {
			kind := "method"
			if method.Accessor != "" {
				kind = method.Accessor
			}
			methodID := classID + "|" + kind + "|" + method.Name
			attach(method.Decorators, ast.DecoratorTarget{
				Kind: kind, Name: method.Name, Identity: methodID,
				ClassName: className, MemberName: method.Name,
				Owner: class.NameSpan, Declaration: method.NameSpan, ParameterIndex: -1, Static: method.Static, Visibility: method.Visibility, ValueType: signature(method.Parameters, &method.ReturnType),
			})
			parameters(method.Parameters, className, method.Name, methodID, method.NameSpan, method.Static, method.Visibility)
		}
	}
	for _, application := range program.Decorators {
		if application.Target == nil {
			c.report(application.Span, "decorator target must be a class, class member, or constructor/method parameter")
		}
	}
}

func (c *Checker) decoratorValueIdentity(ref *ast.TypeRef) string {
	if ref == nil || ref.Qualifier != "" || ref.Name == "" || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() || ref.GoInterface || len(ref.GoResults) != 0 {
		return ""
	}
	if c.classes[ref.Name] != nil {
		return "type|" + ref.Name
	}
	if c.interfaces[ref.Name] != nil {
		return "interface|" + ref.Name
	}
	if c.structs[ref.Name] != nil {
		return "struct|" + ref.Name
	}
	return ""
}

func isDecoratorCallback(value Type) bool {
	return value.Kind == Function && !value.Variadic && len(value.Parameters) == 1 &&
		value.Parameters[0].Kind == Object && value.Parameters[0].Name == ast.DecoratorContextTypeName &&
		value.Result != nil && value.Result.Kind == Void
}
