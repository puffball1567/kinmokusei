package sema

import (
	"fmt"
	goast "go/ast"
	gotoken "go/token"
	gotypes "go/types"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkNativeGenericCall(expr *ast.CallExpr, callableName string, callable Type) Type {
	if expr.Expanded && !callable.Variadic {
		c.report(expr.Span, fmt.Sprintf("%s is not variadic and cannot receive a spread argument", callableName))
	}
	if len(expr.TypeArguments) > len(callable.TypeParameters) {
		c.report(expr.Span, fmt.Sprintf("%s has %d type parameters, got %d explicit type arguments", callableName, len(callable.TypeParameters), len(expr.TypeArguments)))
	}
	bindings := make(nativeTypeBindings, len(callable.TypeParameters))
	for _, parameter := range callable.TypeParameters {
		// Only the callee's own declarations are inference variables. A type
		// parameter captured from its receiver or caller must remain fixed.
		bindings[parameter.GoType] = Type{Kind: Invalid}
	}
	for index, argument := range expr.TypeArguments {
		resolved := c.resolveType(argument)
		if resolved.Kind == Invalid {
			continue
		}
		if !validNativeTypeArgument(resolved) {
			c.report(argument.Span, fmt.Sprintf("type %s cannot be used as a generic function type argument", resolved.String()))
			continue
		}
		if index < len(callable.TypeParameters) {
			bindings[callable.TypeParameters[index].GoType] = resolved
		}
	}
	actualTypes := c.checkGenericCallbackArguments(expr, callable, bindings)
	numericArguments := c.genericNumericArguments(expr.Arguments, actualTypes)
	deferredNumeric := map[gotypes.Type]int{}
	minimumArguments := len(callable.Parameters)
	if callable.Variadic {
		minimumArguments--
	}
	if expr.Expanded && callable.Variadic && len(expr.Arguments) != len(callable.Parameters) {
		c.report(expr.Span, fmt.Sprintf("spread call to %s expects %d arguments (%d fixed and one slice), got %d", callableName, len(callable.Parameters), minimumArguments, len(expr.Arguments)))
	} else if !expr.Expanded && (len(expr.Arguments) < minimumArguments || (!callable.Variadic && len(expr.Arguments) != len(callable.Parameters))) {
		if callable.Variadic {
			c.report(expr.Span, fmt.Sprintf("%s expects at least %d arguments, got %d", callableName, minimumArguments, len(expr.Arguments)))
		} else {
			c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d", callableName, len(callable.Parameters), len(expr.Arguments)))
		}
	}
	for index := range actualTypes {
		parameterIndex := index
		if callable.Variadic && parameterIndex >= len(callable.Parameters)-1 {
			parameterIndex = len(callable.Parameters) - 1
		}
		if parameterIndex < 0 || parameterIndex >= len(callable.Parameters) {
			continue
		}
		expected := callable.Parameters[parameterIndex]
		if expr.Expanded && callable.Variadic && index == len(actualTypes)-1 {
			element := expected
			expected = Type{Kind: Array, Name: "array", Element: &element}
		}
		if rank := untypedNumericRank(numericArguments[index]); rank != 0 && expected.Kind == TypeParameter {
			if _, inferable := bindings[expected.GoType]; inferable {
				previous, exists := deferredNumeric[expected.GoType]
				if !exists || rank > untypedNumericRank(numericArguments[previous]) {
					deferredNumeric[expected.GoType] = index
				}
				continue
			}
		}
		if err := c.inferNativeTypeArguments(expected, actualTypes[index], bindings); err != nil {
			c.report(expr.Arguments[index].GetSpan(), fmt.Sprintf("cannot infer type arguments for %s from argument %d: %v", callableName, index+1, err))
		}
	}
	c.inferNativeConstraintArguments(callable.TypeParameters, bindings)
	// Typed arguments and their constraints take precedence over constants,
	// regardless of source argument order. Only then choose a default kind.
	for parameter, index := range deferredNumeric {
		if bindings[parameter].Kind == Invalid {
			bindings[parameter] = defaultLiteralType(actualTypes[index])
		}
	}
	c.inferNativeConstraintArguments(callable.TypeParameters, bindings)
	missing := make([]string, 0, len(callable.TypeParameters))
	for _, parameter := range callable.TypeParameters {
		if bindings[parameter.GoType].Kind == Invalid {
			missing = append(missing, parameter.Name)
		}
	}
	if len(missing) != 0 {
		c.report(expr.Span, fmt.Sprintf("cannot infer type argument%s %s for %s; provide explicit type arguments", pluralSuffix(len(missing)), strings.Join(missing, ", "), callableName))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	arguments := make([]Type, len(callable.TypeParameters))
	for index, parameter := range callable.TypeParameters {
		arguments[index] = bindings[parameter.GoType]
	}
	if !c.validateNativeTypeArguments(callable.TypeParameters, arguments, expr.TypeArguments, expr.Span, callableName) {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if len(expr.TypeArguments) < len(arguments) {
		for _, parameter := range callable.Parameters {
			if containsNativeInterface(parameter) {
				expr.ResolvedTypeArguments = make([]ast.TypeRef, len(arguments))
				for index, argument := range arguments {
					expr.ResolvedTypeArguments[index] = typeRefFromType(argument, expr.Span)
				}
				break
			}
		}
	}
	parameters := make([]Type, len(callable.Parameters))
	for index, parameter := range callable.Parameters {
		parameters[index] = substituteNativeTypeParameters(parameter, bindings)
	}
	result := substituteNativeTypeParameters(*callable.Result, bindings)
	instantiated := Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: callable.Variadic, Result: &result}
	c.recordCallSignature(expr, instantiated)
	for index := range actualTypes {
		parameterIndex := index
		if instantiated.Variadic && parameterIndex >= len(parameters)-1 {
			parameterIndex = len(parameters) - 1
		}
		if parameterIndex < 0 || parameterIndex >= len(parameters) {
			continue
		}
		expected := parameters[parameterIndex]
		if expr.Expanded && instantiated.Variadic && index == len(actualTypes)-1 {
			element := expected
			expected = Type{Kind: Array, Name: "array", Element: &element}
		}
		if info := numericArguments[index]; info.Value != nil {
			if target, ok := goTypeOf(expected); ok {
				if err := checkNumericConstantAssignment(info, target); err != nil {
					c.report(expr.Arguments[index].GetSpan(), err.Error())
				}
			}
		}
		c.requireAssignable(expected, actualTypes[index], expr.Arguments[index].GetSpan())
		c.applyClassUpcast(&expr.Arguments[index], expected, actualTypes[index])
	}
	return result
}

func validNativeTypeArgument(value Type) bool {
	switch value.Kind {
	case Invalid, Void, MultiValue, Result, Task, GoPackage, GoTypeName, Nil, Null:
		return false
	default:
		return true
	}
}

func containsNativeInterface(value Type) bool {
	if value.Kind == Interface {
		return true
	}
	switch value.Kind {
	case Nullable, Array, FixedArray, GoPointer, Result, Task, GoChannel, Map, Function:
		for _, nested := range []*Type{value.Element, value.Key, value.Result} {
			if nested != nil && containsNativeInterface(*nested) {
				return true
			}
		}
	}
	for _, group := range [][]Type{value.TypeArguments, value.Parameters} {
		for _, nested := range group {
			if containsNativeInterface(nested) {
				return true
			}
		}
	}
	// Named structs may contain recursive fields, but generic inference only
	// descends their type arguments. Anonymous object fields are structural.
	if value.Kind == Object {
		for _, field := range value.Fields {
			if containsNativeInterface(field) {
				return true
			}
		}
	}
	return false
}

// nativeTypeBindings keys substitutions by declaration identity. Names can be
// reused by inherited methods, generic receivers, and enclosing callables.
type nativeTypeBindings map[gotypes.Type]Type

func (c *Checker) inferNativeTypeArguments(formal, actual Type, bindings nativeTypeBindings) error {
	if formal.Kind == TypeParameter {
		existing, inferable := bindings[formal.GoType]
		if !inferable {
			return nil
		}
		actual = defaultLiteralType(actual)
		if existing.Kind != Invalid {
			if !sameType(existing, actual) {
				return fmt.Errorf("%s was already inferred as %s, not %s", formal.Name, existing.String(), actual.String())
			}
			return nil
		}
		if !validNativeTypeArgument(actual) {
			return fmt.Errorf("%s cannot be inferred from %s", formal.Name, actual.String())
		}
		bindings[formal.GoType] = actual
		return nil
	}
	if formal.Kind == Interface && (actual.Kind == Interface || actual.Kind == Class) && formal.Name != actual.Name {
		candidates := []Type{actual}
		if actual.Kind == Class {
			candidates = nil
			if class := c.classes[actual.Name]; class != nil {
				classBindings := nativeClassBindings(class, actual)
				for _, implemented := range class.implementedTypes {
					candidates = append(candidates, substituteNativeTypeParameters(implemented, classBindings))
				}
			}
		}
		var inferred nativeTypeBindings
		var firstError error
		for _, candidate := range candidates {
			for _, ancestor := range c.interfaceAncestors(candidate) {
				if ancestor.Name == formal.Name {
					trial := make(nativeTypeBindings, len(bindings))
					for parameter, value := range bindings {
						trial[parameter] = value
					}
					if err := c.inferNativeTypeArguments(formal, ancestor, trial); err != nil {
						if firstError == nil {
							firstError = err
						}
						continue
					}
					if !exactType(substituteNativeTypeParameters(formal, trial), ancestor) {
						continue
					}
					if inferred != nil {
						for parameter, value := range trial {
							if !exactType(value, inferred[parameter]) {
								return fmt.Errorf("ambiguous %s interface ancestors; provide explicit type arguments", formal.Name)
							}
						}
					}
					inferred = trial
				}
			}
		}
		if inferred == nil {
			return firstError
		}
		for parameter, value := range inferred {
			bindings[parameter] = value
		}
		return nil
	}
	if formal.Kind != actual.Kind {
		return nil
	}
	if formal.Kind == Class && formal.Name != actual.Name {
		ancestor, ok := c.classAncestorType(actual, formal.Name)
		if !ok {
			return nil
		}
		actual = ancestor
	}
	switch formal.Kind {
	case Nullable, Array, FixedArray, GoPointer, Result, Task, GoChannel:
		if formal.Element != nil && actual.Element != nil {
			return c.inferNativeTypeArguments(*formal.Element, *actual.Element, bindings)
		}
	case Map:
		if formal.Key != nil && actual.Key != nil {
			if err := c.inferNativeTypeArguments(*formal.Key, *actual.Key, bindings); err != nil {
				return err
			}
		}
		if formal.Element != nil && actual.Element != nil {
			return c.inferNativeTypeArguments(*formal.Element, *actual.Element, bindings)
		}
	case Function:
		if len(formal.Parameters) == len(actual.Parameters) {
			for index := range formal.Parameters {
				if err := c.inferNativeTypeArguments(formal.Parameters[index], actual.Parameters[index], bindings); err != nil {
					return err
				}
			}
		}
		if formal.Result != nil && actual.Result != nil {
			return c.inferNativeTypeArguments(*formal.Result, *actual.Result, bindings)
		}
	case Object:
		for name, formalField := range formal.Fields {
			if actualField, ok := actual.Fields[name]; ok {
				if err := c.inferNativeTypeArguments(formalField, actualField, bindings); err != nil {
					return err
				}
			}
		}
	case Class, Struct, Interface:
		if formal.Name != actual.Name {
			return nil
		}
		fallthrough
	case GoNamed:
		if len(formal.TypeArguments) == len(actual.TypeArguments) {
			for index := range formal.TypeArguments {
				if err := c.inferNativeTypeArguments(formal.TypeArguments[index], actual.TypeArguments[index], bindings); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func substituteNativeTypeParameters(value Type, bindings nativeTypeBindings) Type {
	if len(bindings) == 0 {
		return value
	}
	return substituteNativeTypeParametersSeen(value, bindings, map[string]bool{})
}

// substituteNativeMethodOwnerTypeParameters instantiates the generic owner of
// a method while retaining type parameters declared by the method itself.
// Go cannot encode those parameters in a method set, but Kinmokusei lowers the
// callable to a top-level helper after semantic checking.
func (c *Checker) substituteNativeMethodOwnerTypeParameters(value Type, bindings nativeTypeBindings) Type {
	methodParameters := append([]Type(nil), value.TypeParameters...)
	if len(methodParameters) != 0 && len(bindings) != 0 {
		// Each receiver instantiation owns fresh method parameters. Otherwise a
		// bound such as Slice<E> would still refer to the generic owner's E, or
		// changing it would corrupt other instantiations of the same method.
		native := make(nativeTypeBindings, len(bindings)+len(methodParameters))
		goBindings := make(map[gotypes.Type]gotypes.Type, len(native))
		for identity, argument := range bindings {
			native[identity] = argument
			if storage, ok := c.goTypeForNativeStorage(argument); ok {
				goBindings[identity] = storage
			}
		}
		for i, parameter := range methodParameters {
			clone := gotypes.NewTypeParam(gotypes.NewTypeName(gotoken.NoPos, nil, parameter.Name, nil), nil)
			methodParameters[i].GoType = clone
			native[parameter.GoType] = methodParameters[i]
			goBindings[parameter.GoType] = clone
		}
		for i, parameter := range value.TypeParameters {
			bound := parameter.GoType.(*gotypes.TypeParam).Constraint()
			methodParameters[i].GoType.(*gotypes.TypeParam).SetConstraint(substituteConstraintType(bound, goBindings))
			if shape, ok := c.parameterRangeShape(parameter.GoType.(*gotypes.TypeParam)); ok {
				c.setParameterRangeShape(methodParameters[i].GoType.(*gotypes.TypeParam), substituteNativeTypeParameters(shape, native))
			}
		}
		bindings = native
	}
	result := substituteNativeTypeParameters(value, bindings)
	if len(methodParameters) != 0 {
		result.TypeParameters = methodParameters
		result.Generic = true
	}
	return result
}

func substituteNativeTypeParametersSeen(value Type, bindings nativeTypeBindings, visiting map[string]bool) Type {
	if value.Kind == TypeParameter {
		if replacement, ok := bindings[value.GoType]; ok && replacement.Kind != Invalid {
			return replacement
		}
		return value
	}
	if value.Kind == Struct {
		if visiting[value.Name] {
			result := value
			result.Fields = nil
			result.TypeArguments = append([]Type(nil), value.TypeArguments...)
			for index := range result.TypeArguments {
				result.TypeArguments[index] = substituteNativeTypeParametersSeen(result.TypeArguments[index], bindings, visiting)
			}
			result.TypeParameters = nil
			result.Generic = false
			return instantiateSubstitutedNamedType(result)
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
	}
	result := value
	result.Parameters = append([]Type(nil), value.Parameters...)
	result.GoMethods = append([]GoInterfaceMethod(nil), value.GoMethods...)
	for i := range result.GoMethods {
		result.GoMethods[i].Type = substituteNativeTypeParametersSeen(result.GoMethods[i].Type, bindings, visiting)
	}
	for index := range result.Parameters {
		result.Parameters[index] = substituteNativeTypeParametersSeen(result.Parameters[index], bindings, visiting)
	}
	result.TypeParameters = nil
	if value.Result != nil {
		item := substituteNativeTypeParametersSeen(*value.Result, bindings, visiting)
		result.Result = &item
	}
	if value.Element != nil {
		item := substituteNativeTypeParametersSeen(*value.Element, bindings, visiting)
		result.Element = &item
	}
	if value.Key != nil {
		item := substituteNativeTypeParametersSeen(*value.Key, bindings, visiting)
		result.Key = &item
	}
	if value.Fields != nil {
		result.Fields = make(map[string]Type, len(value.Fields))
		for name, field := range value.Fields {
			result.Fields[name] = substituteNativeTypeParametersSeen(field, bindings, visiting)
		}
	}
	result.Results = append([]Type(nil), value.Results...)
	for index := range result.Results {
		result.Results[index] = substituteNativeTypeParametersSeen(result.Results[index], bindings, visiting)
	}
	result.TypeArguments = append([]Type(nil), value.TypeArguments...)
	for index := range result.TypeArguments {
		result.TypeArguments[index] = substituteNativeTypeParametersSeen(result.TypeArguments[index], bindings, visiting)
	}
	result.Generic = false
	result = instantiateSubstitutedNamedType(result)
	if value.Kind == GoInterface {
		result.GoType = nil
		if converted, ok := goTypeOf(result); ok {
			result.GoType = converted
			result.Name = goTypeDisplayName(converted)
		}
	}
	if value.Kind == Array || value.Kind == FixedArray || value.Kind == Map || value.Kind == Function || value.Kind == Object || value.Kind == Nullable || value.Kind == Result || value.Kind == Task {
		result.GoType = nil
	}
	if value.Kind == GoPointer && result.Element != nil {
		result.GoType = nil
		if element, ok := goTypeOf(*result.Element); ok {
			result.GoType = gotypes.NewPointer(element)
		}
	}
	if value.Kind == GoChannel && result.Element != nil {
		direction := gotypes.SendRecv
		if value.GoType != nil {
			if channel, ok := gotypes.Unalias(value.GoType).Underlying().(*gotypes.Chan); ok {
				direction = channel.Dir()
			}
		}
		result.GoType = nil
		if element, ok := goTypeOf(*result.Element); ok {
			result.GoType = gotypes.NewChan(direction, element)
		}
	}
	return result
}

// Keep the Go storage type in sync with substituted source arguments, also for
// native structs nested in invariant collection types or recursive pointers.
func instantiateSubstitutedNamedType(value Type) Type {
	if (value.Kind != GoNamed && value.Kind != Struct) || len(value.TypeArguments) == 0 {
		return value
	}
	named, ok := value.GoType.(*gotypes.Named)
	if !ok || named.Obj() == nil {
		return value
	}
	arguments := make([]gotypes.Type, len(value.TypeArguments))
	for index, argument := range value.TypeArguments {
		converted, ok := goTypeOf(argument)
		if !ok {
			return value
		}
		arguments[index] = converted
	}
	instantiated, err := gotypes.Instantiate(nil, named.Origin(), arguments, true)
	if err != nil {
		return value
	}
	value.GoType = instantiated
	if value.Kind == GoNamed {
		names := make([]string, len(value.TypeArguments))
		for index, argument := range value.TypeArguments {
			names[index] = argument.String()
		}
		value.Name = named.Obj().Name() + "<" + strings.Join(names, ", ") + ">"
	}
	return value
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (c *Checker) checkExplicitGenericCall(expr *ast.CallExpr, callableName string, callable Type) Type {
	signature, ok := callable.GoType.(*gotypes.Signature)
	if !ok || signature.TypeParams() == nil || signature.TypeParams().Len() == 0 {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s has invalid generic Go type information", callableName))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if len(expr.TypeArguments) > signature.TypeParams().Len() {
		c.report(expr.Span, fmt.Sprintf("%s has %d Go type parameters, got %d explicit type arguments", callableName, signature.TypeParams().Len(), len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	typeArguments := make([]gotypes.Type, len(expr.TypeArguments))
	for i := range expr.TypeArguments {
		resolved := c.resolveType(expr.TypeArguments[i])
		if resolved.Kind == Invalid {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		goType, valid := goTypeOf(resolved)
		if !valid {
			c.report(expr.TypeArguments[i].Span, fmt.Sprintf("type argument %s cannot be represented as a Go type", resolved.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		typeArguments[i] = goType
	}
	actualTypes := c.checkGoGenericCallbackArguments(expr, signature, typeArguments)
	numericArguments := c.genericNumericArguments(expr.Arguments, actualTypes)
	instantiatedSignature, err := inferGoGenericCall(signature, actualTypes, typeArguments, expr.Expanded, numericArguments)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("cannot apply explicit Go type arguments to %s: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	converted, err := kinmokuseiFunctionFromGo(instantiatedSignature)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("instantiated Go call to %s is not supported: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.recordCallSignature(expr, converted)
	return *converted.Result
}

func (c *Checker) checkInferredGenericCall(expr *ast.CallExpr, callableName string, callable Type) Type {
	signature, ok := callable.GoType.(*gotypes.Signature)
	if !ok || signature.TypeParams() == nil || signature.TypeParams().Len() == 0 {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s has invalid generic Go type information", callableName))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	actualTypes := c.checkGoGenericCallbackArguments(expr, signature, nil)
	numericArguments := c.genericNumericArguments(expr.Arguments, actualTypes)
	instantiated, err := inferGoGenericCall(signature, actualTypes, nil, expr.Expanded, numericArguments)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("cannot infer Go type arguments for %s: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	converted, err := kinmokuseiFunctionFromGo(instantiated)
	if err != nil {
		c.report(expr.Span, fmt.Sprintf("inferred Go call to %s is not supported: %v", callableName, err))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.recordCallSignature(expr, converted)
	return *converted.Result
}

func inferGoGenericCall(signature *gotypes.Signature, actualTypes []Type, explicitTypeArguments []gotypes.Type, expanded bool, numericArguments []gotypes.TypeAndValue) (*gotypes.Signature, error) {
	packageInfo := gotypes.NewPackage("kinmokusei.synthetic/generic", "generic")
	functionName := "__kinmokusei_generic_function"
	functionIdentifier := goast.NewIdent(functionName)
	if existing := packageInfo.Scope().Insert(gotypes.NewFunc(0, packageInfo, functionName, signature)); existing != nil {
		return nil, fmt.Errorf("cannot create inference scope")
	}
	arguments := make([]goast.Expr, len(actualTypes))
	for i, actual := range actualTypes {
		if i < len(numericArguments) && numericArguments[i].Value != nil {
			name := fmt.Sprintf("__kinmokusei_argument_%d", i)
			info := numericArguments[i]
			packageInfo.Scope().Insert(gotypes.NewConst(0, packageInfo, name, info.Type, info.Value))
			arguments[i] = goast.NewIdent(name)
			continue
		}
		switch actual.Kind {
		case UntypedInt:
			arguments[i] = &goast.BasicLit{Kind: gotoken.INT, Value: "0"}
		case Nil:
			arguments[i] = goast.NewIdent("nil")
		default:
			goType, ok := goTypeOf(actual)
			if !ok {
				return nil, fmt.Errorf("argument %d has non-Go-representable type %s", i+1, actual.String())
			}
			name := fmt.Sprintf("__kinmokusei_argument_%d", i)
			if existing := packageInfo.Scope().Insert(gotypes.NewVar(0, packageInfo, name, goType)); existing != nil {
				return nil, fmt.Errorf("cannot create inference argument %d", i+1)
			}
			arguments[i] = goast.NewIdent(name)
		}
	}
	packageInfo.MarkComplete()
	var functionExpression goast.Expr = functionIdentifier
	if len(explicitTypeArguments) != 0 {
		typeExpressions := make([]goast.Expr, len(explicitTypeArguments))
		for i, argumentType := range explicitTypeArguments {
			name := fmt.Sprintf("__kinmokusei_type_argument_%d", i)
			if existing := packageInfo.Scope().Insert(gotypes.NewTypeName(0, packageInfo, name, argumentType)); existing != nil {
				return nil, fmt.Errorf("cannot create explicit type argument %d", i+1)
			}
			typeExpressions[i] = goast.NewIdent(name)
		}
		if len(typeExpressions) == 1 {
			functionExpression = &goast.IndexExpr{X: functionIdentifier, Index: typeExpressions[0]}
		} else {
			functionExpression = &goast.IndexListExpr{X: functionIdentifier, Indices: typeExpressions}
		}
	}
	call := &goast.CallExpr{Fun: functionExpression, Args: arguments}
	if expanded {
		call.Ellipsis = gotoken.Pos(1)
	}
	info := &gotypes.Info{
		Types:     map[goast.Expr]gotypes.TypeAndValue{},
		Uses:      map[*goast.Ident]gotypes.Object{},
		Instances: map[*goast.Ident]gotypes.Instance{},
	}
	if err := gotypes.CheckExpr(gotoken.NewFileSet(), packageInfo, gotoken.NoPos, call, info); err != nil {
		return nil, err
	}
	instance, ok := info.Instances[functionIdentifier]
	if !ok {
		return nil, fmt.Errorf("Go type checker did not report an inferred instance")
	}
	instantiated, ok := instance.Type.(*gotypes.Signature)
	if !ok {
		return nil, fmt.Errorf("Go type checker returned %T instead of a function signature", instance.Type)
	}
	return instantiated, nil
}
