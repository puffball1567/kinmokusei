package sema

import (
	"fmt"
	gotypes "go/types"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) declareClasses(program *ast.Program) {
	declarations := map[string]*ast.ClassDecl{}
	for _, declaration := range program.Declarations {
		if class, ok := declaration.(*ast.ClassDecl); ok {
			declarations[class.Name] = class
		}
	}
	state := map[string]uint8{}
	var declare func(*ast.ClassDecl)
	declare = func(decl *ast.ClassDecl) {
		switch state[decl.Name] {
		case 2:
			return
		case 1:
			c.report(decl.NameSpan, fmt.Sprintf("inheritance cycle contains class %s", decl.Name))
			return
		}
		state[decl.Name] = 1
		if decl.Base != nil {
			base := decl.Base
			if base.Nullable || base.Qualifier != "" || base.Name == "" || base.IsArray() || base.IsPointer() || base.IsFunction() || base.IsObject() {
				c.report(base.Span, "extends expects an unqualified class name")
			} else if baseDecl := declarations[base.Name]; baseDecl != nil {
				declare(baseDecl)
				if baseDecl.Final {
					c.report(base.Span, fmt.Sprintf("cannot extend final class %s", base.Name))
				}
			} else if builtinBase := c.classes[base.Name]; builtinBase != nil {
				c.usesExceptions = true
				if builtinBase.final {
					c.report(decl.Base.Span, fmt.Sprintf("cannot extend final class %s", decl.Base.Name))
				}
			} else {
				c.report(base.Span, fmt.Sprintf("unknown base class %q", base.Name))
			}
		}
		c.declareClass(decl)
		state[decl.Name] = 2
	}
	for _, declaration := range program.Declarations {
		if class, ok := declaration.(*ast.ClassDecl); ok {
			declare(class)
		}
	}
	for name, declaration := range declarations {
		symbol := c.classes[name]
		if symbol == nil || len(symbol.ancestors) == 0 {
			continue
		}
		root := symbol.ancestors[len(symbol.ancestors)-1]
		declaration.HierarchyRoot = root
		if rootDeclaration := declarations[root]; rootDeclaration != nil {
			rootDeclaration.HierarchyRoot = root
		}
		for _, ancestor := range symbol.ancestors {
			if ancestorDeclaration := declarations[ancestor]; ancestorDeclaration != nil {
				ancestorDeclaration.HierarchyRoot = root
			}
		}
	}
}

func (c *Checker) installExceptionBuiltin() {
	stringType := builtins["string"]
	errorType := builtins["error"]
	methodResult := stringType
	c.classes["Exception"] = &classSymbol{
		fields: map[string]fieldSymbol{
			"message": {
				typeInfo: stringType, visibility: ast.Public, goName: "Message", declaringClass: "Exception",
			},
		},
		methods: map[string]methodSymbol{
			"error": {
				typeInfo: Type{Kind: Function, Name: "function", Result: &methodResult}, visibility: ast.Public,
				goName: "Error", declaringClass: "Exception",
			},
		},
		constructor:  []Type{stringType},
		implements:   map[string]bool{},
		goImplements: []gotypes.Type{errorType.GoType},
	}
}

func (c *Checker) declareClass(decl *ast.ClassDecl) {
	predeclared := c.classes[decl.Name]
	symbol := &classSymbol{
		fields: map[string]fieldSymbol{}, methods: map[string]methodSymbol{}, implements: map[string]bool{}, declarationSpan: decl.NameSpan, final: decl.Final,
	}
	if predeclared != nil {
		symbol.typeParameters = predeclared.typeParameters
		symbol.typeParamScope = predeclared.typeParamScope
		symbol.goNamed = predeclared.goNamed
	}
	c.classes[decl.Name] = symbol
	c.pushTypeParameterScope(symbol.typeParamScope)
	defer c.popTypeParameterScope()
	if decl.Base != nil {
		base := c.classes[decl.Base.Name]
		if base != nil && base.fields != nil && decl.Base.Name != decl.Name {
			baseType := c.resolveNativeClassType(*decl.Base, base)
			baseBindings := nativeClassBindings(base, baseType)
			symbol.base = decl.Base.Name
			symbol.baseType = baseType
			symbol.ancestors = append([]string{decl.Base.Name}, base.ancestors...)
			symbol.ancestorTypes = append(symbol.ancestorTypes, baseType)
			for _, ancestorType := range base.ancestorTypes {
				symbol.ancestorTypes = append(symbol.ancestorTypes, substituteNativeTypeParameters(ancestorType, baseBindings))
			}
			decl.Ancestors = append(decl.Ancestors, symbol.ancestors...)
			for _, ancestorType := range symbol.ancestorTypes {
				decl.AncestorTypes = append(decl.AncestorTypes, typeRefFromType(ancestorType, decl.Base.Span))
			}
			decl.Base.ResolvedDeclaration = base.declarationSpan
			for name, field := range base.fields {
				field.typeInfo = substituteNativeTypeParameters(field.typeInfo, baseBindings)
				symbol.fields[name] = field
			}
			for name, method := range base.methods {
				if !method.static {
					method.typeInfo = c.substituteNativeMethodOwnerTypeParameters(method.typeInfo, baseBindings)
				}
				symbol.methods[name] = method
			}
			for name := range base.implements {
				symbol.implements[name] = true
			}
			for _, implemented := range base.implementedTypes {
				symbol.implementedTypes = append(symbol.implementedTypes, substituteNativeTypeParameters(implemented, baseBindings))
			}
			symbol.goImplements = append(symbol.goImplements, base.goImplements...)
		}
	}
	declareField := func(name string, typeRef ast.TypeRef, visibility ast.Visibility, span, declarationSpan source.Span, setGoName func(string)) {
		if name == "__kinmokuseiRoot" {
			c.report(span, fmt.Sprintf("field %q is reserved for class identity", name))
			return
		}
		if method, exists := symbol.methods[name]; exists {
			c.report(span, fmt.Sprintf("field %q conflicts with method declared by class %s", name, method.declaringClass))
			return
		}
		if symbol.base != "" && name == symbol.base {
			c.report(span, fmt.Sprintf("field %q conflicts with the embedded base class", name))
			return
		}
		reservedVirtualField := name == "__kinmokusei"+decl.Name+"Self"
		for _, inheritedMethod := range symbol.methods {
			if inheritedMethod.virtualOwner != "" && name == "__kinmokusei"+inheritedMethod.virtualOwner+"Self" {
				reservedVirtualField = true
			}
		}
		if reservedVirtualField {
			c.report(span, fmt.Sprintf("field %q is reserved for virtual dispatch", name))
			return
		}
		if _, exists := symbol.fields[name]; exists {
			c.report(span, fmt.Sprintf("duplicate field %q", name))
			return
		}
		goName := memberGoName(name, visibility)
		setGoName(goName)
		fieldType := c.resolveType(typeRef)
		c.rejectResultValueType(fieldType, typeRef.Span, "fields")
		c.rejectTaskAPIType(fieldType, typeRef.Span, "class fields")
		symbol.fields[name] = fieldSymbol{typeInfo: fieldType, visibility: visibility, goName: goName, declarationSpan: declarationSpan, declaringClass: decl.Name}
	}
	for i := range decl.Fields {
		field := &decl.Fields[i]
		declareField(field.Name, field.Type, field.Visibility, field.Span, field.NameSpan, func(name string) { field.GoName = name })
	}
	if decl.Constructor != nil {
		c.validateLabels(decl.Constructor.Body)
		symbol.constructor = make([]Type, len(decl.Constructor.Parameters))
		for i := range decl.Constructor.Parameters {
			parameter := &decl.Constructor.Parameters[i]
			resolved := c.resolveType(parameter.Type)
			c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
			c.rejectTaskAPIType(resolved, parameter.Type.Span, "constructor parameters")
			symbol.constructor[i] = c.callableParameterType(*parameter, resolved)
			if parameter.IsField {
				declareField(parameter.Name, parameter.Type, parameter.Visibility, parameter.Span, declarationNameSpan(parameter.Name, parameter.Span), func(string) {})
			}
		}
		symbol.constructorVariadic = hasVariadicParameter(decl.Constructor.Parameters)
	}
	for _, method := range decl.Methods {
		c.validateLabels(method.Body)
		for _, parameter := range method.TypeParameters {
			if _, exists := symbol.typeParamScope[parameter.Name]; exists {
				c.report(parameter.Span, fmt.Sprintf("generic class method type parameter %q conflicts with a class type parameter", parameter.Name))
			}
		}
		methodTypeParameters, methodScope := c.declareTypeParameters(method.TypeParameters, "generic class method")
		c.methodTypeParameters[method] = methodScope
		c.pushTypeParameterScope(methodScope)
		parameters := make([]Type, len(method.Parameters))
		for i, parameter := range method.Parameters {
			resolved := c.resolveType(parameter.Type)
			c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
			c.rejectTaskAPIType(resolved, parameter.Type.Span, "method parameters")
			parameters[i] = c.callableParameterType(parameter, resolved)
		}
		result := c.resolveType(method.ReturnType)
		c.popTypeParameterScope()
		c.rejectTaskAPIType(result, method.ReturnType.Span, "method return types")
		method.GoName = memberGoName(method.Name, method.Visibility)
		inherited, replaces := symbol.methods[method.Name]
		if field, conflicts := symbol.fields[method.Name]; conflicts {
			c.report(method.Span, fmt.Sprintf("method %q conflicts with field declared by class %s", method.Name, field.declaringClass))
		}
		if method.Static && (method.Virtual || method.Override) {
			c.report(method.Span, "static methods cannot be virtual or override")
		}
		if len(method.TypeParameters) != 0 && (method.Virtual || method.Override || method.Final) {
			c.report(method.Span, "generic methods cannot be virtual, override, or final because Go method sets cannot represent method type parameters")
		}
		if method.Virtual && method.Override {
			c.report(method.Span, "override already remains virtual; remove the virtual modifier")
		}
		if method.Final && !method.Override {
			c.report(method.Span, "final methods must override an inherited virtual method")
		}
		if method.Virtual && method.Visibility == ast.Private {
			c.report(method.Span, "virtual methods must be public or protected")
		}
		methodType := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result}
		if len(methodTypeParameters) != 0 {
			methodType.TypeParameters = append(methodType.TypeParameters, methodTypeParameters...)
		}
		if method.Static && len(symbol.typeParameters) != 0 {
			methodType.TypeParameters = append(methodType.TypeParameters, symbol.typeParameters...)
		}
		if len(methodType.TypeParameters) != 0 {
			methodType.Generic = true
		}
		virtualOwner := ""
		if replaces && inherited.declaringClass == decl.Name {
			c.report(method.Span, fmt.Sprintf("duplicate method %q", method.Name))
			continue
		}
		if replaces {
			switch {
			case !method.Override:
				c.report(method.Span, fmt.Sprintf("method %q replaces inherited method from %s; add override", method.Name, inherited.declaringClass))
			case inherited.final:
				c.report(method.Span, fmt.Sprintf("method %q in %s is final and cannot be overridden", method.Name, inherited.declaringClass))
			case inherited.static:
				c.report(method.Span, fmt.Sprintf("static method %q cannot be overridden", method.Name))
			case !inherited.virtual:
				c.report(method.Span, fmt.Sprintf("method %q in %s is not virtual", method.Name, inherited.declaringClass))
			case method.Static:
				c.report(method.Span, fmt.Sprintf("override method %q cannot be static", method.Name))
			case method.Visibility != inherited.visibility:
				c.report(method.Span, fmt.Sprintf("override method %q must preserve inherited visibility", method.Name))
			case !exactType(methodType, inherited.typeInfo):
				c.report(method.Span, fmt.Sprintf("override method %q has an incompatible signature", method.Name))
			}
			virtualOwner = inherited.virtualOwner
		} else if method.Override {
			c.report(method.Span, fmt.Sprintf("method %q has override but no inherited method", method.Name))
		}
		if method.Virtual && virtualOwner == "" {
			virtualOwner = decl.Name
		}
		method.VirtualOwner = virtualOwner
		symbol.methods[method.Name] = methodSymbol{
			typeInfo: methodType, visibility: method.Visibility, static: method.Static, goName: method.GoName,
			declarationSpan: method.NameSpan, declaringClass: decl.Name,
			virtual: method.Virtual || method.Override, final: method.Final, virtualOwner: virtualOwner,
		}
	}
	owners := map[string]bool{}
	for _, method := range symbol.methods {
		if method.virtualOwner != "" {
			owners[method.virtualOwner] = true
		}
	}
	for owner := range owners {
		decl.VirtualOwners = append(decl.VirtualOwners, owner)
	}
	sort.Strings(decl.VirtualOwners)
	for _, implemented := range decl.Implements {
		if implemented.IsArray() || implemented.IsFunction() {
			c.report(implemented.Span, "implements expects an interface name")
			continue
		}
		if implemented.Qualifier != "" || implemented.Go || implemented.Name == "error" {
			contract := c.resolveType(implemented)
			goInterface := underlyingGoInterface(contract.GoType)
			if contract.Kind == Invalid {
				continue
			}
			if goInterface == nil {
				c.report(implemented.Span, fmt.Sprintf("Go type %s is not an interface", contract.String()))
				continue
			}
			duplicate := false
			for _, existing := range symbol.goImplements {
				if gotypes.Identical(existing, contract.GoType) {
					duplicate = true
					break
				}
			}
			if duplicate {
				c.report(implemented.Span, fmt.Sprintf("duplicate implemented Go interface %s", contract.String()))
				continue
			}
			symbol.goImplements = append(symbol.goImplements, contract.GoType)
			c.validateGoInterfaceImplementation(decl.Name, symbol, contract, goInterface, implemented.Span)
			continue
		}
		if _, exists := c.interfaces[implemented.Name]; !exists {
			c.report(implemented.Span, fmt.Sprintf("unknown interface %q", implemented.Name))
			continue
		}
		contractType := c.resolveType(implemented)
		if contractType.Kind == Invalid {
			continue
		}
		if contractType.Kind != Interface {
			c.report(implemented.Span, fmt.Sprintf("unknown interface %q", implemented.Name))
			continue
		}
		contract := c.interfaces[implemented.Name]
		contractName := contractType.String()
		if symbol.implements[contractName] {
			c.report(implemented.Span, fmt.Sprintf("duplicate implemented interface %q", contractName))
			continue
		}
		symbol.implements[contractName] = true
		symbol.implementedTypes = append(symbol.implementedTypes, contractType)
		bindings := nativeInterfaceBindings(contract, contractType)
		for name, required := range contract.methods {
			requiredType := substituteNativeTypeParameters(required.typeInfo, bindings)
			actual, exists := symbol.methods[name]
			if required.goInterfaceMethod {
				actual, exists = classMethodByGoName(symbol, required.goName)
			}
			switch {
			case !exists:
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: missing method %s", decl.Name, contractName, name))
			case actual.visibility != ast.Public:
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: method %s must be public", decl.Name, contractName, name))
			case actual.static:
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: method %s cannot be static", decl.Name, contractName, name))
			case !exactType(actual.typeInfo, requiredType):
				c.report(decl.Span, fmt.Sprintf("class %s does not implement %s: method %s has an incompatible signature", decl.Name, contractName, name))
			}
		}
	}
}

func (c *Checker) checkClass(decl *ast.ClassDecl) {
	previousClass := c.currentClass
	previousInConstructor := c.inConstructor
	defer func() {
		c.inConstructor = previousInConstructor
	}()
	c.currentClass = decl.Name
	class := c.classes[decl.Name]
	if class != nil {
		c.pushTypeParameterScope(class.typeParamScope)
		defer c.popTypeParameterScope()
	}
	thisType := Type{Kind: Class, Name: decl.Name}
	if class != nil {
		thisType.TypeArguments = append([]Type(nil), class.typeParameters...)
	}
	c.checkClassFieldInitializers(decl)
	if decl.Constructor == nil {
		if class := c.classes[decl.Name]; class != nil && class.base != "" {
			if base := c.classes[class.base]; base != nil && len(base.constructor) != 0 {
				c.report(decl.Span, fmt.Sprintf("class %s needs a constructor that calls super(...) because %s expects %d arguments", decl.Name, class.base, len(base.constructor)))
			}
		}
	}
	if decl.Constructor != nil {
		c.validateSuperConstructorPlacement(decl)
		previousMemberFlow := c.memberFlow
		c.memberFlow = map[memberFlowKey]memberFlowState{}
		c.inConstructor = true
		c.pushScope()
		c.declareLocal("this", thisType, true, nil, decl.Constructor.Span)
		for _, parameter := range decl.Constructor.Parameters {
			t := c.resolveType(parameter.Type)
			c.rejectResultValueType(t, parameter.Type.Span, "parameters")
			c.declareLocal(parameter.Name, t, false, nil, parameter.Span)
		}
		previousControl := c.enterCallableControl()
		c.result = builtins["void"]
		c.checkBlock(decl.Constructor.Body, false)
		c.callableControlState = previousControl
		c.popScope()
		c.memberFlow = previousMemberFlow
	}
	c.inConstructor = false
	c.checkClassFieldInitialization(decl)
	for _, method := range decl.Methods {
		c.pushTypeParameterScope(c.methodTypeParameters[method])
		previousMemberFlow := c.memberFlow
		c.memberFlow = map[memberFlowKey]memberFlowState{}
		c.pushScope()
		if !method.Static {
			c.declareLocal("this", thisType, true, nil, method.Span)
		}
		for _, parameter := range method.Parameters {
			t := c.resolveType(parameter.Type)
			c.rejectResultValueType(t, parameter.Type.Span, "parameters")
			c.declareLocal(parameter.Name, t, false, nil, parameter.Span)
		}
		previousControl := c.enterCallableControl()
		c.result = c.resolveType(method.ReturnType)
		c.checkBlock(method.Body, false)
		if c.result.Kind != Void && !definitelyReturns(method.Body) {
			c.report(method.Span, fmt.Sprintf("method %q may complete without returning %s", method.Name, c.result.String()))
		}
		c.callableControlState = previousControl
		c.popScope()
		c.memberFlow = previousMemberFlow
		c.popTypeParameterScope()
	}
	c.currentClass = previousClass
}

func (c *Checker) validateSuperConstructorPlacement(decl *ast.ClassDecl) {
	class := c.classes[decl.Name]
	if class == nil {
		return
	}
	firstIsSuper := false
	for index, statement := range decl.Constructor.Body.Statements {
		expression, ok := statement.(*ast.ExpressionStmt)
		if !ok {
			continue
		}
		call, ok := expression.Value.(*ast.CallExpr)
		if !ok {
			continue
		}
		name, ok := call.Callee.(*ast.IdentifierExpr)
		if !ok || name.Name != "super" {
			continue
		}
		if index == 0 {
			firstIsSuper = true
		} else {
			c.report(call.Span, "super constructor call must be the first statement")
		}
	}
	if class.base == "" {
		return
	}
	base := c.classes[class.base]
	if base != nil && len(base.constructor) != 0 && !firstIsSuper {
		c.report(decl.Constructor.Span, fmt.Sprintf("constructor for %s must call super(...) first because %s expects %d arguments", decl.Name, class.base, len(base.constructor)))
	}
}

func (c *Checker) applyClassUpcast(slot *ast.Expression, expected, actual Type) {
	targetClass, actualClass := expected, actual
	if targetClass.Kind == Nullable && targetClass.Element != nil {
		targetClass = *targetClass.Element
	}
	if actualClass.Kind == Nullable && actualClass.Element != nil {
		actualClass = *actualClass.Element
	}
	if targetClass.Kind == Class && actualClass.Kind == Class && targetClass.Name != actualClass.Name {
		if ancestor, ok := c.classAncestorType(actualClass, targetClass.Name); ok && exactType(targetClass, ancestor) {
			*slot = &ast.ClassUpcastExpr{
				Value: *slot, SourceClass: actualClass.Name, TargetClass: targetClass.Name,
				SourceType: typeRefFromType(actualClass, (*slot).GetSpan()), TargetType: typeRefFromType(targetClass, (*slot).GetSpan()), Span: (*slot).GetSpan(),
			}
		}
	}
}

func (c *Checker) classAncestorType(value Type, baseName string) (Type, bool) {
	class := c.classes[value.Name]
	if class == nil {
		return Type{}, false
	}
	bindings := nativeClassBindings(class, value)
	for index, ancestor := range class.ancestors {
		if ancestor == baseName && index < len(class.ancestorTypes) {
			return substituteNativeTypeParameters(class.ancestorTypes[index], bindings), true
		}
	}
	return Type{}, false
}

func (c *Checker) classExtends(className, baseName string) bool {
	class := c.classes[className]
	if class == nil {
		return false
	}
	for _, ancestor := range class.ancestors {
		if ancestor == baseName {
			return true
		}
	}
	return false
}

func (c *Checker) canAccessClassMember(visibility ast.Visibility, declaringClass string) bool {
	switch visibility {
	case ast.Public:
		return true
	case ast.Private:
		return c.currentClass == declaringClass
	case ast.Protected:
		return c.currentClass == declaringClass || (c.currentClass != "" && c.classExtends(c.currentClass, declaringClass))
	default:
		return false
	}
}

func (c *Checker) reportInaccessibleClassMember(span source.Span, kind, name string, visibility ast.Visibility) {
	label := "private"
	if visibility == ast.Protected {
		label = "protected"
	}
	c.report(span, fmt.Sprintf("%s %q is %s", kind, name, label))
}
