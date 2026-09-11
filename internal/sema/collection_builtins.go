package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkGoChannelMake(expr *ast.CallExpr) Type {
	expr.Builtin = ast.MakeGoChannelCall
	if expr.Expanded {
		c.report(expr.Span, "goChannel does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("goChannel expects one type argument, got %d", len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element := c.resolveType(expr.TypeArguments[0])
	elementGoType, ok := goTypeOf(element)
	if !ok {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Go channel element", element.String()))
	}
	if len(expr.Arguments) > 1 {
		c.report(expr.Span, fmt.Sprintf("goChannel expects zero or one capacity argument, got %d", len(expr.Arguments)))
	}
	for _, argument := range expr.Arguments {
		capacity := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if capacity.Kind != Invalid && !c.isIntegerContext(argument, capacity) {
			c.report(argument.GetSpan(), fmt.Sprintf("goChannel capacity must be an integer, got %s", capacity.String()))
		}
		if constant, known := c.integerContextValue(argument); known && constant.Sign() < 0 {
			c.report(argument.GetSpan(), "goChannel capacity cannot be negative")
		} else if known && !constant.IsInt64() {
			c.report(argument.GetSpan(), "goChannel capacity is out of range")
		}
	}
	if !ok || element.Kind == Invalid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: GoChannel, Name: "GoChannel", Element: &element, GoType: gotypes.NewChan(gotypes.SendRecv, elementGoType), GoQualifier: element.GoQualifier}
}

func (c *Checker) checkGoChannelClose(expr *ast.CallExpr) Type {
	expr.Builtin = ast.CloseGoChannelCall
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, "closeGoChannel does not accept type arguments")
	}
	if expr.Expanded {
		c.report(expr.Span, "closeGoChannel does not accept spread arguments")
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("closeGoChannel expects one channel argument, got %d", len(expr.Arguments)))
	}
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind == Nullable {
			c.report(argument.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before closing", value.String()))
			if value.Element == nil {
				continue
			}
			value = *value.Element
		}
		goType, ok := goTypeOf(value)
		if !ok {
			c.report(argument.GetSpan(), fmt.Sprintf("closeGoChannel requires a Go channel, got %s", value.String()))
			continue
		}
		channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
		if !ok {
			c.report(argument.GetSpan(), fmt.Sprintf("closeGoChannel requires a Go channel, got %s", value.String()))
			continue
		}
		if channel.Dir() == gotypes.RecvOnly {
			c.report(argument.GetSpan(), fmt.Sprintf("cannot close receive-only channel %s", value.String()))
		}
	}
	return builtins["void"]
}

func (c *Checker) checkCollectionLen(expr *ast.CallExpr) Type {
	expr.Builtin = ast.LenCall
	c.checkBuiltinCallShape(expr, "len", 1, 1, 0)
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind != Invalid && !isLenCollection(value) {
			c.report(argument.GetSpan(), fmt.Sprintf("len requires a string, array, array pointer, slice, map, or channel, got %s", value.String()))
		}
	}
	return builtins["int"]
}

func (c *Checker) checkCollectionCap(expr *ast.CallExpr) Type {
	expr.Builtin = ast.CapCall
	c.checkBuiltinCallShape(expr, "cap", 1, 1, 0)
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind != Invalid && !isCapCollection(value) {
			c.report(argument.GetSpan(), fmt.Sprintf("cap requires an array, array pointer, slice, or channel, got %s", value.String()))
		}
	}
	return builtins["int"]
}

func (c *Checker) checkCollectionAppend(expr *ast.CallExpr) Type {
	expr.Builtin = ast.AppendCall
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, "append does not accept type arguments")
	}
	if len(expr.Arguments) == 0 {
		c.report(expr.Span, "append expects a destination slice")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	destination := c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan())
	element, ok := c.sliceElementType(destination, expr.Arguments[0].GetSpan())
	if !ok {
		if destination.Kind != Invalid {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("append requires a slice as its first argument, got %s", destination.String()))
		}
		for _, argument := range expr.Arguments[1:] {
			c.checkExpression(argument)
		}
		return destination
	}
	if expr.Expanded {
		if len(expr.Arguments) != 2 {
			c.report(expr.Span, fmt.Sprintf("spread append expects a destination and one expanded source, got %d arguments", len(expr.Arguments)))
		}
		if len(expr.Arguments) >= 2 {
			source := c.singleValue(c.checkExpression(expr.Arguments[1]), expr.Arguments[1].GetSpan())
			if !(source.IsString() && isBuiltinByte(element)) {
				sourceElement, sourceOK := c.sliceElementType(source, expr.Arguments[1].GetSpan())
				if !sourceOK {
					if source.Kind != Invalid {
						c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("expanded append source must be a compatible slice, got %s", source.String()))
					}
				} else if !identicalCollectionElement(element, sourceElement) {
					c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("expanded append source element %s does not match destination element %s", sourceElement.String(), element.String()))
				}
			}
		}
		for _, argument := range expr.Arguments[2:] {
			c.checkExpression(argument)
		}
		return destination
	}
	for _, argument := range expr.Arguments[1:] {
		actual := c.checkExpressionExpected(argument, element)
		c.requireAssignable(element, actual, argument.GetSpan())
	}
	return destination
}

func (c *Checker) checkCollectionCopy(expr *ast.CallExpr) Type {
	expr.Builtin = ast.CopyCall
	c.checkBuiltinCallShape(expr, "copy", 2, 2, 0)
	values := make([]Type, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		values[i] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}
	if len(values) < 2 {
		return builtins["int"]
	}
	destinationElement, destinationOK := c.sliceElementType(values[0], expr.Arguments[0].GetSpan())
	if !destinationOK {
		if values[0].Kind != Invalid {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("copy destination must be a slice, got %s", values[0].String()))
		}
		return builtins["int"]
	}
	if values[1].IsString() && isBuiltinByte(destinationElement) {
		return builtins["int"]
	}
	sourceElement, sourceOK := c.sliceElementType(values[1], expr.Arguments[1].GetSpan())
	if !sourceOK {
		if values[1].Kind != Invalid {
			c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("copy source must be a compatible slice or string for byte destinations, got %s", values[1].String()))
		}
	} else if !identicalCollectionElement(destinationElement, sourceElement) {
		c.report(expr.Arguments[1].GetSpan(), fmt.Sprintf("copy source element %s does not match destination element %s", sourceElement.String(), destinationElement.String()))
	}
	return builtins["int"]
}

func (c *Checker) checkCollectionDelete(expr *ast.CallExpr) Type {
	expr.Builtin = ast.DeleteCall
	c.checkBuiltinCallShape(expr, "delete", 2, 2, 0)
	values := make([]Type, len(expr.Arguments))
	for i, argument := range expr.Arguments {
		values[i] = c.singleValue(c.checkExpression(argument), argument.GetSpan())
	}
	if len(values) < 2 {
		return builtins["void"]
	}
	key, _, ok := c.mapCollectionTypes(values[0], expr.Arguments[0].GetSpan())
	if !ok {
		if values[0].Kind != Invalid {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("delete requires a map as its first argument, got %s", values[0].String()))
		}
		return builtins["void"]
	}
	c.requireAssignable(key, values[1], expr.Arguments[1].GetSpan())
	return builtins["void"]
}

func (c *Checker) checkCollectionClear(expr *ast.CallExpr) Type {
	expr.Builtin = ast.ClearCall
	c.checkBuiltinCallShape(expr, "clear", 1, 1, 0)
	for _, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if value.Kind != Invalid && !isClearCollection(value) {
			c.report(argument.GetSpan(), fmt.Sprintf("clear requires a slice or map, got %s", value.String()))
		}
	}
	return builtins["void"]
}

func (c *Checker) checkOrderedBuiltin(expr *ast.CallExpr, name string) Type {
	if name == "min" {
		expr.Builtin = ast.MinCall
	} else {
		expr.Builtin = ast.MaxCall
	}
	if len(expr.TypeArguments) != 0 {
		c.report(expr.Span, fmt.Sprintf("%s does not accept type arguments", name))
	}
	if expr.Expanded {
		c.report(expr.Span, fmt.Sprintf("%s does not accept spread arguments", name))
	}
	if len(expr.Arguments) == 0 {
		c.report(expr.Span, fmt.Sprintf("%s expects at least 1 argument, got 0", name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}

	values := make([]Type, len(expr.Arguments))
	result := Type{Kind: Invalid, Name: "<invalid>"}
	for index, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		values[index] = value
		if value.Kind != Invalid && !value.IsOrdered() {
			c.report(argument.GetSpan(), fmt.Sprintf("%s requires ordered operands, got %s", name, value.String()))
		}
		if result.Kind == Invalid || (result.Kind == UntypedInt && value.Kind != Invalid && value.Kind != UntypedInt) {
			result = value
		}
	}
	if result.Kind == Invalid {
		return result
	}
	for index, value := range values {
		if value.Kind == Invalid {
			continue
		}
		if !sameType(result, value) {
			c.report(expr.Arguments[index].GetSpan(), fmt.Sprintf("%s operands must have one ordered type; got %s and %s", name, result.String(), value.String()))
			continue
		}
		if integer, known := c.resolvedIntegerConstantValue(expr.Arguments[index]); known && value.Kind == UntypedInt && !integerConstantFitsFixedType(integer, result) {
			c.report(expr.Arguments[index].GetSpan(), fmt.Sprintf("integer constant %s cannot be represented as %s", integer.String(), result.String()))
		}
	}
	return result
}

func (c *Checker) checkMakeSlice(expr *ast.CallExpr) Type {
	expr.Builtin = ast.MakeSliceCall
	if expr.Expanded {
		c.report(expr.Span, "makeSlice does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("makeSlice expects one element type argument, got %d", len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	element := c.resolveType(expr.TypeArguments[0])
	if !isCollectionElementType(element) {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be used as a slice element", element.String()))
	}
	c.checkMakeSizeArguments(expr, "makeSlice", 1, 2)
	if len(expr.Arguments) >= 2 {
		length, lengthOK := c.integerContextValue(expr.Arguments[0])
		capacity, capacityOK := c.integerContextValue(expr.Arguments[1])
		if lengthOK && capacityOK && capacity.Cmp(length) < 0 {
			c.report(expr.Arguments[1].GetSpan(), "makeSlice capacity cannot be smaller than length")
		}
	}
	if element.Kind == Invalid {
		return element
	}
	return Type{Kind: Array, Name: "array", Element: &element}
}

func (c *Checker) checkMakeMap(expr *ast.CallExpr) Type {
	expr.Builtin = ast.MakeMapCall
	if expr.Expanded {
		c.report(expr.Span, "makeMap does not accept spread arguments")
	}
	if len(expr.TypeArguments) != 2 {
		c.report(expr.Span, fmt.Sprintf("makeMap expects key and value type arguments, got %d", len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	key := c.resolveType(expr.TypeArguments[0])
	value := c.resolveType(expr.TypeArguments[1])
	if key.Kind != Invalid && !key.IsComparable() {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be used as a Map key", key.String()))
	}
	if !isCollectionElementType(value) {
		c.report(expr.TypeArguments[1].Span, fmt.Sprintf("type %s cannot be used as a Map value", value.String()))
	}
	c.checkMakeSizeArguments(expr, "makeMap", 0, 1)
	return Type{Kind: Map, Name: "Map", Key: &key, Element: &value}
}

func (c *Checker) checkSliceToArray(expr *ast.CallExpr, view bool) Type {
	name := "copyArray"
	expr.Builtin = ast.CopyArrayCall
	if view {
		name = "viewArray"
		expr.Builtin = ast.ViewArrayCall
	}
	if expr.Expanded {
		c.report(expr.Span, fmt.Sprintf("%s does not accept spread arguments", name))
	}
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("%s expects one fixed array type argument, got %d", name, len(expr.TypeArguments)))
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	target := c.resolveType(expr.TypeArguments[0])
	targetGo, targetOK := goTypeOf(target)
	if targetOK {
		_, targetOK = gotypes.Unalias(targetGo).Underlying().(*gotypes.Array)
	}
	if target.Kind != Invalid && !targetOK {
		c.report(expr.TypeArguments[0].Span, fmt.Sprintf("%s target must be a fixed array type, got %s", name, target.String()))
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("%s expects one slice argument, got %d", name, len(expr.Arguments)))
	}
	var source Type
	for i, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		if i == 0 {
			source = value
		}
	}
	if len(expr.Arguments) == 1 && source.Kind != Invalid {
		sourceGo, sourceOK := goTypeOf(source)
		if sourceOK {
			_, sourceOK = gotypes.Unalias(sourceGo).Underlying().(*gotypes.Slice)
		}
		if !sourceOK {
			c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s requires a slice source, got %s", name, source.String()))
		} else if targetOK {
			conversionTarget := targetGo
			if view {
				conversionTarget = gotypes.NewPointer(targetGo)
			}
			if !gotypes.ConvertibleTo(sourceGo, conversionTarget) {
				c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("cannot convert slice %s to %s target %s", source.String(), name, target.String()))
			}
		}
	}
	if !targetOK || target.Kind == Invalid {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if !view {
		return target
	}
	return Type{Kind: GoPointer, Name: "*" + target.String(), Element: &target, GoType: gotypes.NewPointer(targetGo), GoQualifier: target.GoQualifier}
}

func (c *Checker) checkBuiltinCallShape(expr *ast.CallExpr, name string, minimum, maximum, typeArguments int) {
	if len(expr.TypeArguments) != typeArguments {
		c.report(expr.Span, fmt.Sprintf("%s expects %d type arguments, got %d", name, typeArguments, len(expr.TypeArguments)))
	}
	if expr.Expanded {
		c.report(expr.Span, fmt.Sprintf("%s does not accept spread arguments", name))
	}
	if len(expr.Arguments) < minimum || len(expr.Arguments) > maximum {
		if minimum == maximum {
			c.report(expr.Span, fmt.Sprintf("%s expects %d arguments, got %d", name, minimum, len(expr.Arguments)))
		} else {
			c.report(expr.Span, fmt.Sprintf("%s expects between %d and %d arguments, got %d", name, minimum, maximum, len(expr.Arguments)))
		}
	}
}

func (c *Checker) checkMakeSizeArguments(expr *ast.CallExpr, name string, minimum, maximum int) {
	if len(expr.Arguments) < minimum || len(expr.Arguments) > maximum {
		c.report(expr.Span, fmt.Sprintf("%s expects between %d and %d size arguments, got %d", name, minimum, maximum, len(expr.Arguments)))
	}
	expr.IntegerSizeArguments = make([]bool, len(expr.Arguments))
	for index, argument := range expr.Arguments {
		value := c.singleValue(c.checkExpression(argument), argument.GetSpan())
		integer := c.isIntegerContext(argument, value)
		expr.IntegerSizeArguments[index] = integer && !value.IsInteger()
		if value.Kind != Invalid && !integer {
			c.report(argument.GetSpan(), fmt.Sprintf("%s size must be an integer, got %s", name, value.String()))
		}
		if constant, known := c.integerContextValue(argument); known {
			if constant.Sign() < 0 {
				c.report(argument.GetSpan(), fmt.Sprintf("%s size cannot be negative", name))
			} else if !constant.IsInt64() {
				c.report(argument.GetSpan(), fmt.Sprintf("%s size is out of range", name))
			}
		}
	}
}

func (c *Checker) sliceElementType(value Type, span source.Span) (Type, bool) {
	if value.Kind == Nullable && value.Element != nil {
		return c.sliceElementType(*value.Element, span)
	}
	if value.Kind == Array && value.Element != nil {
		return *value.Element, true
	}
	goType, ok := goTypeOf(value)
	if !ok {
		return Type{}, false
	}
	slice, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Slice)
	if !ok {
		return Type{}, false
	}
	return c.collectionElementType(slice.Elem(), value, span), true
}

func (c *Checker) mapCollectionTypes(value Type, span source.Span) (Type, Type, bool) {
	if value.Kind == Nullable && value.Element != nil {
		return c.mapCollectionTypes(*value.Element, span)
	}
	if value.Kind == Map && value.Key != nil && value.Element != nil {
		return *value.Key, *value.Element, true
	}
	goType, ok := goTypeOf(value)
	if !ok {
		return Type{}, Type{}, false
	}
	mapping, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Map)
	if !ok {
		return Type{}, Type{}, false
	}
	return c.collectionElementType(mapping.Key(), value, span), c.collectionElementType(mapping.Elem(), value, span), true
}

func isLenCollection(value Type) bool {
	if value.Kind == Nullable && value.Element != nil {
		return isLenCollection(*value.Element)
	}
	if value.Kind == Array || value.Kind == FixedArray || value.Kind == Map || value.Kind == String || value.Kind == GoChannel {
		return true
	}
	return goCollectionAcceptsLenOrCap(value, true)
}

func isCapCollection(value Type) bool {
	if value.Kind == Nullable && value.Element != nil {
		return isCapCollection(*value.Element)
	}
	if value.Kind == Array || value.Kind == FixedArray || value.Kind == GoChannel {
		return true
	}
	return goCollectionAcceptsLenOrCap(value, false)
}

func isClearCollection(value Type) bool {
	if value.Kind == Nullable && value.Element != nil {
		return isClearCollection(*value.Element)
	}
	if value.Kind == Array || value.Kind == Map {
		return true
	}
	goType, ok := goTypeOf(value)
	if !ok {
		return false
	}
	switch gotypes.Unalias(goType).Underlying().(type) {
	case *gotypes.Slice, *gotypes.Map:
		return true
	default:
		return false
	}
}

func goCollectionAcceptsLenOrCap(value Type, allowLenOnly bool) bool {
	goType, ok := goTypeOf(value)
	if !ok {
		return false
	}
	underlying := gotypes.Unalias(goType).Underlying()
	switch collection := underlying.(type) {
	case *gotypes.Array, *gotypes.Slice, *gotypes.Chan:
		return true
	case *gotypes.Map:
		return allowLenOnly
	case *gotypes.Basic:
		return allowLenOnly && collection.Info()&gotypes.IsString != 0
	case *gotypes.Pointer:
		_, ok := gotypes.Unalias(collection.Elem()).Underlying().(*gotypes.Array)
		return ok
	default:
		return false
	}
}

func isBuiltinByte(value Type) bool {
	goType, ok := goTypeOf(value)
	return ok && gotypes.Identical(goType, gotypes.Typ[gotypes.Uint8])
}

func identicalCollectionElement(left, right Type) bool {
	if left.Kind == Nullable || right.Kind == Nullable {
		return left.Kind == Nullable && right.Kind == Nullable && left.Element != nil && right.Element != nil && identicalCollectionElement(*left.Element, *right.Element)
	}
	leftGo, leftOK := goTypeOf(left)
	rightGo, rightOK := goTypeOf(right)
	if leftOK && rightOK {
		return gotypes.Identical(leftGo, rightGo)
	}
	return left.Kind == right.Kind && left.Name == right.Name && (left.Kind == Class || left.Kind == Interface || left.Kind == Object)
}

func isCollectionElementType(value Type) bool {
	switch value.Kind {
	case Invalid, Void, GoPackage, GoTypeName, Nil, Null, MultiValue, Result:
		return false
	default:
		return true
	}
}

func isNullableBaseType(value Type) bool {
	switch value.Kind {
	case Class, Interface, Array, Map, Function, GoChannel, GoPointer:
		return true
	case Invalid, Void, Nil, Null, MultiValue, Result, Nullable, GoPackage, GoTypeName, Object, FixedArray:
		return false
	default:
		return isNilable(value)
	}
}
