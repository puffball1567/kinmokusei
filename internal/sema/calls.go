package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkCall(expr *ast.CallExpr) Type {
	defer func() {
		if !expr.Conversion && expr.Builtin == ast.NotBuiltinCall {
			c.recordMemberWrite(expr.Span)
			c.invalidateAllMemberFacts(expr.Span, "a call with unknown mutation effects")
		}
	}()
	name, ok := expr.Callee.(*ast.IdentifierExpr)
	if ok {
		if _, shadowed := c.lookupValue(name.Name, name.Span); !shadowed {
			if parameter, exists := c.lookupTypeParameter(name.Name); exists {
				expr.Conversion = true
				ref := ast.TypeRef{Name: name.Name, NameSpan: name.Span, Span: name.Span, TypeParameter: true}
				expr.ConversionType = &ref
				if len(expr.TypeArguments) != 0 {
					c.report(expr.Span, "type parameter conversions do not accept type arguments")
				}
				return c.checkNativeTypeConversion(expr, parameter)
			}
		}
		if name.Name == "super" {
			return c.checkSuperConstructorCall(expr)
		}
		switch name.Name {
		case "goChannel":
			return c.checkGoChannelMake(expr)
		case "closeGoChannel":
			return c.checkGoChannelClose(expr)
		}
		if !c.hasCallBinding(name.Name, name.Span) {
			switch name.Name {
			case "ok":
				return c.checkResultConstructor(expr, true)
			case "fail":
				return c.checkResultConstructor(expr, false)
			case "len":
				return c.checkCollectionLen(expr)
			case "cap":
				return c.checkCollectionCap(expr)
			case "append":
				return c.checkCollectionAppend(expr)
			case "copy":
				return c.checkCollectionCopy(expr)
			case "delete":
				return c.checkCollectionDelete(expr)
			case "clear":
				return c.checkCollectionClear(expr)
			case "min", "max":
				return c.checkOrderedBuiltin(expr, name.Name)
			case "complex", "real", "imag":
				return c.checkComplexBuiltin(expr, name.Name)
			case "makeSlice":
				return c.checkMakeSlice(expr)
			case "makeMap":
				return c.checkMakeMap(expr)
			case "copyArray":
				return c.checkSliceToArray(expr, false)
			case "viewArray":
				return c.checkSliceToArray(expr, true)
			}
		}
	}
	if result, unsafeBuiltin := c.checkUnsafeBuiltinCall(expr); unsafeBuiltin {
		return result
	}
	if ok {
		if _, shadowed := c.lookupValue(name.Name, name.Span); !shadowed {
			if named, exists := c.nativeTypes[name.Name]; exists && c.isTopLevelAllowed(name.Span, name.Name) {
				name.ResolvedDeclaration = named.declaration.NameSpan
				expr.Conversion = true
				targetRef := ast.TypeRef{Name: name.Name, NameSpan: name.Span, GenericArguments: expr.TypeArguments, Span: expr.Span}
				if named.declaration.Alias && len(named.typeParameters) != 0 && len(expr.TypeArguments) == len(named.typeParameters) {
					expanded := instantiateGenericAliasTypeRef(targetRef, named.declaration)
					expr.ConversionType = &expanded
				}
				return c.checkNativeTypeConversion(expr, c.resolveNativeDefinedType(targetRef, named))
			}
			if structure, exists := c.structs[name.Name]; exists && c.isTopLevelAllowed(name.Span, name.Name) {
				name.ResolvedDeclaration = structure.declarationSpan
				expr.Conversion = true
				targetRef := ast.TypeRef{Name: name.Name, NameSpan: name.Span, GenericArguments: expr.TypeArguments, Span: expr.Span}
				return c.checkNativeTypeConversion(expr, c.resolveNativeStructType(targetRef, structure))
			}
		}
		if target, isConversion := LookupType(name.Name); isConversion && target.Kind != Void {
			expr.Conversion = true
			if expr.Expanded {
				c.report(expr.Span, "spread arguments cannot be used in type conversions")
			}
			if len(expr.Arguments) != 1 {
				c.report(expr.Span, fmt.Sprintf("conversion to %s expects 1 argument, got %d", name.Name, len(expr.Arguments)))
				return target
			}
			value := c.checkExpression(expr.Arguments[0])
			if isComplexType(target) || isComplexType(value) || isUntypedGoNumeric(value) {
				return c.checkComplexConversion(expr, target, value)
			}
			targetGo, targetRepresentable := goTypeOf(target)
			valueGo, valueRepresentable := goTypeOf(value)
			convertible := targetRepresentable && valueRepresentable && value.Kind != Nullable && gotypes.ConvertibleTo(valueGo, targetGo)
			if !convertible {
				c.report(expr.Span, fmt.Sprintf("cannot convert %s to %s", value.String(), target.String()))
			} else if target.IsNumeric() {
				integer, known := c.resolvedIntegerConstantValue(expr.Arguments[0])
				if known && !integerConstantFitsFixedType(integer, target) {
					c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), target.String()))
				}
			}
			return target
		}
	}
	var callable Type
	var callableName string
	if ok {
		if fn, exists := c.functions[name.Name]; exists && c.isTopLevelAllowed(name.Span, name.Name) {
			name.ResolvedDeclaration = fn.declarationSpan
			callable = callableTypeForFunction(fn)
			callableName = fmt.Sprintf("function %q", name.Name)
		} else if symbol, exists := c.lookupSymbol(name.Name, name.Span); exists {
			name.ResolvedDeclaration = symbol.declarationSpan
			callable = symbol.typeInfo
			callableName = fmt.Sprintf("value %q", name.Name)
		} else if imported, exists := c.lookupNamedGoImport(name.Name, name.Span); exists {
			callable = c.checkNamedGoIdentifier(name, imported)
			callableName = fmt.Sprintf("Go member %q", name.Name)
		} else {
			c.report(name.Span, fmt.Sprintf("undefined function %q", name.Name))
		}
	} else {
		previousCallee := c.directCallCallee
		c.directCallCallee = expr.Callee
		callable = c.checkExpression(expr.Callee)
		c.directCallCallee = previousCallee
		callableName = "expression"
	}
	if callable.Kind == Invalid {
		for _, arg := range expr.Arguments {
			c.checkExpression(arg)
		}
		return callable
	}
	if callable.Kind == GoTypeName {
		expr.Conversion = true
		return c.checkGoConversion(expr, callable)
	}
	if callable.Kind == GoNamed && callable.GoType != nil {
		if converted, err := kinmokuseiTypeFromGo(callable.GoType); err == nil && converted.Kind == Function {
			callable = converted
		}
	}
	if callable.Kind == Nullable {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("nullable callable %s must be checked against null before calling", callable.String()))
		if callable.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		callable = *callable.Element
	}
	if callable.Kind != Function || callable.Result == nil {
		c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s is not callable", callableName))
		for _, arg := range expr.Arguments {
			c.checkExpression(arg)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.recordCallSignature(expr, callable)
	if callable.Generic && len(callable.TypeParameters) != 0 {
		return c.checkNativeGenericCall(expr, callableName, callable)
	}
	if len(expr.TypeArguments) != 0 {
		if !callable.Generic {
			c.report(expr.Callee.GetSpan(), fmt.Sprintf("%s is not a generic Go function and cannot receive explicit type arguments", callableName))
			for _, argument := range expr.Arguments {
				c.checkExpression(argument)
			}
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return c.checkExplicitGenericCall(expr, callableName, callable)
	} else if callable.Generic {
		return c.checkInferredGenericCall(expr, callableName, callable)
	}
	if expr.Expanded {
		if !callable.Variadic || len(callable.Parameters) == 0 {
			c.report(expr.Span, fmt.Sprintf("%s is not variadic and cannot receive a spread argument", callableName))
			for _, argument := range expr.Arguments {
				c.checkExpression(argument)
			}
			return *callable.Result
		}
		expectedArguments := len(callable.Parameters)
		if len(expr.Arguments) != expectedArguments {
			c.report(expr.Span, fmt.Sprintf("spread call to %s expects %d arguments (%d fixed and one slice), got %d", callableName, expectedArguments, expectedArguments-1, len(expr.Arguments)))
		}
		fixedArguments := len(callable.Parameters) - 1
		for i, argument := range expr.Arguments {
			if i == len(expr.Arguments)-1 {
				element := callable.Parameters[len(callable.Parameters)-1]
				expected := Type{Kind: Array, Name: "array", Element: &element}
				actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
				c.requireAssignable(expected, actual, argument.GetSpan())
				continue
			}
			if i < fixedArguments {
				expected := callable.Parameters[i]
				actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
				c.requireAssignable(expected, actual, argument.GetSpan())
			} else {
				expected := callable.Parameters[len(callable.Parameters)-1]
				actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
				c.requireAssignable(expected, actual, argument.GetSpan())
			}
		}
		return *callable.Result
	}
	minimumArguments := len(callable.Parameters)
	if callable.Variadic {
		minimumArguments--
	}
	if len(expr.Arguments) < minimumArguments || (!callable.Variadic && len(expr.Arguments) != len(callable.Parameters)) {
		if callable.Variadic {
			c.report(expr.Span, fmt.Sprintf("%s expects at least %d arguments, got %d", callableName, minimumArguments, len(expr.Arguments)))
		} else {
			c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d", callableName, len(callable.Parameters), len(expr.Arguments)))
		}
	}
	for i, arg := range expr.Arguments {
		parameterIndex := i
		if callable.Variadic && parameterIndex >= len(callable.Parameters)-1 {
			parameterIndex = len(callable.Parameters) - 1
		}
		if parameterIndex >= 0 && parameterIndex < len(callable.Parameters) {
			expected := callable.Parameters[parameterIndex]
			actual := c.checkExpressionExpectedSlot(&expr.Arguments[i], expected)
			c.requireAssignable(expected, actual, arg.GetSpan())
		} else {
			c.checkExpression(arg)
		}
	}
	return *callable.Result
}

func (c *Checker) checkSuperConstructorCall(expr *ast.CallExpr) Type {
	class := c.classes[c.currentClass]
	if !c.inConstructor || class == nil || class.base == "" {
		c.report(expr.Span, "super(...) may only be called from a derived-class constructor")
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	base := c.classes[class.base]
	if base == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	bindings := nativeClassBindings(base, class.baseType)
	parameters := make([]Type, len(base.constructor))
	for index, parameter := range base.constructor {
		parameters[index] = substituteNativeTypeParameters(parameter, bindings)
	}
	c.checkConstructorArguments(expr.Arguments, expr.Expanded, parameters, base.constructorVariadic, fmt.Sprintf("base constructor %q", class.base), expr.Span)
	expr.SuperConstructor = true
	expr.SuperBase = class.base
	return builtins["void"]
}

func (c *Checker) recordCallSignature(expr *ast.CallExpr, callable Type) {
	if callable.Kind != Function || callable.Result == nil {
		return
	}
	signature := &ast.CallableSignature{
		ParameterNames: make([]string, len(callable.Parameters)),
		ParameterTypes: make([]string, len(callable.Parameters)),
		Result:         callable.Result.String(),
		Variadic:       callable.Variadic,
	}
	for index, parameter := range callable.Parameters {
		signature.ParameterTypes[index] = parameter.String()
	}
	if goSignature, ok := callable.GoType.(*gotypes.Signature); ok {
		for index := 0; index < goSignature.Params().Len() && index < len(signature.ParameterNames); index++ {
			signature.ParameterNames[index] = goSignature.Params().At(index).Name()
		}
	}
	expr.Signature = signature
}
