package sema

import (
	"fmt"
	gotoken "go/token"
	gotypes "go/types"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) declareStruct(decl *ast.StructDecl) {
	symbol := c.structs[decl.Name]
	if symbol == nil {
		fields := map[string]Type{}
		fieldNames := map[string]string{}
		symbol = &structSymbol{
			fields: map[string]fieldSymbol{}, methods: map[string]methodSymbol{}, declarationSpan: decl.NameSpan,
			typeInfo: Type{Kind: Struct, Name: decl.Name, Fields: fields, FieldNames: fieldNames},
		}
		c.structs[decl.Name] = symbol
	}
	c.pushTypeParameterScope(symbol.typeParamScope)
	defer c.popTypeParameterScope()
	for i := range decl.Fields {
		field := &decl.Fields[i]
		if _, exists := symbol.fields[field.Name]; exists {
			c.report(field.Span, fmt.Sprintf("duplicate struct field %q", field.Name))
			continue
		}
		fieldType := c.resolveType(field.Type)
		if fieldType.Kind == Void {
			c.report(field.Type.Span, fmt.Sprintf("struct field %q cannot have type void", field.Name))
		}
		c.rejectResultValueType(fieldType, field.Type.Span, "struct fields")
		c.rejectTaskAPIType(fieldType, field.Type.Span, "struct fields")
		goName := memberGoName(field.Name, field.Visibility)
		field.GoName = goName
		symbol.fields[field.Name] = fieldSymbol{
			typeInfo: fieldType, visibility: field.Visibility, goName: goName, declarationSpan: field.NameSpan,
		}
		symbol.typeInfo.Fields[field.Name] = fieldType
		symbol.typeInfo.FieldNames[field.Name] = goName
	}
	for _, method := range decl.Methods {
		c.declareStructMethod(symbol, method)
	}
}

func (c *Checker) declareStructs(program *ast.Program) {
	for _, declaration := range program.Declarations {
		if structure, ok := declaration.(*ast.StructDecl); ok {
			c.declareStruct(structure)
		}
	}
}

func (c *Checker) finalizeNativeStructGoTypes(program *ast.Program) {
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.StructDecl)
		if !ok {
			continue
		}
		symbol := c.structs[decl.Name]
		if symbol == nil || symbol.goNamed == nil {
			continue
		}
		fields := make([]*gotypes.Var, 0, len(decl.Fields))
		tags := make([]string, 0, len(decl.Fields))
		valid := true
		seen := map[string]bool{}
		for _, field := range decl.Fields {
			if seen[field.Name] {
				valid = false
				continue
			}
			seen[field.Name] = true
			stored, exists := symbol.fields[field.Name]
			if !exists {
				valid = false
				continue
			}
			fieldType, ok := c.goTypeForNativeStorage(stored.typeInfo)
			if !ok {
				valid = false
				continue
			}
			fields = append(fields, gotypes.NewField(gotoken.NoPos, nil, stored.goName, fieldType, false))
			tag := ""
			if field.Visibility == ast.Public {
				tag = `json:"` + field.Name + `"`
			}
			tags = append(tags, tag)
		}
		if !valid {
			continue
		}
		symbol.goNamed.SetUnderlying(gotypes.NewStruct(fields, tags))
		symbol.typeInfo.GoType = symbol.goNamed
	}
}

func (c *Checker) goTypeForNativeStorage(value Type) (gotypes.Type, bool) {
	switch value.Kind {
	case Struct:
		symbol := c.structs[value.Name]
		if symbol == nil || symbol.goNamed == nil {
			return nil, false
		}
		return c.instantiateNativeStorageType(symbol.goNamed, value.TypeArguments)
	case Class:
		symbol := c.classes[value.Name]
		if symbol == nil || symbol.goNamed == nil {
			return nil, false
		}
		instantiated, ok := c.instantiateNativeStorageType(symbol.goNamed, value.TypeArguments)
		if !ok {
			return nil, false
		}
		return gotypes.NewPointer(instantiated), true
	case Interface:
		symbol := c.interfaces[value.Name]
		if symbol == nil || symbol.goNamed == nil {
			return nil, false
		}
		return c.instantiateNativeStorageType(symbol.goNamed, value.TypeArguments)
	case Nullable:
		if value.Element == nil {
			return nil, false
		}
		return c.goTypeForNativeStorage(*value.Element)
	case GoPointer:
		if value.Element == nil {
			return nil, false
		}
		element, ok := c.goTypeForNativeStorage(*value.Element)
		if !ok {
			return nil, false
		}
		return gotypes.NewPointer(element), true
	case Array:
		if value.Element == nil {
			return nil, false
		}
		element, ok := c.goTypeForNativeStorage(*value.Element)
		if !ok {
			return nil, false
		}
		return gotypes.NewSlice(element), true
	case FixedArray:
		if value.Element == nil {
			return nil, false
		}
		element, ok := c.goTypeForNativeStorage(*value.Element)
		if !ok {
			return nil, false
		}
		return gotypes.NewArray(element, value.Length), true
	case Map:
		if value.Key == nil || value.Element == nil {
			return nil, false
		}
		key, keyOK := c.goTypeForNativeStorage(*value.Key)
		element, elementOK := c.goTypeForNativeStorage(*value.Element)
		if !keyOK || !elementOK {
			return nil, false
		}
		return gotypes.NewMap(key, element), true
	case Object:
		names := make([]string, 0, len(value.Fields))
		for name := range value.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		fields := make([]*gotypes.Var, len(names))
		tags := make([]string, len(names))
		for index, name := range names {
			fieldType, ok := c.goTypeForNativeStorage(value.Fields[name])
			if !ok {
				return nil, false
			}
			goName := value.FieldNames[name]
			if goName == "" {
				goName = name
			}
			fields[index] = gotypes.NewField(gotoken.NoPos, nil, goName, fieldType, false)
			tags[index] = `json:"` + name + `"`
		}
		return gotypes.NewStruct(fields, tags), true
	case Function:
		parameters := make([]*gotypes.Var, len(value.Parameters))
		for index, parameter := range value.Parameters {
			parameterType, ok := c.goTypeForNativeStorage(parameter)
			if !ok {
				return nil, false
			}
			if value.Variadic && index == len(value.Parameters)-1 {
				parameterType = gotypes.NewSlice(parameterType)
			}
			parameters[index] = gotypes.NewVar(gotoken.NoPos, nil, "", parameterType)
		}
		if value.Result == nil {
			return nil, false
		}
		var results []*gotypes.Var
		switch value.Result.Kind {
		case Void:
		case MultiValue:
			results = make([]*gotypes.Var, len(value.Result.Results))
			for index, result := range value.Result.Results {
				resultType, ok := c.goTypeForNativeStorage(result)
				if !ok {
					return nil, false
				}
				results[index] = gotypes.NewVar(gotoken.NoPos, nil, "", resultType)
			}
		default:
			resultType, ok := c.goTypeForNativeStorage(*value.Result)
			if !ok {
				return nil, false
			}
			results = []*gotypes.Var{gotypes.NewVar(gotoken.NoPos, nil, "", resultType)}
		}
		return gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(parameters...), gotypes.NewTuple(results...), value.Variadic), true
	case GoChannel:
		if value.Element == nil {
			return nil, false
		}
		element, ok := c.goTypeForNativeStorage(*value.Element)
		if !ok {
			return nil, false
		}
		direction := gotypes.SendRecv
		switch value.Name {
		case "GoSendChannel":
			direction = gotypes.SendOnly
		case "GoReceiveChannel":
			direction = gotypes.RecvOnly
		}
		return gotypes.NewChan(direction, element), true
	default:
		return goTypeOf(value)
	}
}

func (c *Checker) instantiateNativeStorageType(named *gotypes.Named, arguments []Type) (gotypes.Type, bool) {
	if len(arguments) == 0 {
		return named, true
	}
	goArguments := make([]gotypes.Type, len(arguments))
	for index, argument := range arguments {
		resolved, ok := c.goTypeForNativeStorage(argument)
		if !ok {
			return nil, false
		}
		goArguments[index] = resolved
	}
	instantiated, err := gotypes.Instantiate(nil, named, goArguments, true)
	return instantiated, err == nil
}

func (c *Checker) declareStructMethod(symbol *structSymbol, method *ast.MethodDecl) {
	if _, exists := symbol.fields[method.Name]; exists {
		c.report(method.Span, fmt.Sprintf("struct member %q conflicts with a field", method.Name))
		return
	}
	if _, exists := symbol.methods[method.Name]; exists {
		c.report(method.Span, fmt.Sprintf("duplicate struct method %q", method.Name))
		return
	}
	var methodTypeParameters []Type
	if !method.External {
		for _, parameter := range method.TypeParameters {
			if _, exists := symbol.typeParamScope[parameter.Name]; exists {
				c.report(parameter.Span, fmt.Sprintf("generic struct method type parameter %q conflicts with a struct type parameter", parameter.Name))
			}
		}
		var methodScope map[string]Type
		methodTypeParameters, methodScope = c.declareTypeParameters(method.TypeParameters, "generic struct method")
		c.methodTypeParameters[method] = methodScope
		c.pushTypeParameterScope(methodScope)
		defer c.popTypeParameterScope()
	}
	parameters := make([]Type, len(method.Parameters))
	for i, parameter := range method.Parameters {
		resolved := c.resolveType(parameter.Type)
		if resolved.Kind == Void {
			c.report(parameter.Type.Span, "parameters cannot have type void")
		}
		c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
		c.rejectTaskAPIType(resolved, parameter.Type.Span, "method parameters")
		parameters[i] = c.callableParameterType(parameter, resolved)
	}
	result := c.resolveType(method.ReturnType)
	c.rejectTaskAPIType(result, method.ReturnType.Span, "method return types")
	method.GoName = memberGoName(method.Name, method.Visibility)
	methodType := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result}
	if len(methodTypeParameters) != 0 {
		methodType.TypeParameters = methodTypeParameters
		methodType.Generic = true
	}
	symbol.methods[method.Name] = methodSymbol{
		typeInfo:   methodType,
		visibility: method.Visibility, pointerReceiver: method.PointerReceiver,
		goName: method.GoName, declarationSpan: method.NameSpan,
	}
}

func (c *Checker) declareReceiverMethods(program *ast.Program) {
	for _, declaration := range program.Declarations {
		method, ok := declaration.(*ast.MethodDecl)
		if !ok {
			continue
		}
		receiverRef := method.ReceiverType
		nullableReceiver := receiverRef.Nullable
		method.PointerReceiver = receiverRef.IsPointer()
		if method.PointerReceiver && receiverRef.Pointee != nil {
			receiverRef = *receiverRef.Pointee
		}
		if nullableReceiver || receiverRef.Nullable || receiverRef.Qualifier != "" || receiverRef.Name == "" || receiverRef.IsArray() || receiverRef.IsPointer() || receiverRef.IsFunction() || receiverRef.IsObject() {
			c.report(method.ReceiverType.Span, "external method receiver must be a native struct value or pointer, or a defined type value or pointer")
			continue
		}
		structure := c.structs[receiverRef.Name]
		if structure != nil {
			if structure.declarationSpan.Path != method.Span.Path {
				c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s must be declared in the same module", receiverRef.Name))
				continue
			}
			typeParameterScope, valid := c.prepareReceiverTypeParameters(method, structure.typeParameters, receiverRef.GenericArguments, "generic struct")
			if !valid {
				continue
			}
			c.receiverTypeParameters[method] = typeParameterScope
			c.pushTypeParameterScope(typeParameterScope)
			c.declareStructMethod(structure, method)
			c.popTypeParameterScope()
			continue
		}
		named := c.nativeTypes[receiverRef.Name]
		if named == nil {
			c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s is not a native struct or defined type", formatTypeRefForDiagnostic(method.ReceiverType)))
			continue
		}
		if named.declaration.NameSpan.Path != method.Span.Path {
			c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s must be declared in the same module", receiverRef.Name))
			continue
		}
		if named.declaration.Alias {
			c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s is an alias; methods require a distinct defined type", receiverRef.Name))
			continue
		}
		typeParameterScope, valid := c.prepareReceiverTypeParameters(method, named.typeParameters, receiverRef.GenericArguments, "generic defined type")
		if !valid {
			continue
		}
		c.receiverTypeParameters[method] = typeParameterScope
		c.pushTypeParameterScope(typeParameterScope)
		resolved := c.resolveNativeType(named)
		if resolved.Kind == Invalid {
			c.popTypeParameterScope()
			continue
		}
		if base, ok := resolved.GoType.(*gotypes.Named); ok {
			switch base.Underlying().(type) {
			case *gotypes.Pointer, *gotypes.Interface:
				c.report(method.ReceiverType.Span, fmt.Sprintf("defined type %s has a pointer or interface underlying type and cannot declare Go receiver methods", receiverRef.Name))
				c.popTypeParameterScope()
				continue
			}
		}
		c.declareNativeTypeMethod(named, method)
		c.popTypeParameterScope()
	}
}

func (c *Checker) prepareReceiverTypeParameters(method *ast.MethodDecl, targetParameters []Type, receiverArguments []ast.TypeRef, targetKind string) (map[string]Type, bool) {
	if len(targetParameters) == 0 {
		if len(method.TypeParameters) != 0 || len(receiverArguments) != 0 {
			c.report(method.ReceiverType.Span, fmt.Sprintf("non-generic receiver type %s cannot declare receiver type parameters", formatTypeRefForDiagnostic(method.ReceiverType)))
			return nil, false
		}
		return nil, true
	}
	valid := true
	if len(method.TypeParameters) != len(targetParameters) {
		c.report(method.NameSpan, fmt.Sprintf("external method on %s %s requires %d receiver type parameters, got %d", targetKind, formatTypeRefForDiagnostic(method.ReceiverType), len(targetParameters), len(method.TypeParameters)))
		valid = false
	}
	if len(receiverArguments) != len(targetParameters) {
		c.report(method.ReceiverType.Span, fmt.Sprintf("external method receiver %s requires %d type arguments, got %d", formatTypeRefForDiagnostic(method.ReceiverType), len(targetParameters), len(receiverArguments)))
		valid = false
	}
	_, declared := c.declareTypeParameters(method.TypeParameters, "generic receiver method")
	scope := make(map[string]Type, len(declared))
	for index, parameter := range method.TypeParameters {
		if _, exists := declared[parameter.Name]; !exists || index >= len(targetParameters) {
			continue
		}
		// Semantically use the declaration's parameter identity so existing
		// generic member substitution remains positional. The source binder may
		// use a different name; code generation retains that spelling from its
		// TypeRef while checking uses the target parameter and its constraint.
		scope[parameter.Name] = targetParameters[index]
	}
	for index, argument := range receiverArguments {
		if index >= len(method.TypeParameters) {
			break
		}
		parameter := method.TypeParameters[index]
		if argument.Name != parameter.Name || argument.Qualifier != "" || argument.Nullable || argument.IsArray() || argument.IsPointer() || argument.IsFunction() || argument.IsObject() || len(argument.GenericArguments) != 0 {
			c.report(argument.Span, fmt.Sprintf("receiver type argument %d must be receiver type parameter %s", index+1, parameter.Name))
			valid = false
		}
	}
	return scope, valid
}

func (c *Checker) declareNativeTypeMethod(symbol *nativeTypeSymbol, method *ast.MethodDecl) {
	if symbol == nil || c.resolveNativeType(symbol).Kind == Invalid {
		return
	}
	method.GoName = memberGoName(method.Name, method.Visibility)
	methodUnderlying := c.nativeDefinedUnderlying(symbol, symbol.typeInfo)
	if methodUnderlying.Kind == Struct {
		if structure := c.structs[methodUnderlying.Name]; structure != nil {
			for fieldName, field := range structure.fields {
				if field.goName == method.GoName {
					c.report(method.Span, fmt.Sprintf("defined type method %q conflicts with underlying struct field %q", method.Name, fieldName))
					return
				}
			}
		}
	}
	if _, exists := symbol.methods[method.Name]; exists {
		c.report(method.Span, fmt.Sprintf("duplicate defined type method %q", method.Name))
		return
	}
	parameters := make([]Type, len(method.Parameters))
	for index, parameter := range method.Parameters {
		resolved := c.resolveType(parameter.Type)
		if resolved.Kind == Void {
			c.report(parameter.Type.Span, "parameters cannot have type void")
		}
		c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
		c.rejectTaskAPIType(resolved, parameter.Type.Span, "method parameters")
		parameters[index] = c.callableParameterType(parameter, resolved)
	}
	result := c.resolveType(method.ReturnType)
	c.rejectTaskAPIType(result, method.ReturnType.Span, "method return types")
	methodType := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result}
	symbol.methods[method.Name] = methodSymbol{
		typeInfo: methodType, visibility: method.Visibility, pointerReceiver: method.PointerReceiver,
		goName: method.GoName, declarationSpan: method.NameSpan,
	}

	named, namedOK := symbol.typeInfo.GoType.(*gotypes.Named)
	signature, signatureOK := goTypeOf(methodType)
	if !namedOK || !signatureOK {
		return
	}
	callable, ok := signature.(*gotypes.Signature)
	if !ok {
		return
	}
	receiverType := gotypes.Type(named)
	if method.PointerReceiver {
		receiverType = gotypes.NewPointer(named)
	}
	receiver := gotypes.NewVar(gotoken.NoPos, nil, method.ReceiverName, receiverType)
	methodSignature := gotypes.NewSignatureType(receiver, nil, nil, callable.Params(), callable.Results(), callable.Variadic())
	named.AddMethod(gotypes.NewFunc(gotoken.NoPos, nil, method.GoName, methodSignature))
}

func (c *Checker) validateStructValueCycles(program *ast.Program) {
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.StructDecl)
		if !ok {
			continue
		}
		for _, field := range decl.Fields {
			symbol := c.structs[decl.Name]
			if symbol == nil {
				continue
			}
			fieldSymbol, exists := symbol.fields[field.Name]
			if exists && c.typeContainsStructByValue(fieldSymbol.typeInfo, decl.Name, map[string]bool{}) {
				c.report(field.Type.Span, fmt.Sprintf("struct %s recursively contains itself by value through field %q; use an explicit pointer, slice, or map indirection", decl.Name, field.Name))
			}
		}
	}
}

func (c *Checker) typeContainsStructByValue(value Type, target string, visiting map[string]bool) bool {
	switch value.Kind {
	case Struct:
		if value.Name == target {
			return true
		}
		if visiting[value.Name] {
			return false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
		if symbol := c.structs[value.Name]; symbol != nil {
			for _, field := range symbol.fields {
				if c.typeContainsStructByValue(field.typeInfo, target, visiting) {
					return true
				}
			}
		}
	case FixedArray:
		return value.Element != nil && c.typeContainsStructByValue(*value.Element, target, visiting)
	case Object:
		for _, field := range value.Fields {
			if c.typeContainsStructByValue(field, target, visiting) {
				return true
			}
		}
	}
	return false
}

func (c *Checker) checkStruct(decl *ast.StructDecl) {
	symbol := c.structs[decl.Name]
	if symbol != nil {
		c.pushTypeParameterScope(symbol.typeParamScope)
		defer c.popTypeParameterScope()
	}
	for _, method := range decl.Methods {
		c.pushTypeParameterScope(c.methodTypeParameters[method])
		c.checkStructMethod(method, "this", decl.Name)
		c.popTypeParameterScope()
	}
}

func (c *Checker) checkStructMethod(method *ast.MethodDecl, receiverName, structName string) {
	c.validateLabels(method.Body)
	valueReceiver := Type{Kind: Struct, Name: structName}
	if method.External {
		receiverRef := method.ReceiverType
		if receiverRef.IsPointer() && receiverRef.Pointee != nil {
			receiverRef = *receiverRef.Pointee
		}
		valueReceiver = c.resolveType(receiverRef)
	} else if symbol := c.structs[structName]; symbol != nil {
		valueReceiver = symbol.typeInfo
		if len(symbol.typeParameters) != 0 {
			valueReceiver.TypeArguments = append([]Type(nil), symbol.typeParameters...)
			valueReceiver.TypeParameters = nil
			valueReceiver.Generic = false
		}
	}
	receiver := valueReceiver
	if method.PointerReceiver {
		receiver = Type{Kind: GoPointer, Name: "*" + structName, Element: &valueReceiver}
	}
	c.pushScope()
	c.declareLocal(receiverName, receiver, true, nil, method.ReceiverNameSpan)
	if method.External {
		scope := c.scopes[len(c.scopes)-1]
		symbol := scope[receiverName]
		symbol.declarationSpan = method.ReceiverNameSpan
		scope[receiverName] = symbol
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
}

func (c *Checker) checkNativeTypeMethod(method *ast.MethodDecl, receiverName, typeName string) {
	c.validateLabels(method.Body)
	symbol := c.nativeTypes[typeName]
	if symbol == nil {
		return
	}
	receiverRef := method.ReceiverType
	if receiverRef.IsPointer() && receiverRef.Pointee != nil {
		receiverRef = *receiverRef.Pointee
	}
	valueReceiver := c.resolveType(receiverRef)
	if valueReceiver.Kind == Invalid || symbol.declaration.Alias {
		return
	}
	receiver := valueReceiver
	if method.PointerReceiver {
		pointerType := gotypes.NewPointer(valueReceiver.GoType)
		receiver = Type{Kind: GoPointer, Name: "*" + typeName, Element: &valueReceiver, GoType: pointerType}
	}
	c.pushScope()
	c.declareLocal(receiverName, receiver, true, nil, method.ReceiverNameSpan)
	scope := c.scopes[len(c.scopes)-1]
	receiverSymbol := scope[receiverName]
	receiverSymbol.declarationSpan = method.ReceiverNameSpan
	scope[receiverName] = receiverSymbol
	for _, parameter := range method.Parameters {
		parameterType := c.resolveType(parameter.Type)
		c.rejectResultValueType(parameterType, parameter.Type.Span, "parameters")
		c.declareLocal(parameter.Name, parameterType, false, nil, parameter.Span)
	}
	previousControl := c.enterCallableControl()
	c.result = c.resolveType(method.ReturnType)
	c.checkBlock(method.Body, false)
	if c.result.Kind != Void && !definitelyReturns(method.Body) {
		c.report(method.Span, fmt.Sprintf("method %q may complete without returning %s", method.Name, c.result.String()))
	}
	c.callableControlState = previousControl
	c.popScope()
}
