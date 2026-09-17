package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) prepareForRange(stmt *ast.ForRangeStmt) []Type {
	sourceType := c.singleValue(c.checkExpression(stmt.Source), stmt.Source.GetSpan())
	stmt.GuaranteedNonEmpty = rangeTypeGuaranteedNonEmpty(sourceType) || c.rangeExpressionGuaranteedNonEmpty(stmt.Source)
	key, value, kind := c.rangeBindingTypes(sourceType, stmt.Source.GetSpan())
	stmt.Kind = kind
	if kind == ast.IteratorRange && len(stmt.Bindings) != 1 {
		c.report(stmt.Span, "single-value iterator range requires exactly one binding")
	}
	if kind == ast.IteratorZeroRange && (len(stmt.Bindings) != 1 || stmt.Bindings[0].Name != "_" || stmt.Bindings[0].Type.IsSpecified()) {
		c.report(stmt.Span, "zero-value iterator range requires a single untyped '_' binding")
	}
	if kind == ast.IntegerRange {
		if len(stmt.Bindings) != 1 {
			c.report(stmt.Span, fmt.Sprintf("integer range requires exactly one binding, got %d", len(stmt.Bindings)))
		}
		if integer, known := c.resolvedIntegerConstantValue(stmt.Source); known {
			stmt.GuaranteedNonEmpty = integer.Sign() > 0
			if !integerConstantFitsFixedType(integer, value) || sourceType.Kind == UntypedInt && !integer.IsInt64() {
				c.report(stmt.Source.GetSpan(), fmt.Sprintf("integer range bound overflows %s", value.String()))
			}
		}
	}
	if kind == ast.ChannelRange && len(stmt.Bindings) != 1 {
		c.report(stmt.Span, fmt.Sprintf("channel range requires exactly one binding, got %d", len(stmt.Bindings)))
	}
	if (kind == ast.CollectionRange || kind == ast.IteratorPairRange) && (len(stmt.Bindings) < 1 || len(stmt.Bindings) > 2) {
		c.report(stmt.Span, fmt.Sprintf("collection range requires one or two bindings, got %d", len(stmt.Bindings)))
	}
	types := []Type{value}
	if len(stmt.Bindings) == 2 {
		types = []Type{key, value}
	}
	return types
}

func rangeTypeGuaranteedNonEmpty(t Type) bool {
	if parameter, ok := t.GoType.(*gotypes.TypeParam); ok && t.Kind == TypeParameter {
		core := goRangeCoreType(parameter)
		if pointer, ok := core.(*gotypes.Pointer); ok {
			core = gotypes.Unalias(pointer.Elem()).Underlying()
		}
		if array, ok := core.(*gotypes.Array); ok {
			return array.Len() > 0
		}
	}
	if t.Kind == FixedArray {
		return t.Length > 0
	}
	return t.Kind == GoPointer && t.Element != nil && t.Element.Kind == FixedArray && t.Element.Length > 0
}

func (c *Checker) checkForRangeBody(stmt *ast.ForRangeStmt, types []Type) {
	c.pushScope()
	for index := range stmt.Bindings {
		binding := &stmt.Bindings[index]
		actual := Type{Kind: Invalid, Name: "<invalid>"}
		if index < len(types) {
			actual = types[index]
		}
		declared := actual
		if binding.Type.IsSpecified() {
			declared = c.resolveType(binding.Type)
			c.requireAssignable(declared, actual, binding.NameSpan)
			if stmt.Kind == ast.IntegerRange && declared.Kind != Invalid && actual.Kind != Invalid && c.isAssignable(declared, actual) && !exactType(declared, actual) {
				c.report(binding.NameSpan, fmt.Sprintf("integer range binding must have the bound's type %s", actual.String()))
			}
			if (stmt.Kind == ast.IteratorRange || stmt.Kind == ast.IteratorPairRange) && declared.Kind != Invalid && actual.Kind != Invalid && c.isAssignable(declared, actual) && !exactType(declared, actual) {
				c.report(binding.NameSpan, fmt.Sprintf("iterator range binding must have the yielded type %s", actual.String()))
			}
			if stmt.Kind == ast.CollectionRange && declared.Kind != Invalid && actual.Kind != Invalid && c.isAssignable(declared, actual) && !exactType(declared, actual) {
				c.report(binding.NameSpan, fmt.Sprintf("collection range binding must have the iterated type %s", actual.String()))
			}
		}
		if binding.Name != "_" {
			binding.ResolvedType = typeRefFromType(declared, binding.NameSpan)
			c.declareRangeLocal(binding, declared, stmt.Constant)
		}
	}
	c.loopDepth++
	c.checkBlock(stmt.Body, true)
	c.loopDepth--
	c.popScope()
}

func (c *Checker) rangeBindingTypes(sourceType Type, span source.Span) (Type, Type, ast.ForRangeKind) {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	nullableSource := sourceType.Kind == Nullable
	if sourceType.Kind == TypeParameter && sourceType.IsInteger() {
		mask := goTypeSetMask(sourceType.GoType, map[gotypes.Type]bool{})
		if mask != 0 && mask&(mask-1) == 0 {
			return invalid, sourceType, ast.IntegerRange
		}
		c.report(span, "integer range type parameter requires a single underlying integer type")
		return invalid, invalid, ast.UnknownRange
	}
	if sourceType.Kind == Nullable && sourceType.Element != nil {
		if _, ok := rangeFunctionType(*sourceType.Element); ok {
			c.report(span, "nullable iterator must be narrowed before range")
			return invalid, invalid, ast.UnknownRange
		}
		sourceType = *sourceType.Element
	}
	if sourceType.Kind == TypeParameter {
		parameter, ok := sourceType.GoType.(*gotypes.TypeParam)
		if ok {
			if shape, valid := c.parameterRangeShape(parameter); valid {
				if nullableSource && shape.Kind == Function {
					c.report(span, "nullable iterator must be narrowed before range")
					return invalid, invalid, ast.UnknownRange
				}
				inheritGoQualifier(&shape, sourceType)
				return c.rangeBindingTypes(shape, span)
			}
		}
		c.report(span, "range type parameter requires a common underlying range type or compatible receive-capable channels")
		return invalid, invalid, ast.UnknownRange
	}
	if callable, ok := rangeFunctionType(sourceType); ok {
		return c.iteratorBindingTypes(callable, span)
	}
	goType, ok := goTypeOf(sourceType)
	if !ok {
		goType, ok = c.goTypeForNativeStorage(sourceType)
	}
	if !ok {
		if sourceType.Kind != Invalid {
			c.report(span, fmt.Sprintf("range requires an integer, array, slice, map, string, receive-capable Go channel, or iterator function, got %s", sourceType.String()))
		}
		return invalid, invalid, ast.UnknownRange
	}
	underlying := gotypes.Unalias(goType).Underlying()
	if pointer, pointerOK := underlying.(*gotypes.Pointer); pointerOK {
		if _, arrayOK := gotypes.Unalias(pointer.Elem()).Underlying().(*gotypes.Array); arrayOK {
			underlying = gotypes.Unalias(pointer.Elem()).Underlying()
		}
	}
	switch ranged := underlying.(type) {
	case *gotypes.Chan:
		if ranged.Dir() == gotypes.SendOnly {
			c.report(span, fmt.Sprintf("cannot range over send-only channel %s", sourceType.String()))
			return invalid, invalid, ast.UnknownRange
		}
		element := c.restoreNativeRangeType(c.collectionElementType(ranged.Elem(), sourceType, span))
		if sourceType.Element != nil {
			element = *sourceType.Element
		}
		return invalid, element, ast.ChannelRange
	case *gotypes.Array:
		element := c.restoreNativeRangeType(c.collectionElementType(ranged.Elem(), sourceType, span))
		if sourceType.Kind == FixedArray && sourceType.Element != nil {
			element = *sourceType.Element
		}
		return builtins["int"], element, ast.CollectionRange
	case *gotypes.Slice:
		element := c.restoreNativeRangeType(c.collectionElementType(ranged.Elem(), sourceType, span))
		if sourceType.Kind == Array && sourceType.Element != nil {
			element = *sourceType.Element
		}
		return builtins["int"], element, ast.CollectionRange
	case *gotypes.Map:
		key := c.restoreNativeRangeType(c.collectionElementType(ranged.Key(), sourceType, span))
		value := c.restoreNativeRangeType(c.collectionElementType(ranged.Elem(), sourceType, span))
		if sourceType.Key != nil {
			key = *sourceType.Key
		}
		if sourceType.Element != nil {
			value = *sourceType.Element
		}
		return key, value, ast.CollectionRange
	case *gotypes.Basic:
		if ranged.Info()&gotypes.IsInteger != 0 {
			return invalid, defaultLiteralType(sourceType), ast.IntegerRange
		}
		if ranged.Info()&gotypes.IsString != 0 {
			return builtins["int"], builtins["int32"], ast.CollectionRange
		}
	}
	if sourceType.Kind != Invalid {
		c.report(span, fmt.Sprintf("range requires an integer, array, slice, map, string, receive-capable Go channel, or iterator function, got %s", sourceType.String()))
	}
	return invalid, invalid, ast.UnknownRange
}

func rangeFunctionType(value Type) (Type, bool) {
	if value.Kind == Function {
		return value, true
	}
	if goType, ok := goTypeOf(value); ok {
		if signature, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Signature); ok {
			converted, err := kinmokuseiFunctionFromGo(signature)
			return converted, err == nil
		}
	}
	return Type{}, false
}

func (c *Checker) iteratorBindingTypes(callable Type, span source.Span) (Type, Type, ast.ForRangeKind) {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	if callable.Generic || callable.Variadic || len(callable.Parameters) != 1 || callable.Result == nil || callable.Result.Kind != Void {
		c.report(span, "range iterator must be a non-generic, non-variadic function taking one yield callback and returning void")
		return invalid, invalid, ast.UnknownRange
	}
	yield, ok := rangeFunctionType(callable.Parameters[0])
	if !ok || yield.Generic || yield.Variadic || len(yield.Parameters) > 2 || yield.Result == nil || !exactType(*yield.Result, builtins["boolean"]) {
		c.report(span, "range iterator yield callback must take zero, one, or two values and return boolean")
		return invalid, invalid, ast.UnknownRange
	}
	for index := range yield.Parameters {
		c.prepareGoTypeForEmission(&yield.Parameters[index], span)
	}
	switch len(yield.Parameters) {
	case 0:
		return invalid, invalid, ast.IteratorZeroRange
	case 1:
		return invalid, yield.Parameters[0], ast.IteratorRange
	default:
		return yield.Parameters[0], yield.Parameters[1], ast.IteratorPairRange
	}
}
