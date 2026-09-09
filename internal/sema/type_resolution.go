package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) resolveTypeThroughNativeIndirection(ref ast.TypeRef) Type {
	c.nativeTypeIndirectionDepth++
	defer func() { c.nativeTypeIndirectionDepth-- }()
	return c.resolveType(ref)
}

func (c *Checker) resolveType(ref ast.TypeRef) Type {
	if ref.GoInterface && !ref.Nullable {
		result := Type{Kind: GoInterface, Name: "interface{}"}
		for _, method := range ref.ObjectFields {
			result.GoMethods = append(result.GoMethods, GoInterfaceMethod{Name: method.Name, Type: c.resolveType(method.Type)})
		}
		if converted, ok := goTypeOf(result); ok {
			result.GoType = converted
			result.Name = goTypeDisplayName(converted)
		}
		return result
	}
	if len(ref.GoResults) != 0 {
		result := Type{Kind: MultiValue, Name: "multiple values"}
		for _, item := range ref.GoResults {
			result.Results = append(result.Results, c.resolveType(item))
		}
		return result
	}
	if ref.Nullable {
		baseRef := ref
		baseRef.Nullable = false
		base := c.resolveType(baseRef)
		if base.Kind == Invalid {
			return base
		}
		if base.Kind == Nullable {
			c.report(ref.Span, "nullable types cannot be nested")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if !isNullableBaseType(base) {
			c.report(ref.Span, fmt.Sprintf("type %s cannot be nullable because it has no nil representation", base.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Nullable, Name: "nullable", Element: &base}
	}
	if ref.IsPointer() {
		pointee := c.resolveTypeThroughNativeIndirection(*ref.Pointee)
		if pointee.Kind == Invalid {
			return pointee
		}
		if containsTaskType(pointee) {
			c.report(ref.Pointee.Span, "Task cannot be nested inside a pointer type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		goType, ok := goTypeOf(pointee)
		if !ok && pointee.Kind != Struct && pointee.Kind != FixedArray {
			c.report(ref.Span, fmt.Sprintf("type %s cannot be used as a Go pointer target", pointee.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		var pointerType gotypes.Type
		if ok {
			pointerType = gotypes.NewPointer(goType)
		}
		return Type{Kind: GoPointer, Name: "*" + pointee.String(), Element: &pointee, GoType: pointerType, GoQualifier: pointee.GoQualifier}
	}
	if ref.IsArray() {
		element := Type{}
		if ref.IsSlice() {
			element = c.resolveTypeThroughNativeIndirection(*ref.Element)
		} else {
			element = c.resolveType(*ref.Element)
		}
		if containsTaskType(element) {
			c.report(ref.Element.Span, "Task cannot be nested inside an array type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if containsResultType(element) {
			c.report(ref.Element.Span, "Result cannot be nested inside an array type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if ref.IsFixedArray() {
			if element.Kind == Invalid {
				return element
			}
			elementGoType, ok := goTypeOf(element)
			if !ok {
				elementGoType, ok = c.goTypeForNativeStorage(element)
			}
			if !ok && element.Kind != Struct && element.Kind != FixedArray {
				c.report(ref.Element.Span, fmt.Sprintf("type %s cannot be used as a fixed array element", element.String()))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			var arrayType gotypes.Type
			if ok {
				arrayType = gotypes.NewArray(elementGoType, *ref.FixedLength)
			}
			return Type{Kind: FixedArray, Name: "fixed array", Element: &element, Length: *ref.FixedLength, GoType: arrayType}
		}
		return Type{Kind: Array, Name: "array", Element: &element}
	}
	if ref.IsFunction() {
		parameters := make([]Type, len(ref.Parameters))
		for i, parameter := range ref.Parameters {
			resolved := c.resolveTypeThroughNativeIndirection(parameter)
			if ref.Variadic && i == len(ref.Parameters)-1 {
				parameters[i] = c.callableParameterType(ast.Parameter{Type: parameter, Variadic: true}, resolved)
			} else {
				parameters[i] = resolved
			}
		}
		result := c.resolveTypeThroughNativeIndirection(*ref.Return)
		for _, parameter := range parameters {
			if containsTaskType(parameter) {
				c.report(ref.Span, "Task is not supported inside a function type")
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			if containsResultType(parameter) {
				c.report(ref.Span, "Result is not supported inside a function type")
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
		}
		if containsResultType(result) {
			c.report(ref.Span, "Result is not supported inside a function type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if containsTaskType(result) {
			c.report(ref.Span, "Task is not supported inside a function type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: ref.Variadic, Result: &result}
	}
	if ref.IsObject() {
		fields := map[string]Type{}
		fieldNames := map[string]string{}
		for _, field := range ref.ObjectFields {
			if _, duplicate := fields[field.Name]; duplicate {
				c.report(field.Span, fmt.Sprintf("duplicate object type field %q", field.Name))
				continue
			}
			fieldType := c.resolveType(field.Type)
			if fieldType.Kind == Void {
				c.report(field.Type.Span, fmt.Sprintf("object field %q cannot have type void", field.Name))
			}
			if containsResultType(fieldType) {
				c.report(field.Type.Span, fmt.Sprintf("object field %q cannot contain Result", field.Name))
				fieldType = Type{Kind: Invalid, Name: "<invalid>"}
			}
			if containsTaskType(fieldType) {
				c.report(field.Type.Span, fmt.Sprintf("object field %q cannot contain Task", field.Name))
				fieldType = Type{Kind: Invalid, Name: "<invalid>"}
			}
			fields[field.Name] = fieldType
			fieldNames[field.Name] = memberGoName(field.Name, ast.Public)
		}
		return Type{Kind: Object, Name: "object", Fields: fields, FieldNames: fieldNames}
	}
	if ref.Qualifier == "" {
		if len(ref.GenericArguments) == 0 {
			if parameter, ok := c.lookupTypeParameter(ref.Name); ok {
				return parameter
			}
		}
		if named, ok := c.nativeTypes[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
			return c.resolveNativeDefinedType(ref, named)
		}
	}
	if ref.Name == "Result" {
		if len(ref.GenericArguments) != 1 {
			c.report(ref.Span, "Result expects one type argument")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		element := c.resolveType(ref.GenericArguments[0])
		if element.Kind == Invalid {
			return element
		}
		if containsTaskType(element) {
			c.report(ref.GenericArguments[0].Span, "Task cannot be nested inside Result")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if element.Kind == Result {
			c.report(ref.GenericArguments[0].Span, "nested Result types are not supported")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if element.Kind == MultiValue || element.Kind == GoPackage || element.Kind == GoTypeName || element.Kind == Nil {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Result value", element.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Result, Name: "Result", Element: &element}
	}
	if ref.Name == "Task" {
		if len(ref.GenericArguments) != 1 {
			c.report(ref.Span, "Task expects one type argument")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		element := c.resolveType(ref.GenericArguments[0])
		if element.Kind == Invalid {
			return element
		}
		if element.Kind == Task || element.Kind == MultiValue || element.Kind == GoPackage || element.Kind == GoTypeName || element.Kind == Nil || element.Kind == Null {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Task result", element.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Task, Name: "Task", Element: &element}
	}
	if ref.Qualifier == "" && ref.Name == "Map" {
		if len(ref.GenericArguments) != 2 {
			c.report(ref.Span, "Map expects two type arguments")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		key := c.resolveTypeThroughNativeIndirection(ref.GenericArguments[0])
		value := c.resolveTypeThroughNativeIndirection(ref.GenericArguments[1])
		if containsTaskType(key) || containsTaskType(value) {
			c.report(ref.Span, "Task cannot be nested inside a Map type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if containsResultType(key) || containsResultType(value) {
			c.report(ref.Span, "Result cannot be nested inside a Map type")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if key.Kind != Invalid && !key.IsComparable() {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Map key", key.String()))
		}
		return Type{Kind: Map, Name: "Map", Key: &key, Element: &value}
	}
	if ref.Name == "GoChannel" || ref.Name == "GoSendChannel" || ref.Name == "GoReceiveChannel" {
		if len(ref.GenericArguments) != 1 {
			c.report(ref.Span, fmt.Sprintf("%s expects one type argument", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		element := c.resolveTypeThroughNativeIndirection(ref.GenericArguments[0])
		if containsTaskType(element) {
			c.report(ref.GenericArguments[0].Span, "Task cannot be used as a Go channel element")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		elementGoType, ok := c.goTypeForNativeStorage(element)
		if !ok {
			c.report(ref.GenericArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Go channel element", element.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		direction := gotypes.SendRecv
		if ref.Name == "GoSendChannel" {
			direction = gotypes.SendOnly
		} else if ref.Name == "GoReceiveChannel" {
			direction = gotypes.RecvOnly
		}
		return Type{Kind: GoChannel, Name: ref.Name, Element: &element, GoType: gotypes.NewChan(direction, elementGoType), GoQualifier: element.GoQualifier}
	}
	if ref.Qualifier == "" {
		if symbol, ok := c.structs[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
			return c.resolveNativeStructType(ref, symbol)
		}
	}
	if ref.Qualifier != "" {
		imported := c.lookupGoPackage(ref.Span.Path, ref.Qualifier)
		if imported == nil || imported.packageInfo == nil {
			c.report(ref.Span, fmt.Sprintf("unknown Go package alias %q", ref.Qualifier))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object := imported.packageInfo.Scope().Lookup(ref.Name)
		typeName, ok := object.(*gotypes.TypeName)
		if !ok || !typeName.Exported() {
			c.report(ref.Span, fmt.Sprintf("Go package %q has no exported type %q", imported.path, ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		goType := typeName.Type()
		if len(ref.GenericArguments) != 0 {
			parameterCount := goTypeParameterCount(goType)
			if parameterCount == 0 {
				c.report(ref.Span, fmt.Sprintf("Go type %s.%s is not generic", ref.Qualifier, ref.Name))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			if len(ref.GenericArguments) != parameterCount {
				c.report(ref.Span, fmt.Sprintf("Go type %s.%s expects %d type arguments, got %d", ref.Qualifier, ref.Name, parameterCount, len(ref.GenericArguments)))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			typeArguments := make([]gotypes.Type, len(ref.GenericArguments))
			valid := true
			for i := range ref.GenericArguments {
				resolved := c.resolveType(ref.GenericArguments[i])
				argument, ok := goTypeOf(resolved)
				if !ok {
					c.report(ref.GenericArguments[i].Span, fmt.Sprintf("type argument %s cannot be represented as a Go type", resolved.String()))
					valid = false
					continue
				}
				typeArguments[i] = argument
			}
			if !valid {
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			instantiated, instantiateErr := gotypes.Instantiate(nil, goType, typeArguments, c.pendingBoundInstances == nil)
			if instantiateErr != nil {
				c.report(ref.Span, fmt.Sprintf("cannot instantiate Go type %s.%s: %v", ref.Qualifier, ref.Name, instantiateErr))
				return Type{Kind: Invalid, Name: "<invalid>"}
			}
			if c.pendingBoundInstances != nil {
				*c.pendingBoundInstances = append(*c.pendingBoundInstances, boundInstance{origin: goType, arguments: typeArguments, ref: ref})
			}
			goType = instantiated
		}
		result, err := kinmokuseiTypeFromGo(goType)
		if err != nil {
			c.report(ref.Span, fmt.Sprintf("Go type %s.%s is not supported: %v", ref.Qualifier, ref.Name, err))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if !c.allowUnsafeGo && goTypeContainsUnsafePointer(goType, nil) {
			c.report(ref.Span, fmt.Sprintf(`Go type %s.%s uses unsafe.Pointer; set [go.interop] unsafe = "allow" to use it`, ref.Qualifier, ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		imported.declaration.Used = true
		alias := imported.declaration.Alias
		if imported.declaration.ResolvedAlias != "" {
			alias = imported.declaration.ResolvedAlias
		}
		applyGoQualifier(&result, imported.path, alias)
		return result
	}
	if contract, ok := c.interfaces[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
		return c.resolveNativeInterfaceType(ref, contract)
	}
	if class, ok := c.classes[ref.Name]; ok && c.isTopLevelAllowed(ref.Span, ref.Name) {
		return c.resolveNativeClassType(ref, class)
	}
	if len(ref.GenericArguments) != 0 {
		c.report(ref.Span, fmt.Sprintf("generic type %q is not supported", ref.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if t, ok := LookupType(ref.Name); ok {
		return t
	}
	c.report(ref.Span, fmt.Sprintf("unknown type %q", ref.Name))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) resolveNativeClassType(ref ast.TypeRef, symbol *classSymbol) Type {
	want := len(symbol.typeParameters)
	got := len(ref.GenericArguments)
	if want == 0 {
		if got != 0 {
			c.report(ref.Span, fmt.Sprintf("class %s is not generic", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Class, Name: ref.Name}
	}
	if got != want {
		c.report(ref.Span, fmt.Sprintf("generic class %s expects %d type arguments, got %d", ref.Name, want, got))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	arguments := make([]Type, got)
	valid := true
	for index := range ref.GenericArguments {
		arguments[index] = c.resolveType(ref.GenericArguments[index])
		argument := arguments[index]
		if argument.Kind == Invalid {
			valid = false
			continue
		}
		if argument.Kind == Void || argument.Kind == Result || argument.Kind == Task || argument.Kind == MultiValue || argument.Kind == GoPackage || argument.Kind == GoTypeName || argument.Kind == Nil || argument.Kind == Null {
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot be used as a generic class type argument", argument.String()))
			valid = false
		}
	}
	if !valid || !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "generic class "+ref.Name) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: Class, Name: ref.Name, TypeArguments: arguments}
}

func nativeClassBindings(symbol *classSymbol, instantiated Type) nativeTypeBindings {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(nativeTypeBindings, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.GoType] = instantiated.TypeArguments[index]
	}
	return bindings
}

func (c *Checker) resolveNativeStructType(ref ast.TypeRef, symbol *structSymbol) Type {
	want := len(symbol.typeParameters)
	got := len(ref.GenericArguments)
	if want == 0 {
		if got != 0 {
			c.report(ref.Span, fmt.Sprintf("struct %s is not generic", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return symbol.typeInfo
	}
	if got != want {
		c.report(ref.Span, fmt.Sprintf("generic struct %s expects %d type arguments, got %d", ref.Name, want, got))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	arguments := make([]Type, got)
	valid := true
	for index := range ref.GenericArguments {
		arguments[index] = c.resolveType(ref.GenericArguments[index])
		argument := arguments[index]
		if argument.Kind == Invalid {
			valid = false
			continue
		}
		if argument.Kind == Void || argument.Kind == Result || argument.Kind == Task || argument.Kind == MultiValue || argument.Kind == GoPackage || argument.Kind == GoTypeName || argument.Kind == Nil || argument.Kind == Null {
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot be used as a generic struct type argument", argument.String()))
			valid = false
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "generic struct "+ref.Name) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	bindings := make(nativeTypeBindings, want)
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.GoType] = arguments[index]
	}
	result := substituteNativeTypeParameters(symbol.typeInfo, bindings)
	result.TypeParameters = nil
	result.TypeArguments = arguments
	result.Generic = false
	if symbol.typeInfo.GoType != nil {
		if instantiated, ok := c.instantiateNativeStorageType(symbol.goNamed, arguments); ok {
			result.GoType = instantiated
		}
	}
	return result
}

func nativeStructBindings(symbol *structSymbol, instantiated Type) nativeTypeBindings {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(nativeTypeBindings, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.GoType] = instantiated.TypeArguments[index]
	}
	return bindings
}

func nativeDefinedTypeBindings(symbol *nativeTypeSymbol, instantiated Type) nativeTypeBindings {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(nativeTypeBindings, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.GoType] = instantiated.TypeArguments[index]
	}
	return bindings
}

func (c *Checker) nativeDefinedUnderlying(symbol *nativeTypeSymbol, instantiated Type) Type {
	return c.nativeDefinedUnderlyingSeen(symbol, instantiated, map[string]bool{})
}

func (c *Checker) nativeDefinedUnderlyingSeen(symbol *nativeTypeSymbol, instantiated Type, visiting map[string]bool) Type {
	if symbol == nil || visiting[symbol.declaration.Name] {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	visiting[symbol.declaration.Name] = true
	defer delete(visiting, symbol.declaration.Name)
	underlying := substituteNativeTypeParameters(symbol.underlying, nativeDefinedTypeBindings(symbol, instantiated))
	if underlying.Kind != GoNamed || underlying.GoQualifier != "" {
		return underlying
	}
	object := goTypeNameObject(underlying.GoType)
	if object == nil {
		return underlying
	}
	dependency := c.nativeTypes[object.Name()]
	if dependency == nil || dependency.declaration.Alias {
		return underlying
	}
	return c.nativeDefinedUnderlyingSeen(dependency, underlying, visiting)
}

func (c *Checker) resolveNativeInterfaceType(ref ast.TypeRef, symbol *interfaceSymbol) Type {
	if symbol.constraint {
		c.report(ref.Span, fmt.Sprintf("constraint %s can only be used after 'extends' in a type parameter", ref.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	want := len(symbol.typeParameters)
	got := len(ref.GenericArguments)
	if want == 0 {
		if got != 0 {
			c.report(ref.Span, fmt.Sprintf("interface %s is not generic", ref.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return Type{Kind: Interface, Name: ref.Name}
	}
	if got != want {
		c.report(ref.Span, fmt.Sprintf("generic interface %s expects %d type arguments, got %d", ref.Name, want, got))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	arguments := make([]Type, got)
	valid := true
	for index := range ref.GenericArguments {
		arguments[index] = c.resolveType(ref.GenericArguments[index])
		argument := arguments[index]
		if argument.Kind == Invalid {
			valid = false
			continue
		}
		if argument.Kind == Void || argument.Kind == Result || argument.Kind == Task || argument.Kind == MultiValue || argument.Kind == GoPackage || argument.Kind == GoTypeName || argument.Kind == Nil || argument.Kind == Null {
			c.report(ref.GenericArguments[index].Span, fmt.Sprintf("type %s cannot be used as a generic interface type argument", argument.String()))
			valid = false
		}
	}
	if !valid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !c.validateNativeTypeArguments(symbol.typeParameters, arguments, ref.GenericArguments, ref.Span, "generic interface "+ref.Name) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: Interface, Name: ref.Name, TypeArguments: arguments}
}

func nativeInterfaceBindings(symbol *interfaceSymbol, instantiated Type) nativeTypeBindings {
	if symbol == nil || len(symbol.typeParameters) == 0 || len(symbol.typeParameters) != len(instantiated.TypeArguments) {
		return nil
	}
	bindings := make(nativeTypeBindings, len(symbol.typeParameters))
	for index, parameter := range symbol.typeParameters {
		bindings[parameter.GoType] = instantiated.TypeArguments[index]
	}
	return bindings
}

func goTypeParameterCount(goType gotypes.Type) int {
	switch goType := goType.(type) {
	case *gotypes.Named:
		if goType.TypeParams() != nil {
			return goType.TypeParams().Len()
		}
	case *gotypes.Alias:
		if goType.TypeParams() != nil {
			return goType.TypeParams().Len()
		}
	}
	return 0
}
