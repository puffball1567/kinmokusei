package sema

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	gotypes "go/types"
)

func (c *Checker) coalesceStaticHelper(expr *ast.BinaryExpr, left, right, result Type) string {
	call, ok := expr.Right.(*ast.CallExpr)
	if !ok || !call.StaticCallee || call.GenericCall || call.Conversion || call.Builtin != ast.NotBuiltinCall || len(call.Arguments) != 0 || len(call.NativeContractArguments) != 0 || left.Kind == Null || left.Kind == Nil {
		return ""
	}
	l, lo := c.goTypeForNativeStorage(baseOfNullable(left))
	r, ro := c.goTypeForNativeStorage(baseOfNullable(right))
	out, oo := c.goTypeForNativeStorage(baseOfNullable(result))
	if !lo || !ro || !oo || !gotypes.Identical(l, r) || !gotypes.Identical(l, out) {
		return ""
	}
	// A type parameter's underlying interface is a constraint, not runtime
	// interface storage. Boxing a nil pointer here would hide its nilness.
	if _, parameter := gotypes.Unalias(l).(*gotypes.TypeParam); parameter {
		return ""
	}
	switch typ := gotypes.Unalias(l).Underlying().(type) {
	case *gotypes.Pointer:
		return "Pointer"
	case *gotypes.Slice:
		return "Slice"
	case *gotypes.Map:
		return "Map"
	case *gotypes.Interface:
		return "Interface"
	case *gotypes.Chan:
		switch typ.Dir() {
		case gotypes.SendOnly:
			return "Send"
		case gotypes.RecvOnly:
			return "Receive"
		default:
			return "Channel"
		}
	}
	return ""
}

// Named public Go types hide their implementation fields; only names that
// actually appear in a generated result signature need to be accessible.
func coalesceTypeNameable(value Type) bool {
	// Bound structural expansion and memoize shared Go types. Nominal source
	// types expose only their name and type arguments in a generated signature.
	budget := 16384
	seen := map[gotypes.Type]bool{}
	var native func(Type, int) bool
	var named func(gotypes.Type, int) bool
	named = func(t gotypes.Type, depth int) bool {
		if t == nil || seen[t] {
			return true
		}
		budget--
		if budget < 0 || depth > 256 {
			return false
		}
		seen[t] = true
		switch t := t.(type) {
		case *gotypes.Named:
			if t.Obj().Pkg() != nil && !t.Obj().Exported() {
				return false
			}
			for i := 0; i < t.TypeArgs().Len(); i++ {
				if !named(t.TypeArgs().At(i), depth+1) {
					return false
				}
			}
		case *gotypes.Alias:
			return named(gotypes.Unalias(t), depth+1)
		case *gotypes.Pointer:
			return named(t.Elem(), depth+1)
		case *gotypes.Array:
			return named(t.Elem(), depth+1)
		case *gotypes.Slice:
			return named(t.Elem(), depth+1)
		case *gotypes.Map:
			return named(t.Key(), depth+1) && named(t.Elem(), depth+1)
		case *gotypes.Chan:
			return named(t.Elem(), depth+1)
		case *gotypes.Signature:
			for _, tuple := range []*gotypes.Tuple{t.Params(), t.Results()} {
				for i := 0; i < tuple.Len(); i++ {
					if !named(tuple.At(i).Type(), depth+1) {
						return false
					}
				}
			}
		case *gotypes.Struct:
			for i := 0; i < t.NumFields(); i++ {
				if !t.Field(i).Exported() || !named(t.Field(i).Type(), depth+1) {
					return false
				}
			}
		case *gotypes.Interface:
			for i := 0; i < t.NumMethods(); i++ {
				if !t.Method(i).Exported() || !named(t.Method(i).Type(), depth+1) {
					return false
				}
			}
		}
		return true
	}
	native = func(value Type, depth int) bool {
		budget--
		if budget < 0 || depth > 256 {
			return false
		}
		for _, argument := range value.TypeArguments {
			if !native(argument, depth+1) {
				return false
			}
		}
		if value.Kind == Class || value.Kind == Struct || value.Kind == Interface || value.Kind == TypeParameter {
			return true
		}
		if !named(value.GoType, depth) {
			return false
		}
		for _, child := range []*Type{value.Element, value.Key, value.Result} {
			if child != nil && !native(*child, depth+1) {
				return false
			}
		}
		for _, group := range [][]Type{value.Parameters, value.Results} {
			for _, child := range group {
				if !native(child, depth+1) {
					return false
				}
			}
		}
		for _, field := range value.Fields {
			if !native(field, depth+1) {
				return false
			}
		}
		return true
	}
	return native(value, 0)
}
