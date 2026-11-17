package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Zero-safety concerns the value itself, not the contents of referenced storage:
// a nil slice/map/channel is valid, but a zero non-null class/pointer/function
// is not. Nullable values terminate the recursion without erasing their contract.
func (c *Checker) zeroValueSafe(value Type) bool {
	state := zeroValueCheck{checker: c, remaining: 32768, records: map[string]uint8{}, storage: map[gotypes.Type]uint8{}}
	return state.check(value, 0)
}

type zeroValueCheck struct {
	checker   *Checker
	remaining int
	records   map[string]uint8
	storage   map[gotypes.Type]uint8
}

func (s *zeroValueCheck) check(value Type, depth int) bool {
	if s.remaining == 0 || depth > 256 {
		return false // An unproved zero value must never create a non-null reference.
	}
	s.remaining--
	check := func(child Type) bool { return s.check(child, depth+1) }
	switch value.Kind {
	case Invalid, Boolean, String, BString, Int, Int8, Int16, Int32, Int64, Uint, Uint16, Uint32, Uint64, Byte, Float32, Float64, UntypedInt, Nullable, Array, Map, GoChannel:
		return true
	case Class, Interface, Function, GoPointer, GoInterface:
		return false
	case FixedArray:
		return value.Length == 0 || value.Element != nil && check(*value.Element)
	case Object:
		for _, field := range value.Fields {
			if !check(field) {
				return false
			}
		}
		return true
	case Struct:
		// Read canonical fields: forward/indirect references may retain an early
		// field snapshot. Generic bindings must preserve source nullability.
		structure := s.checker.structs[value.Name]
		if structure == nil {
			return false
		}
		if len(structure.typeParameters) == 0 {
			if memo := s.records[value.Name]; memo != 0 {
				return memo == 2
			}
			s.records[value.Name] = 1
		}
		bindings := nativeStructBindings(structure, value)
		for _, field := range structure.fields {
			if !check(substituteNativeTypeParameters(field.typeInfo, bindings)) {
				return false
			}
		}
		if len(structure.typeParameters) == 0 {
			s.records[value.Name] = 2
		}
		return true
	case TypeParameter:
		if parameter, ok := value.GoType.(*gotypes.TypeParam); ok {
			terms := s.checker.collectionTerms(parameter)
			if len(terms) == 0 {
				return false
			}
			for _, term := range terms {
				if !check(term) {
					return false
				}
			}
			return true
		}
		return false
	}
	if shape, native := s.checker.nativeDefinedShape(value); native {
		return check(shape)
	}
	storage, ok := goTypeOf(value)
	if !ok {
		return false
	}
	if memo := s.storage[storage]; memo != 0 {
		return memo == 2
	}
	s.storage[storage] = 1
	safe := false
	switch underlying := gotypes.Unalias(storage).Underlying().(type) {
	case *gotypes.Basic:
		safe = underlying.Info()&(gotypes.IsBoolean|gotypes.IsNumeric|gotypes.IsString) != 0
	case *gotypes.Slice, *gotypes.Map, *gotypes.Chan:
		safe = true
	case *gotypes.Array:
		safe = underlying.Len() == 0 || s.goField(underlying.Elem(), depth+1)
	case *gotypes.Struct:
		safe = true
		for i := 0; i < underlying.NumFields(); i++ {
			field := underlying.Field(i)
			// Hidden Go implementation state is a platform boundary, not a source
			// field contract (for example time.Time's optional location pointer).
			if field.Exported() && !s.goField(field.Type(), depth+1) {
				safe = false
				break
			}
		}
	}
	if safe {
		s.storage[storage] = 2
	}
	return safe
}

func (s *zeroValueCheck) goField(storage gotypes.Type, depth int) bool {
	value, err := kinmokuseiTypeFromGo(storage)
	return err == nil && s.check(s.checker.restoreNativeRangeType(value), depth)
}

func (c *Checker) checkZeroSliceAllocation(name string, expr *ast.CallExpr, element Type, safe bool) {
	if len(expr.Arguments) == 0 || safe {
		return
	}
	if length, known := c.integerContextValue(expr.Arguments[0]); known && length.Sign() == 0 {
		return
	}
	c.report(expr.Arguments[0].GetSpan(), fmt.Sprintf("%s cannot initialize elements of %s with their zero value; use a nullable element type or length 0 and append initialized values", name, element.String()))
}

// A common Go slice representation may erase different source contracts in a
// union of named types. Every alternative must be safe, not only the core shape.
func (c *Checker) sliceElementsZeroSafe(value Type, span source.Span) bool {
	if value.Kind == Nullable && value.Element != nil {
		return c.sliceElementsZeroSafe(*value.Element, span)
	}
	if parameter, ok := value.GoType.(*gotypes.TypeParam); value.Kind == TypeParameter && ok {
		terms := c.collectionTerms(parameter)
		for _, term := range terms {
			if !c.sliceElementsZeroSafe(term, span) {
				return false
			}
		}
		return len(terms) != 0
	}
	element, ok := c.sliceElementType(value, span)
	return ok && c.zeroValueSafe(element)
}

func (c *Checker) clearPreservesZeroSafety(value Type, span source.Span) bool {
	if value.Kind == Nullable && value.Element != nil {
		return c.clearPreservesZeroSafety(*value.Element, span)
	}
	if parameter, ok := value.GoType.(*gotypes.TypeParam); value.Kind == TypeParameter && ok {
		terms := c.collectionTerms(parameter)
		for _, term := range terms {
			if !c.clearPreservesZeroSafety(term, span) {
				return false
			}
		}
		return len(terms) != 0
	}
	if element, slice := c.sliceElementType(value, span); slice {
		return c.zeroValueSafe(element)
	}
	return true // Clearing a map removes entries rather than producing payloads.
}
