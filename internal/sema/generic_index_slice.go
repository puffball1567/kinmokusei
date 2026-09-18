package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkMixedSequenceIndex(expr *ast.IndexExpr, object, index Type, checked bool) (Type, bool) {
	parameter, ok := object.GoType.(*gotypes.TypeParam)
	if object.Kind != TypeParameter || !ok {
		return Type{}, false
	}
	terms := c.collectionTerms(parameter)
	var element Type
	length := int64(-1)
	writable := true
	for i, term := range terms {
		shape := c.constraintArgumentShape(term)
		var next Type
		array := shape
		switch shape.Kind {
		case String:
			next, writable = builtins["byte"], false
		case Array:
			if shape.Element == nil {
				return Type{}, false
			}
			next = *shape.Element
		case FixedArray, GoPointer:
			if shape.Kind == GoPointer && shape.Element != nil {
				array = c.constraintArgumentShape(*shape.Element)
			}
			if array.Kind != FixedArray || array.Element == nil {
				return Type{}, false
			}
			next = *array.Element
			storage, ok := c.goTypeForNativeStorage(array)
			if !ok {
				return Type{}, false
			}
			fixed, ok := storage.Underlying().(*gotypes.Array)
			if !ok {
				return Type{}, false
			}
			if length < 0 || fixed.Len() < length {
				length = fixed.Len()
			}
			if shape.Kind == FixedArray && !c.isAddressableExpression(expr.Object) {
				writable = false
			}
		default:
			return Type{}, false
		}
		if i != 0 && !c.identicalCollectionElement(element, next) {
			return Type{}, false
		}
		element = next
	}
	if len(terms) == 0 {
		return Type{}, false
	}
	c.checkSequenceIndex(expr.Index, index, length, "array")
	expr.Addressable, expr.Assignable = writable, writable
	return c.checkedIndexResult(expr, element, checked, false), true
}

// Go 1.23's special string/[]byte slicing rule preserves T, but forbids the
// third index. Other mixed underlying shapes do not become sliceable here.
func (c *Checker) checkMixedTextSlice(expr *ast.SliceExpr, object Type) bool {
	parameter, ok := object.GoType.(*gotypes.TypeParam)
	if object.Kind != TypeParameter || !ok {
		return false
	}
	terms := c.collectionTerms(parameter)
	for _, term := range terms {
		shape := c.constraintArgumentShape(term)
		if shape.Kind != String && (shape.Kind != Array || shape.Element == nil || !isBuiltinByte(*shape.Element)) {
			return false
		}
	}
	if len(terms) == 0 {
		return false
	}
	if expr.Full {
		c.report(expr.Span, "3-index slice cannot be used with string")
	}
	c.checkSliceConstantBounds(expr, -1, "string")
	return true
}
