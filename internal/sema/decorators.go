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
			contextName, valid := decoratorCallbackContext(value)
			if !valid {
				c.report(application.Span, "decorator callback must have type (context: DecoratorContext) => void or use a target-specific decorator context")
				continue
			}
			if !ast.DecoratorContextAcceptsTarget(contextName, target.Kind) {
				c.report(application.Span, fmt.Sprintf("decorator callback using %s cannot be applied to %s target", contextName, target.Kind))
			}
		}
	}
	parameters := func(parameters []ast.Parameter, className, classID, baseID, memberName, ownerID string, overrideChain []string, owner source.Span, static bool, visibility ast.Visibility) {
		for i := range parameters {
			parameter := &parameters[i]
			parameterOverrideChain := make([]string, len(overrideChain))
			for index, identity := range overrideChain {
				parameterOverrideChain[index] = fmt.Sprintf("%s|parameter|%d", identity, i)
			}
			attach(parameter.Decorators, ast.DecoratorTarget{
				Kind: "parameter", Name: parameter.Name, Identity: fmt.Sprintf("%s|parameter|%d", ownerID, i),
				ClassName: className, ClassIdentity: classID, BaseIdentity: baseID, MemberName: memberName, ParameterName: parameter.Name,
				OverrideChain: parameterOverrideChain, Owner: owner, Declaration: parameter.Span, ParameterIndex: i, Static: static, Visibility: visibility, ValueType: &parameter.Type,
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
		baseID := ""
		if class.Base != nil && c.classes[class.Base.Name] != nil {
			baseID = "type|" + class.Base.Name
		}
		classType := &ast.TypeRef{Name: className, Span: class.NameSpan}
		for _, parameter := range class.TypeParameters {
			classType.GenericArguments = append(classType.GenericArguments, ast.TypeRef{Name: parameter.Name, TypeParameter: true, Span: parameter.Span})
		}
		constructible := true
		constructUnavailableReason := ""
		var constructorParameters []ast.TypeRef
		var constructorContracts []string
		classContract := decoratorValueContract(Type{Kind: Class, Name: class.Name})
		if class.Abstract {
			constructible = false
			constructUnavailableReason = "abstract classes cannot be constructed"
		} else if len(class.TypeParameters) != 0 {
			constructible = false
			constructUnavailableReason = "generic classes require concrete type arguments"
		} else if class.Constructor != nil {
			for _, parameter := range class.Constructor.Parameters {
				parameterType := c.resolveType(parameter.Type)
				c.prepareGoTypeForEmission(&parameterType, parameter.Span)
				constructorParameters = append(constructorParameters, typeRefFromType(parameterType, parameter.Type.Span))
				if parameter.Variadic && parameterType.Element != nil {
					parameterType = *parameterType.Element
				}
				constructorContracts = append(constructorContracts, decoratorValueContract(parameterType))
			}
		}
		attach(class.Decorators, ast.DecoratorTarget{
			Kind: "class", Name: className, Identity: classID, ClassIdentity: classID, BaseIdentity: baseID, ClassName: className, ValueIdentity: classID,
			RuntimeClassName: class.Name, Constructible: constructible, ConstructUnavailableReason: constructUnavailableReason, ConstructorParameters: constructorParameters,
			ConstructorContracts: constructorContracts, ClassContract: classContract,
			ConstructorVariadic: class.Constructor != nil && hasVariadicParameter(class.Constructor.Parameters),
			Declaration:         class.NameSpan, ParameterIndex: -1, Visibility: ast.Public, ValueType: classType,
		})
		for i := range class.Fields {
			field := &class.Fields[i]
			attach(field.Decorators, ast.DecoratorTarget{
				Kind: "field", Name: field.Name, Identity: classID + "|field|" + field.Name,
				ClassName: className, ClassIdentity: classID, BaseIdentity: baseID, MemberName: field.Name,
				Owner: class.NameSpan, Declaration: field.NameSpan, ParameterIndex: -1, Static: field.Static, Visibility: field.Visibility, ValueType: &field.Type,
			})
		}
		if constructor := class.Constructor; constructor != nil {
			constructorID := classID + "|constructor"
			attach(constructor.Decorators, ast.DecoratorTarget{
				Kind: "constructor", Name: "constructor", Identity: constructorID,
				ClassName: className, ClassIdentity: classID, BaseIdentity: baseID, MemberName: "constructor",
				RuntimeClassName: class.Name, Constructible: constructible, ConstructUnavailableReason: constructUnavailableReason, ConstructorParameters: constructorParameters,
				ConstructorContracts: constructorContracts, ClassContract: classContract,
				ConstructorVariadic: hasVariadicParameter(constructor.Parameters),
				Owner:               class.NameSpan, Declaration: constructor.Span, ParameterIndex: -1, Visibility: ast.Public, ValueType: signature(constructor.Parameters, classType),
			})
			parameters(constructor.Parameters, className, classID, baseID, "constructor", constructorID, nil, constructor.Span, false, ast.Public)
		}
		for _, method := range class.Methods {
			kind := "method"
			if method.Accessor != "" {
				kind = method.Accessor
			}
			methodID := classID + "|" + kind + "|" + method.Name
			overrideChain := c.decoratorOverrideChain(class, method, kind)
			eligible, reason := true, ""
			switch {
			case method.Visibility != ast.Public:
				eligible, reason = false, "method is not public"
			case method.Abstract:
				eligible, reason = false, "abstract methods cannot be invoked"
			case len(method.TypeParameters) != 0 || len(class.TypeParameters) != 0 && !(method.Static && method.Accessor != ""):
				eligible, reason = false, "generic methods require concrete type arguments"
			}
			// Static properties belong to the declaration, not a generic class
			// instance. Their signatures are already checked without owner type
			// parameters and can use the same receiver-free adapter as methods.
			methodResult := method.ReturnType
			payloadType := Type{Kind: Invalid}
			var resultSlots []ast.DecoratorResultSlot
			if eligible {
				result := c.resolveType(method.ReturnType)
				c.prepareGoTypeForEmission(&result, method.ReturnType.Span)
				methodResult = typeRefFromType(result, method.ReturnType.Span)
				payloadType, resultSlots, reason = decoratorInvocationResult(result, method.ReturnType.Span)
				eligible = reason == ""
			}
			invocable := eligible && !method.Static
			staticInvocable := eligible && method.Static
			invokeReason, staticInvokeReason := reason, reason
			if eligible && method.Static {
				invokeReason = "static method requires invokeStatic"
			} else if eligible {
				staticInvokeReason = "method is not static"
			}
			runtimeMethodName := method.GoName
			if runtimeMethodName == "" {
				runtimeMethodName = method.Name
			}
			if method.Override {
				runtimeMethodName = "__kinmokusei" + method.VirtualOwner + runtimeMethodName
			}
			resultIdentity := ""
			resultContract := ""
			if eligible {
				resultIdentity = decoratorValueRuntimeIdentity(payloadType)
				resultContract = decoratorValueContract(payloadType)
			}
			var methodParameters []ast.TypeRef
			var methodContracts []string
			for _, parameter := range method.Parameters {
				if eligible {
					parameterType := c.resolveType(parameter.Type)
					c.prepareGoTypeForEmission(&parameterType, parameter.Span)
					methodParameters = append(methodParameters, typeRefFromType(parameterType, parameter.Type.Span))
					if parameter.Variadic && parameterType.Element != nil {
						parameterType = *parameterType.Element
					}
					methodContracts = append(methodContracts, decoratorValueContract(parameterType))
				}
			}
			attach(method.Decorators, ast.DecoratorTarget{
				Kind: kind, Name: method.Name, Identity: methodID,
				ClassName: className, ClassIdentity: classID, BaseIdentity: baseID, MemberName: method.Name,
				RuntimeClassName: class.Name, Invocable: invocable, InvokeUnavailableReason: invokeReason,
				StaticInvocable: staticInvocable, StaticInvokeUnavailableReason: staticInvokeReason,
				RuntimeMethodName: runtimeMethodName, MethodParameters: methodParameters,
				MethodContracts: methodContracts, ClassContract: classContract,
				MethodVariadic: hasVariadicParameter(method.Parameters), MethodResult: &methodResult,
				MethodResultIdentity: resultIdentity,
				MethodResultContract: resultContract,
				MethodResultSlots:    resultSlots,
				OverrideChain:        overrideChain, Owner: class.NameSpan, Declaration: method.NameSpan, ParameterIndex: -1, Static: method.Static, Visibility: method.Visibility, ValueType: signature(method.Parameters, &method.ReturnType),
			})
			parameters(method.Parameters, className, classID, baseID, method.Name, methodID, overrideChain, method.NameSpan, method.Static, method.Visibility)
		}
	}
	for _, application := range program.Decorators {
		if application.Target == nil {
			c.report(application.Span, "decorator target must be a class, class member, or constructor/method parameter")
		}
	}
}

func (c *Checker) decoratorOverrideChain(class *ast.ClassDecl, method *ast.MethodDecl, kind string) []string {
	if class.Base == nil {
		return nil
	}
	key := method.Name
	if method.Accessor != "" {
		key = method.Accessor + " " + method.Name
	}
	var identities []string
	seen := map[string]bool{}
	for name := class.Base.Name; name != ""; {
		base := c.classes[name]
		if base == nil {
			break
		}
		if inherited, exists := base.methods[key]; exists && !seen[inherited.declaringClass] {
			seen[inherited.declaringClass] = true
			identities = append(identities, "type|"+inherited.declaringClass+"|"+kind+"|"+method.Name)
		}
		name = base.base
	}
	return identities
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

func decoratorCallbackContext(value Type) (string, bool) {
	if value.Kind != Function || value.Variadic || len(value.Parameters) != 1 ||
		value.Parameters[0].Kind != Object || !ast.IsDecoratorContextTypeName(value.Parameters[0].Name) ||
		value.Result == nil || value.Result.Kind != Void {
		return "", false
	}
	return value.Parameters[0].Name, true
}
