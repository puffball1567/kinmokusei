package sema

import (
	"fmt"
	"github.com/puffball1567/kinmokusei/internal/ast"
	gotypes "go/types"
)

func (c *Checker) checkIndex(expr *ast.IndexExpr, checked bool) Type {
	return c.checkIndexAccess(expr, checked, false)
}

// Targets and explicit whole-value discards need the storage element type;
// neither can expose an unchecked missing payload as an ordinary source value.
func (c *Checker) checkIndexAccess(expr *ast.IndexExpr, checked, storage bool) Type {
	finish := func(element Type, mapIndex bool) Type {
		if storage && mapIndex {
			expr.MapIndex = true
			return element
		}
		return c.checkedIndexResult(expr, element, checked, mapIndex)
	}
	if _, _, found := c.nativeMethodExpressionReceiver(expr); found {
		c.report(expr.Span, "a native instantiated type cannot be used as a value")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	var object Type
	if len(expr.TypeArguments) != 0 {
		object = c.genericValueOperand(expr.Object)
		if object.Kind == GoTypeName {
			return c.checkGoTypeInstantiationExpression(expr, object)
		}
		if object.Generic || expr.Index == nil {
			return c.checkGenericFunctionValue(expr, object)
		}
	} else {
		object = c.checkExpression(expr.Object)
	}
	object = c.singleValue(object, expr.Object.GetSpan())
	if object.Kind == Nullable {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("nullable value %s must be checked against null before indexing", object.String()))
		if object.Element == nil {
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		object = *object.Element
	}
	if !c.mapConstraintContractsAgree(object) {
		c.report(expr.Span, fmt.Sprintf("type %s cannot be indexed: map indexing requires identical key/value types and nullability in every constraint alternative", object.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	object = c.collectionOperandShape(object)
	index := c.singleValue(c.checkExpression(expr.Index), expr.Index.GetSpan())
	if object.Kind == Invalid {
		return object
	}
	if result, ok := c.checkMixedSequenceIndex(expr, object, index, checked); ok {
		return result
	}
	switch object.Kind {
	case Array:
		c.checkSequenceIndex(expr.Index, index, -1, "array")
		expr.Addressable = true
		expr.Assignable = true
		if object.Element != nil {
			return finish(*object.Element, false)
		}
	case Map:
		expr.Assignable = true
		if object.Key != nil {
			c.checkMapIndex(expr, *object.Key, index)
		}
		if object.Element != nil {
			return finish(*object.Element, true)
		}
	case String, BString:
		c.checkSequenceIndex(expr.Index, index, c.constantStringLength(expr.Object), "string")
		return finish(builtins["byte"], false)
	}
	goType, ok := goTypeOf(object)
	if !ok {
		goType, ok = c.goTypeForNativeStorage(object)
	}
	if !ok {
		c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be indexed", object.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	underlying := gotypes.Unalias(goType).Underlying()
	if pointer, pointerOK := underlying.(*gotypes.Pointer); pointerOK {
		if array, arrayOK := gotypes.Unalias(pointer.Elem()).Underlying().(*gotypes.Array); arrayOK {
			c.checkSequenceIndex(expr.Index, index, array.Len(), "array")
			expr.Addressable = true
			expr.Assignable = true
			return finish(c.fixedArrayElementType(array, object, expr.Span), false)
		}
	}
	switch collection := underlying.(type) {
	case *gotypes.Array:
		c.checkSequenceIndex(expr.Index, index, collection.Len(), "array")
		expr.FixedArrayStorage = c.isAddressableExpression(expr.Object)
		expr.Addressable = c.isAddressableExpression(expr.Object)
		expr.Assignable = expr.Addressable
		return finish(c.fixedArrayElementType(collection, object, expr.Span), false)
	case *gotypes.Slice:
		c.checkSequenceIndex(expr.Index, index, -1, "array")
		expr.Addressable = true
		expr.Assignable = true
		if object.Element != nil {
			return finish(*object.Element, false)
		}
		return finish(c.collectionElementType(collection.Elem(), object, expr.Span), false)
	case *gotypes.Map:
		expr.Assignable = true
		key := c.collectionElementType(collection.Key(), object, expr.Span)
		if object.Key != nil {
			key = *object.Key
		}
		c.checkMapIndex(expr, key, index)
		if object.Element != nil {
			return finish(*object.Element, true)
		}
		return finish(c.collectionElementType(collection.Elem(), object, expr.Span), true)
	case *gotypes.Basic:
		if collection.Info()&gotypes.IsString != 0 {
			c.checkSequenceIndex(expr.Index, index, c.constantStringLength(expr.Object), "string")
			return finish(builtins["byte"], false)
		}
	}
	c.report(expr.Object.GetSpan(), fmt.Sprintf("type %s cannot be indexed", object.String()))
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) checkedIndexResult(expr *ast.IndexExpr, element Type, checked, mapIndex bool) Type {
	expr.MapIndex = mapIndex
	if !checked {
		if mapIndex && !c.zeroValueSafe(element) {
			if isNullableBaseType(element) {
				return Type{Kind: Nullable, Name: "nullable", Element: &element}
			}
			c.report(expr.Span, fmt.Sprintf("map value %s has no safe zero value; use [value, ok] and check ok before using value", element.String()))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		return element
	}
	if !mapIndex {
		c.report(expr.Span, "checked index binding requires a map operand")
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	return Type{Kind: MultiValue, Name: "checked map lookup", Results: []Type{element, builtins["boolean"]}}
}

// Common Go storage can erase source nullability. Only index a constrained
// map when every alternative promises the same key and payload contracts.
func (c *Checker) mapConstraintContractsAgree(value Type) bool {
	parameter, ok := value.GoType.(*gotypes.TypeParam)
	if value.Kind != TypeParameter || !ok {
		return true
	}
	terms := c.collectionTerms(parameter)
	var first Type
	for _, term := range terms {
		shape := c.constraintArgumentShape(term)
		if shape.Kind != Map {
			return true
		} // Other collection rules diagnose it.
		if first.Kind == Invalid {
			first = shape
			continue
		}
		if first.Key == nil || first.Element == nil || shape.Key == nil || shape.Element == nil || !c.identicalCollectionElement(*first.Key, *shape.Key) || !c.identicalCollectionElement(*first.Element, *shape.Element) {
			return false
		}
	}
	return true
}
