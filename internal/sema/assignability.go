package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) requireAssignable(target, value Type, span source.Span) {
	if value.Kind == GoPackage {
		c.report(span, "a Go package namespace cannot be used as a value")
		return
	}
	if value.Kind == GoTypeName {
		c.report(span, fmt.Sprintf("Go type %s cannot be used as a value", value.String()))
		return
	}
	if value.Kind == MultiValue {
		c.report(span, fmt.Sprintf("multiple values %s require destructuring", value.String()))
		return
	}
	if value.Kind == Result {
		c.report(span, resultUsageMessage)
		return
	}
	if !c.isAssignable(target, value) {
		c.report(span, fmt.Sprintf("cannot use %s as %s", value.String(), target.String()))
	}
}

func (c *Checker) inferredVariableType(value Type, span source.Span) Type {
	switch value.Kind {
	case Nil, Null:
		c.report(span, "cannot infer a variable type from nil or null; add an explicit nilable or nullable type")
		return Type{Kind: Invalid, Name: "<invalid>"}
	case GoPackage, GoTypeName:
		return value
	case MultiValue:
		c.report(span, fmt.Sprintf("multiple values %s require destructuring", value.String()))
		return Type{Kind: Invalid, Name: "<invalid>"}
	case Result:
		c.report(span, resultUsageMessage)
		return Type{Kind: Invalid, Name: "<invalid>"}
	default:
		return defaultLiteralType(value)
	}
}

func (c *Checker) isAssignable(target, value Type) bool {
	if target.Kind == Invalid || value.Kind == Invalid {
		return true
	}
	if !compatibleResultFunctionTypes(c.callableType(target), c.callableType(value)) {
		return false
	}
	if target.Kind == Nullable && target.Element != nil {
		if value.Kind == Null {
			return true
		}
		if value.Kind == Nil {
			return false
		}
		if value.Kind == Nullable {
			return value.Element != nil && c.isAssignable(*target.Element, *value.Element)
		}
		return c.isAssignable(*target.Element, value)
	}
	if value.Kind == Nullable || value.Kind == Null || target.Kind == Null {
		return value.Kind == Null && target.Kind == Null
	}
	if target.Kind == Interface && value.Kind == Class {
		class := c.classes[value.Name]
		if class == nil {
			return false
		}
		bindings := nativeClassBindings(class, value)
		for _, implemented := range class.implementedTypes {
			if c.interfaceExtends(substituteNativeTypeParameters(implemented, bindings), target) {
				return true
			}
		}
		return false
	}
	if target.Kind == Interface && value.Kind == Interface {
		return c.interfaceExtends(value, target)
	}
	if value.Kind == Interface && (target.Kind == GoNamed || target.Kind == GoInterface) && underlyingGoInterface(target.GoType) != nil {
		if underlyingGoInterface(target.GoType).NumMethods() == 0 {
			return true
		}
		return c.interfaceHasGoAncestor(value, target.GoType)
	}
	if target.Kind == Class && value.Kind == Class {
		if ancestor, ok := c.classAncestorType(value, target.Name); ok {
			return exactType(target, ancestor)
		}
	}
	// A type parameter's underlying interface is a constraint, not an
	// interface value destination. Satisfying any/comparable does not make a
	// concrete receiver assignable to every possible instantiation of T.
	if target.Kind == TypeParameter && (value.Kind == Class || value.Kind == Struct) {
		storage, ok := c.goTypeForNativeStorage(value)
		return ok && target.GoType != nil && gotypes.AssignableTo(storage, target.GoType)
	}
	if value.Kind == Struct {
		if contract := underlyingGoInterface(target.GoType); contract != nil && contract.NumMethods() == 0 {
			return true
		}
	}
	if value.Kind == Class && underlyingGoInterface(target.GoType) != nil {
		if underlyingGoInterface(target.GoType).NumMethods() == 0 {
			return true
		}
		class := c.classes[value.Name]
		if class == nil {
			return false
		}
		// Source anonymous interfaces are structural at the value boundary.
		// Match their exported Go method set against the class's lowered methods
		// even when the class did not declare a named `implements` contract.
		if c.classSatisfiesGoInterface(class, target.GoType) {
			return true
		}
		for _, declared := range class.goImplements {
			if gotypes.AssignableTo(declared, target.GoType) || gotypes.Identical(declared, target.GoType) {
				return true
			}
		}
		bindings := nativeClassBindings(class, value)
		for _, implemented := range class.implementedTypes {
			if c.interfaceHasGoAncestor(substituteNativeTypeParameters(implemented, bindings), target.GoType) {
				return true
			}
		}
		return false
	}
	return assignable(target, value)
}

func (c *Checker) classSatisfiesGoInterface(class *classSymbol, target gotypes.Type) bool {
	contract := underlyingGoInterface(target)
	if contract == nil {
		return false
	}
	for index := 0; index < contract.NumMethods(); index++ {
		required := contract.Method(index)
		var provided *methodSymbol
		for name := range class.methods {
			method := class.methods[name]
			if method.goName == required.Name() && !method.static {
				copy := method
				provided = &copy
				break
			}
		}
		if provided == nil {
			return false
		}
		actual, ok := goTypeOf(provided.typeInfo)
		if !ok || !gotypes.Identical(actual, required.Type()) {
			return false
		}
	}
	return true
}

func exactType(left, right Type) bool { return assignable(left, right) && assignable(right, left) }
