package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkMember(expr *ast.MemberExpr) Type {
	if identifier, ok := expr.Object.(*ast.IdentifierExpr); ok {
		if c.inFieldInitializer && identifier.Name == "this" {
			c.report(identifier.Span, "class field initializers cannot reference this or super; use the constructor")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if identifier.Name == "super" {
			return c.checkSuperMember(expr)
		}
		if enumeration := c.enums[identifier.Name]; enumeration != nil && c.isTopLevelAllowed(identifier.Span, identifier.Name) {
			member := enumeration.members[expr.Name]
			if member == nil {
				c.report(expr.Span, fmt.Sprintf("enum %s has no member %q", identifier.Name, expr.Name))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			identifier.ResolvedDeclaration = enumeration.declaration.NameSpan
			expr.ResolvedDeclaration = member.NameSpan
			expr.ResolvedName = enumMemberGoName(identifier.Name, member.Name)
			expr.Static = true
			expr.Constant = true
			return c.resolveNativeType(c.nativeTypes[identifier.Name])
		}
		if class := c.classes[identifier.Name]; class != nil && c.isTopLevelAllowed(identifier.Span, identifier.Name) {
			method, exists := class.methods[expr.Name]
			if !exists || !method.static {
				c.report(expr.Span, fmt.Sprintf("class %s has no static method %q", identifier.Name, expr.Name))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			if !c.canAccessClassMember(method.visibility, method.declaringClass) {
				c.reportInaccessibleClassMember(expr.Span, "method", expr.Name, method.visibility)
			}
			expr.Static = true
			identifier.ResolvedDeclaration = class.declarationSpan
			expr.ResolvedDeclaration = method.declarationSpan
			owner := method.declaringClass
			if owner == "" {
				owner = identifier.Name
			}
			expr.ResolvedName = staticMethodGoName(owner, method.goName, method.visibility)
			if method.typeInfo.Generic && c.directCallCallee != expr {
				c.report(expr.Span, "generic methods must be called directly; Go cannot represent an uninstantiated generic method value")
			}
			return method.typeInfo
		}
	}
	object := c.singleValue(c.checkExpression(expr.Object), expr.Object.GetSpan())
	if object.Kind == Nullable {
		message := fmt.Sprintf("nullable value %s must be checked against null before member access", object.String())
		if invalidated, cause := c.flowInvalidation(expr.Object); invalidated.Start.Line != 0 {
			if cause == "" {
				cause = "a possible mutation"
			}
			message += fmt.Sprintf("; the previous non-null proof was invalidated by %s at %d:%d", cause, invalidated.Start.Line, invalidated.Start.Column)
		}
		c.report(expr.Object.GetSpan(), message)
		if object.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object = *object.Element
	}
	if object.Kind == Object {
		field, ok := object.Fields[expr.Name]
		if !ok {
			c.report(expr.Span, fmt.Sprintf("object has no member %q", expr.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ResolvedName = object.FieldNames[expr.Name]
		expr.Addressable = c.isAddressableExpression(expr.Object)
		return field
	}
	if object.Kind == Class {
		class := c.classes[object.Name]
		if class == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if field, ok := class.fields[expr.Name]; ok {
			if !c.canAccessClassMember(field.visibility, field.declaringClass) {
				c.reportInaccessibleClassMember(expr.Span, "field", expr.Name, field.visibility)
			}
			expr.ResolvedName = field.goName
			expr.ResolvedDeclaration = field.declarationSpan
			expr.Addressable = true
			fieldType := substituteNativeTypeParameters(field.typeInfo, nativeClassBindings(class, object))
			if key, stable := c.stableMemberFlowKey(expr); stable {
				c.memberTypes[key] = fieldType
				if state, proven := c.memberFlow[key]; proven && state.nonNull && fieldType.Kind == Nullable {
					return state.nonNullType
				}
			}
			return fieldType
		}
		if method, ok := class.methods[expr.Name]; ok {
			if method.static {
				c.report(expr.Span, fmt.Sprintf("static method %q cannot be called on an instance", expr.Name))
			}
			if !c.canAccessClassMember(method.visibility, method.declaringClass) {
				c.reportInaccessibleClassMember(expr.Span, "method", expr.Name, method.visibility)
			}
			expr.ResolvedName = method.goName
			expr.ResolvedDeclaration = method.declarationSpan
			expr.VirtualDispatch = method.virtual
			expr.VirtualOwner = method.virtualOwner
			if len(method.typeInfo.TypeParameters) != 0 {
				expr.GenericMethod = true
				expr.ResolvedName = staticMethodGoName(method.declaringClass, method.goName, method.visibility)
				if method.declaringClass != object.Name {
					expr.GenericReceiverUpcast = "__kinmokuseiUpcast" + object.Name + "To" + method.declaringClass
					for _, argument := range object.TypeArguments {
						expr.GenericReceiverTypeArguments = append(expr.GenericReceiverTypeArguments, typeRefFromType(argument, expr.Object.GetSpan()))
					}
				}
				if c.directCallCallee != expr {
					c.report(expr.Span, "generic methods must be called directly; Go cannot represent an uninstantiated generic method value")
				}
			}
			return c.substituteNativeMethodOwnerTypeParameters(method.typeInfo, nativeClassBindings(class, object))
		}
		c.report(expr.Span, fmt.Sprintf("class %s has no member %q", object.Name, expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	structObject := object
	structPointer := false
	if object.Kind == GoPointer && object.Element != nil && object.Element.Kind == Struct {
		structObject = *object.Element
		structPointer = true
	}
	if structObject.Kind == Struct {
		structure := c.structs[structObject.Name]
		if structure == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if field, ok := structure.fields[expr.Name]; ok {
			expr.ResolvedName = field.goName
			expr.ResolvedDeclaration = field.declarationSpan
			expr.Addressable = structPointer || c.isAddressableExpression(expr.Object)
			return substituteNativeTypeParameters(field.typeInfo, nativeStructBindings(structure, structObject))
		}
		if method, ok := structure.methods[expr.Name]; ok {
			if method.pointerReceiver && !structPointer && !c.isAddressableExpression(expr.Object) {
				c.report(expr.Span, fmt.Sprintf("pointer method %q requires an addressable %s value", expr.Name, structObject.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			expr.ResolvedName = method.goName
			expr.ResolvedDeclaration = method.declarationSpan
			if len(method.typeInfo.TypeParameters) != 0 {
				expr.GenericMethod = true
				expr.ResolvedName = staticMethodGoName(structObject.Name, method.goName, method.visibility)
				expr.GenericReceiverAddress = method.pointerReceiver && !structPointer
				if c.directCallCallee != expr {
					c.report(expr.Span, "generic methods must be called directly; Go cannot represent an uninstantiated generic method value")
				}
			}
			return c.substituteNativeMethodOwnerTypeParameters(method.typeInfo, nativeStructBindings(structure, structObject))
		}
		c.report(expr.Span, fmt.Sprintf("struct %s has no field %q or method with that name", structObject.Name, expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	nativeObject := object
	nativePointer := false
	if object.Kind == GoPointer && object.Element != nil && object.Element.Kind == GoNamed {
		nativeObject = *object.Element
		nativePointer = true
	}
	if nativeObject.Kind == GoNamed && nativeObject.GoQualifier == "" {
		if namedObject := goTypeNameObject(nativeObject.GoType); namedObject != nil {
			if symbol := c.nativeTypes[namedObject.Name()]; symbol != nil && !symbol.declaration.Alias {
				underlying := c.nativeDefinedUnderlying(symbol, nativeObject)
				if underlying.Kind == Struct {
					if structure := c.structs[underlying.Name]; structure != nil {
						if field, exists := structure.fields[expr.Name]; exists {
							expr.ResolvedName = field.goName
							expr.ResolvedDeclaration = field.declarationSpan
							expr.Addressable = nativePointer || c.isAddressableExpression(expr.Object)
							return substituteNativeTypeParameters(field.typeInfo, nativeStructBindings(structure, underlying))
						}
					}
				}
				method, exists := symbol.methods[expr.Name]
				if !exists {
					if underlying.Kind == Struct {
						c.report(expr.Span, fmt.Sprintf("defined type %s has no field or method %q", nativeObject.String(), expr.Name))
					} else {
						c.report(expr.Span, fmt.Sprintf("defined type %s has no method %q", nativeObject.String(), expr.Name))
					}
					return Type{Kind: Invalid, Name: "<invalid>"}
				}
				if method.pointerReceiver && !nativePointer && !c.isAddressableExpression(expr.Object) {
					c.report(expr.Span, fmt.Sprintf("pointer method %q requires an addressable %s value", expr.Name, nativeObject.String()))
					return Type{Kind: Invalid, Name: "<invalid>"}
				}
				if method.visibility != ast.Public && method.declarationSpan.Path != expr.Span.Path {
					c.report(expr.Span, fmt.Sprintf("method %q is private on defined type %s", expr.Name, nativeObject.String()))
				}
				expr.ResolvedName = method.goName
				expr.ResolvedDeclaration = method.declarationSpan
				return substituteNativeTypeParameters(method.typeInfo, nativeDefinedTypeBindings(symbol, nativeObject))
			}
		}
	}
	if object.Kind == Interface {
		contract := c.interfaces[object.Name]
		if contract == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		method, ok := contract.methods[expr.Name]
		if !ok {
			c.report(expr.Span, fmt.Sprintf("interface %s has no method %q", object.Name, expr.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		expr.ResolvedName = method.goName
		expr.ResolvedDeclaration = method.declarationSpan
		return substituteNativeTypeParameters(method.typeInfo, nativeInterfaceBindings(contract, object))
	}
	if object.Kind == GoPackage {
		return c.checkGoMember(expr, object.GoPackage)
	}
	if object.Kind == GoInterface || object.GoType != nil && object.Kind != GoTypeName {
		return c.checkGoValueMember(expr, object)
	}
	c.report(expr.Span, fmt.Sprintf("type %s has no members", object.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkSuperMember(expr *ast.MemberExpr) Type {
	if c.inFieldInitializer {
		c.report(expr.Span, "class field initializers cannot reference this or super; use the constructor")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	class := c.classes[c.currentClass]
	if class == nil || class.base == "" {
		c.report(expr.Span, "super member access requires a derived class")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	base := c.classes[class.base]
	if base == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	method, ok := base.methods[expr.Name]
	if !ok || method.static {
		c.report(expr.Span, fmt.Sprintf("base class %s has no instance method %q", class.base, expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if method.visibility == ast.Private {
		c.report(expr.Span, fmt.Sprintf("method %q is private in base class %s", expr.Name, method.declaringClass))
	}
	if _, ok := c.lookupSymbol("this", expr.Span); !ok {
		c.report(expr.Span, "super cannot be used in a static method")
	}
	expr.Super = true
	expr.SuperBase = class.base
	expr.ResolvedName = method.goName
	expr.VirtualOwner = method.virtualOwner
	expr.ResolvedDeclaration = method.declarationSpan
	if len(method.typeInfo.TypeParameters) != 0 {
		expr.GenericMethod = true
		expr.ResolvedName = staticMethodGoName(method.declaringClass, method.goName, method.visibility)
		expr.GenericReceiverSuperBase = class.base
		if method.declaringClass != class.base {
			expr.GenericReceiverUpcast = "__kinmokuseiUpcast" + class.base + "To" + method.declaringClass
			for _, argument := range class.baseType.TypeArguments {
				expr.GenericReceiverTypeArguments = append(expr.GenericReceiverTypeArguments, typeRefFromType(argument, expr.Span))
			}
		}
		if c.directCallCallee != expr {
			c.report(expr.Span, "generic methods must be called directly; Go cannot represent an uninstantiated generic method value")
		}
	}
	return c.substituteNativeMethodOwnerTypeParameters(method.typeInfo, nativeClassBindings(base, class.baseType))
}

func (c *Checker) checkNew(expr *ast.NewExpr) Type {
	class, ok := c.classes[expr.ClassName]
	ok = ok && c.isTopLevelAllowed(expr.Span, expr.ClassName)
	if !ok {
		c.report(expr.Span, fmt.Sprintf("unknown class %q", expr.ClassName))
		for _, arg := range expr.Arguments {
			c.checkExpression(arg)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if expr.ClassName == "Exception" {
		c.usesExceptions = true
	}
	expr.ResolvedDeclaration = class.declarationSpan
	classType := c.resolveNativeClassType(ast.TypeRef{Name: expr.ClassName, GenericArguments: expr.TypeArguments, Span: expr.Span}, class)
	parameters := class.constructor
	if classType.Kind != Invalid {
		bindings := nativeClassBindings(class, classType)
		parameters = make([]Type, len(class.constructor))
		for index := range class.constructor {
			parameters[index] = substituteNativeTypeParameters(class.constructor[index], bindings)
		}
	}
	c.checkConstructorArguments(expr.Arguments, expr.Expanded, parameters, class.constructorVariadic, fmt.Sprintf("constructor %q", expr.ClassName), expr.Span)
	return classType
}

func (c *Checker) checkConstructorArguments(arguments []ast.Expression, expanded bool, parameters []Type, variadic bool, name string, span source.Span) {
	minimumArguments := len(parameters)
	if variadic {
		minimumArguments--
	}
	if expanded {
		if !variadic {
			c.report(span, fmt.Sprintf("%s is not variadic and cannot receive a spread argument", name))
		} else if len(arguments) != len(parameters) {
			c.report(span, fmt.Sprintf("spread call to %s expects %d arguments (%d fixed and one slice), got %d", name, len(parameters), minimumArguments, len(arguments)))
		}
	} else if len(arguments) < minimumArguments || (!variadic && len(arguments) != len(parameters)) {
		if variadic {
			c.report(span, fmt.Sprintf("%s expects at least %d arguments, got %d", name, minimumArguments, len(arguments)))
		} else {
			c.report(span, fmt.Sprintf("%s expects %d arguments, got %d", name, len(parameters), len(arguments)))
		}
	}
	for index, argument := range arguments {
		parameterIndex := index
		if variadic && parameterIndex >= len(parameters)-1 {
			parameterIndex = len(parameters) - 1
		}
		if parameterIndex < 0 || parameterIndex >= len(parameters) {
			c.checkExpression(argument)
			continue
		}
		expected := parameters[parameterIndex]
		if expanded && variadic && index == len(arguments)-1 {
			element := expected
			expected = Type{Kind: Array, Name: "array", Element: &element}
		}
		actual := c.checkExpressionExpectedSlot(&arguments[index], expected)
		c.requireAssignable(expected, actual, argument.GetSpan())
	}
}
