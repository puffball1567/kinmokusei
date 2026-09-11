package sema

import (
	"fmt"
	gotoken "go/token"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) declareFunctionTypeParameters(decl *ast.FunctionDecl) ([]Type, map[string]Type) {
	if len(decl.TypeParameters) == 0 {
		return nil, nil
	}
	if decl.Name == "main" || decl.Name == "init" {
		c.report(decl.NameSpan, fmt.Sprintf("function %q cannot declare type parameters", decl.Name))
	}
	return c.declareTypeParameters(decl.TypeParameters, "generic function")
}

func (c *Checker) declareTypeParameters(parameters []ast.TypeParameter, context string) ([]Type, map[string]Type) {
	return c.declareTypeParametersWithComparable(parameters, context, nil)
}

func (c *Checker) declareDefinedTypeParameters(declaration *ast.TypeDecl) ([]Type, map[string]Type) {
	comparable := map[string]bool{}
	collectComparableTypeParameters(declaration.Underlying, comparable)
	if !declaration.Alias {
		return c.declareTypeParametersWithComparable(declaration.TypeParameters, "generic defined type", comparable)
	}
	// Generic aliases are expanded at every use site for Go 1.23 and their
	// declarations do not reach generated Go. Loading a Go constraint must not
	// retain an otherwise unused import solely for the erased declaration.
	usage := map[*ast.ImportDecl]bool{}
	for _, byAlias := range c.goPackages {
		for _, imported := range byAlias {
			usage[imported.declaration] = imported.declaration.Used
		}
	}
	parameters, scope := c.declareTypeParametersWithComparable(declaration.TypeParameters, "generic alias", comparable)
	for imported, used := range usage {
		imported.Used = used
	}
	return parameters, scope
}

func (c *Checker) declareTypeParametersWithComparable(parameters []ast.TypeParameter, context string, comparable map[string]bool) ([]Type, map[string]Type) {
	anyConstraint := gotypes.NewInterfaceType(nil, nil)
	anyConstraint.Complete()
	comparableConstraint := gotypes.Universe.Lookup("comparable").Type()
	result := make([]Type, 0, len(parameters))
	scope := make(map[string]Type, len(parameters))
	for _, parameter := range parameters {
		if parameter.Name == "_" {
			c.report(parameter.Span, context+" type parameter name cannot be '_'")
			continue
		}
		if parameter.Name == "any" {
			c.report(parameter.Span, context+" type parameter name cannot be 'any' because native parameters use the Go any constraint")
			continue
		}
		if isBuiltinTypeName(parameter.Name) || parameter.Name == "Map" || parameter.Name == "GoChannel" || parameter.Name == "GoSendChannel" || parameter.Name == "GoReceiveChannel" {
			c.report(parameter.Span, fmt.Sprintf("%s type parameter %q conflicts with a built-in type", context, parameter.Name))
			continue
		}
		if _, duplicate := scope[parameter.Name]; duplicate {
			c.report(parameter.Span, fmt.Sprintf("duplicate %s type parameter %q", context, parameter.Name))
			continue
		}
		constraint := gotypes.Type(anyConstraint)
		if comparable[parameter.Name] {
			constraint = comparableConstraint
		}
		object := gotypes.NewTypeName(gotoken.NoPos, nil, parameter.Name, nil)
		goParameter := gotypes.NewTypeParam(object, constraint)
		typeInfo := Type{Kind: TypeParameter, Name: parameter.Name, GoType: goParameter}
		scope[parameter.Name] = typeInfo
		result = append(result, typeInfo)
	}
	c.completeNativeTypeParameterBounds(parameters, scope, comparable, false)
	return result, scope
}

func (c *Checker) nativeTypeParameterConstraintIsDeferred(ref ast.TypeRef) bool {
	if _, parameter := c.lookupTypeParameter(ref.Name); parameter && ref.Qualifier == "" {
		return false
	}
	if ref.Qualifier != "" || ref.Nullable || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() || ref.IsGoStruct() {
		return false
	}
	symbol := c.interfaces[ref.Name]
	return symbol != nil && c.isTopLevelAllowed(ref.Span, ref.Name) && symbol.constraint && symbol.goNamed != nil && symbol.goNamed.Underlying() == nil
}

func (c *Checker) finalizeDeferredTypeParameterConstraints(program *ast.Program) {
	complete := func(parameters []ast.TypeParameter, types []Type, inferredComparable map[string]bool) {
		byName := make(map[string]Type, len(types))
		for _, parameterType := range types {
			byName[parameterType.Name] = parameterType
		}
		c.completeNativeTypeParameterBounds(parameters, byName, inferredComparable, true)
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			if symbol := c.classes[declaration.Name]; symbol != nil {
				complete(declaration.TypeParameters, symbol.typeParameters, nil)
			}
		case *ast.StructDecl:
			if symbol := c.structs[declaration.Name]; symbol != nil {
				complete(declaration.TypeParameters, symbol.typeParameters, nil)
			}
		case *ast.InterfaceDecl:
			if symbol := c.interfaces[declaration.Name]; symbol != nil {
				complete(declaration.TypeParameters, symbol.typeParameters, nil)
			}
		case *ast.TypeDecl:
			if symbol := c.nativeTypes[declaration.Name]; symbol != nil {
				comparable := map[string]bool{}
				collectComparableTypeParameters(declaration.Underlying, comparable)
				// Erased aliases must not retain imports introduced solely by
				// completing a bound that depended on a forward source constraint.
				usage := map[*ast.ImportDecl]bool{}
				if declaration.Alias {
					for _, byAlias := range c.goPackages {
						for _, imported := range byAlias {
							usage[imported.declaration] = imported.declaration.Used
						}
					}
				}
				complete(declaration.TypeParameters, symbol.typeParameters, comparable)
				for imported, used := range usage {
					imported.Used = used
				}
			}
		}
	}
}

func (c *Checker) resolveNativeTypeParameterConstraint(ref ast.TypeRef) (gotypes.Type, bool) {
	if ref.Qualifier == "" && ref.Name == "comparable" && !ref.Nullable && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() && !ref.IsGoStruct() && len(ref.GenericArguments) == 0 {
		return gotypes.Universe.Lookup("comparable").Type(), true
	}
	if ref.Nullable || ref.IsArray() || ref.IsPointer() || ref.IsFunction() || ref.IsObject() || ref.IsGoStruct() {
		c.report(ref.Span, fmt.Sprintf("native type parameter constraint %s must be a Go interface constraint", formatTypeRefForDiagnostic(ref)))
		return nil, false
	}
	if ref.Qualifier == "" {
		_, parameter := c.lookupTypeParameter(ref.Name)
		if symbol := c.interfaces[ref.Name]; !parameter && symbol != nil && c.isTopLevelAllowed(ref.Span, ref.Name) && symbol.constraint {
			return c.instantiateNativeConstraint(ref, symbol)
		}
	}
	resolved := c.resolveType(ref)
	if resolved.Kind == Invalid {
		return nil, false
	}
	constraint, ok := goTypeOf(resolved)
	if !ok || resolved.Kind == TypeParameter || underlyingGoInterface(constraint) == nil {
		c.report(ref.Span, fmt.Sprintf("native type parameter constraint %s must be a Go interface constraint", formatTypeRefForDiagnostic(ref)))
		return nil, false
	}
	return constraint, true
}

func (c *Checker) nativeTypeArgumentSatisfies(parameter, argument Type, bindings map[gotypes.Type]gotypes.Type) bool {
	goParameter, ok := parameter.GoType.(*gotypes.TypeParam)
	if !ok {
		return true
	}
	bound := substituteConstraintType(goParameter.Constraint(), bindings)
	if bound == nil {
		return false
	}
	constraint, constraintOK := bound.Underlying().(*gotypes.Interface)
	if !constraintOK || constraint.Empty() {
		return true
	}
	argument = defaultLiteralType(argument)
	goArgument, ok := goTypeOf(argument)
	if !ok {
		goArgument, ok = c.goTypeForNativeStorage(argument)
	}
	if ok {
		return gotypes.Satisfies(goArgument, constraint)
	}
	return goParameter.Constraint() == gotypes.Universe.Lookup("comparable").Type() && argument.IsComparable()
}

func (c *Checker) validateNativeTypeArguments(parameters, arguments []Type, refs []ast.TypeRef, fallback source.Span, owner string) bool {
	valid := true
	bindings := make(map[gotypes.Type]gotypes.Type, len(parameters))
	nativeBindings := make(nativeTypeBindings, len(parameters))
	for index, parameter := range parameters {
		if index < len(arguments) {
			nativeBindings[parameter.GoType] = arguments[index]
			if argument, ok := c.goTypeForNativeStorage(defaultLiteralType(arguments[index])); ok {
				bindings[parameter.GoType] = argument
			}
		}
	}
	for index := range parameters {
		if index < len(arguments) {
			if parameter, ok := parameters[index].GoType.(*gotypes.TypeParam); ok {
				if shape, ok := c.parameterRangeShape(parameter); ok && !sameConstraintNullability(substituteNativeTypeParameters(shape, nativeBindings), c.constraintArgumentShape(arguments[index])) {
					span := fallback
					if index < len(refs) {
						span = refs[index].Span
					}
					c.report(span, fmt.Sprintf("nullable type information does not match %s type parameter constraint for %s", parameters[index].Name, owner))
					valid = false
					continue
				}
			}
		}
		if index >= len(arguments) || c.nativeTypeArgumentSatisfies(parameters[index], arguments[index], bindings) {
			continue
		}
		span := fallback
		if index < len(refs) {
			span = refs[index].Span
		}
		c.report(span, fmt.Sprintf("type %s does not satisfy %s type parameter constraint for %s", arguments[index].String(), parameters[index].Name, owner))
		valid = false
	}
	return valid
}

func collectComparableTypeParameters(ref ast.TypeRef, result map[string]bool) {
	if ref.Qualifier == "" && ref.Name == "Map" && len(ref.GenericArguments) == 2 {
		collectTypeParametersRequiringComparability(ref.GenericArguments[0], result)
	}
	if ref.Element != nil {
		collectComparableTypeParameters(*ref.Element, result)
	}
	if ref.Pointee != nil {
		collectComparableTypeParameters(*ref.Pointee, result)
	}
	for _, argument := range ref.GenericArguments {
		collectComparableTypeParameters(argument, result)
	}
	for _, parameter := range ref.Parameters {
		collectComparableTypeParameters(parameter, result)
	}
	if ref.Return != nil {
		collectComparableTypeParameters(*ref.Return, result)
	}
	for _, field := range ref.ObjectFields {
		collectComparableTypeParameters(field.Type, result)
	}
}

func collectTypeParametersRequiringComparability(ref ast.TypeRef, result map[string]bool) {
	if ref.IsPointer() || ref.IsSlice() || ref.Name == "Map" || ref.IsFunction() || ref.Object || ref.GoStruct {
		return
	}
	if len(ref.GenericArguments) == 0 && ref.Element == nil {
		result[ref.Name] = true
		return
	}
	if ref.Element != nil {
		collectTypeParametersRequiringComparability(*ref.Element, result)
	}
	for _, argument := range ref.GenericArguments {
		collectTypeParametersRequiringComparability(argument, result)
	}
}

func (c *Checker) pushTypeParameterScope(scope map[string]Type) {
	c.typeParameterScopes = append(c.typeParameterScopes, scope)
}

func (c *Checker) popTypeParameterScope() {
	c.typeParameterScopes = c.typeParameterScopes[:len(c.typeParameterScopes)-1]
}

func (c *Checker) lookupTypeParameter(name string) (Type, bool) {
	for index := len(c.typeParameterScopes) - 1; index >= 0; index-- {
		parameter, ok := c.typeParameterScopes[index][name]
		if ok {
			return parameter, true
		}
	}
	return Type{}, false
}

func callableTypeForFunction(function functionSymbol) Type {
	result := function.result
	return Type{
		Kind: Function, Name: "function", Parameters: function.parameters,
		TypeParameters: function.typeParameters, Generic: len(function.typeParameters) != 0, Variadic: function.variadic, Result: &result,
	}
}

func hasVariadicParameter(parameters []ast.Parameter) bool {
	return len(parameters) != 0 && parameters[len(parameters)-1].Variadic
}

func (c *Checker) callableParameterType(parameter ast.Parameter, resolved Type) Type {
	if !parameter.Variadic {
		return resolved
	}
	if resolved.Kind != Array || resolved.Element == nil {
		c.report(parameter.Type.Span, "rest parameter type must be a slice")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return *resolved.Element
}
