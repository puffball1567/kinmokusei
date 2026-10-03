package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Go storage cannot distinguish the source UTF-8 contracts.
func textContractMismatch(left, right Type) bool {
	return left.Kind == String && right.Kind == BString || left.Kind == BString && right.Kind == String
}

// An opaque Go interface can be used to mutate pointers/containers or invoke
// callbacks by reflection. Do not let it erase verified-text storage contracts.
func (c *Checker) textEscapesThroughGoInterface(value Type) bool {
	switch shape := c.constraintArgumentShape(value); shape.Kind {
	case Array, Map, GoPointer, Class, Function, Interface, GoInterface, GoChannel, TypeParameter:
		return c.containsVerifiedText(value, nil)
	case Struct, Object, FixedArray:
		// A by-value record/array copies its scalar text fields; references in
		// those fields still expose shared storage.
		for _, field := range shape.Fields {
			if c.textEscapesThroughGoInterface(field) {
				return true
			}
		}
		return shape.Element != nil && c.textEscapesThroughGoInterface(*shape.Element)
	}
	return false
}

func (c *Checker) containsVerifiedText(value Type, visiting []Type) bool {
	return c.containsTextContract(value, visiting, true)
}

// Some declaration checks defer open parameters until instantiation; opaque
// value boundaries instead treat an unbounded parameter conservatively.
func (c *Checker) containsDeclaredVerifiedText(value Type) bool {
	return c.containsTextContract(value, nil, false)
}

func (c *Checker) containsTextContract(value Type, visiting []Type, unknown bool) bool {
	value = c.restoreNativeRangeType(value)
	for _, seen := range visiting {
		if seen.Kind == value.Kind && seen.Name == value.Name && seen.GoType == value.GoType && (value.GoType != nil || value.Kind == Class || value.Kind == Struct || value.Kind == Interface) && exactType(seen, value) && sameConstraintNullability(seen, value) {
			return false
		}
	}
	visiting = append(visiting, value)
	shape := c.constraintArgumentShape(value)
	if shape.Kind == String {
		return true
	}
	if value.Kind == TypeParameter {
		if parameter, ok := value.GoType.(*gotypes.TypeParam); ok {
			terms := c.collectionTerms(parameter)
			// An unconstrained T may conceal verified text in shared storage.
			// A Go any boundary must not become a generic erasure loophole.
			if len(terms) == 0 {
				return unknown
			}
			for _, term := range terms {
				if c.containsTextContract(term, visiting, unknown) {
					return true
				}
			}
		}
	}
	if shape.Kind == Interface {
		if contract := c.interfaces[shape.Name]; contract != nil {
			bindings := nativeInterfaceBindings(contract, shape)
			for _, method := range contract.methods {
				if c.containsTextContract(substituteNativeTypeParameters(method.typeInfo, bindings), visiting, unknown) {
					return true
				}
			}
		}
		// Source interface assignment can hide an implementer's fields. Inspect
		// the checked program's implementations, not only its method signatures.
		for name, class := range c.classes {
			for _, declared := range class.implementedTypes {
				for _, ancestor := range c.interfaceAncestors(declared) {
					if ancestor.Name != shape.Name {
						continue
					}
					bindings := textContractOpenBindings(class.typeParameters)
					if err := c.inferNativeTypeArguments(ancestor, shape, bindings); err != nil {
						continue
					}
					instance, valid := c.textContractClassInstance(name, class, bindings)
					if valid && c.interfaceExtends(substituteNativeTypeParameters(declared, nativeClassBindings(class, instance)), shape) && c.containsTextContract(instance, visiting, unknown) {
						return true
					}
				}
			}
		}
	}
	if shape.Kind == GoInterface && c.classSatisfiesSourceAnonymousInterface(shape) {
		// Structural interfaces hide implementation fields just like named
		// source interfaces. Their method signatures alone are not sufficient
		// to decide whether reflection can reach verified-text storage.
		for name, class := range c.classes {
			if instance, ok := c.textContractStructuralInstance(name, class, shape); ok && c.containsTextContract(instance, visiting, unknown) {
				return true
			}
		}
	}
	if shape.Kind == Class {
		// A base-class reference may also hide fields added by a subclass.
		for name, candidate := range c.classes {
			if name != shape.Name {
				instance := Type{Kind: Class, Name: name, GoType: candidate.goNamed, TypeArguments: candidate.typeParameters}
				if c.classExtends(name, shape.Name) && c.containsTextContract(instance, visiting, unknown) {
					return true
				}
			}
		}
	}
	if shape.Kind == Class {
		if class := c.classes[shape.Name]; class != nil {
			bindings := nativeClassBindings(class, shape)
			for _, field := range class.fields {
				if !field.static && c.containsTextContract(substituteNativeTypeParameters(field.typeInfo, bindings), visiting, unknown) {
					return true
				}
			}
			for _, method := range class.methods {
				if !method.static && c.containsTextContract(substituteNativeTypeParameters(method.typeInfo, bindings), visiting, unknown) {
					return true
				}
			}
		}
	}
	for _, child := range []*Type{shape.Element, shape.Key, shape.Result} {
		if child != nil && c.containsTextContract(*child, visiting, unknown) {
			return true
		}
	}
	for _, children := range [][]Type{shape.Parameters, shape.Results, shape.TypeArguments} {
		for _, child := range children {
			if c.containsTextContract(child, visiting, unknown) {
				return true
			}
		}
	}
	for _, field := range shape.Fields {
		if c.containsTextContract(field, visiting, unknown) {
			return true
		}
	}
	for _, method := range shape.GoMethods {
		if c.containsTextContract(method.Type, visiting, unknown) {
			return true
		}
	}
	return false
}

// Infer the owner arguments visible in a structural method set. Arguments not
// exposed by any method remain open, so containsTextContract handles hidden
// generic fields conservatively rather than treating them as raw Go storage.
func (c *Checker) textContractStructuralInstance(name string, class *classSymbol, contract Type) (Type, bool) {
	bindings := textContractOpenBindings(class.typeParameters)
	for _, required := range contract.GoMethods {
		found := false
		for _, method := range class.methods {
			if method.static || method.goName != required.GoName {
				continue
			}
			if err := c.inferNativeTypeArguments(method.typeInfo, required.Type, bindings); err != nil {
				return Type{}, false
			}
			found = true
			break
		}
		if !found {
			return Type{}, false
		}
	}
	instance, valid := c.textContractClassInstance(name, class, bindings)
	return instance, valid && c.classSatisfiesGoInterface(class, instance, contract)
}

func textContractOpenBindings(parameters []Type) nativeTypeBindings {
	bindings := make(nativeTypeBindings, len(parameters))
	for _, parameter := range parameters {
		bindings[parameter.GoType] = Type{Kind: Invalid}
	}
	return bindings
}

func (c *Checker) textContractClassInstance(name string, class *classSymbol, bindings nativeTypeBindings) (Type, bool) {
	arguments := append([]Type(nil), class.typeParameters...)
	goBindings := make(map[gotypes.Type]gotypes.Type, len(arguments))
	nativeBindings := make(nativeTypeBindings, len(arguments))
	for index, parameter := range arguments {
		if argument := bindings[parameter.GoType]; argument.Kind != Invalid {
			arguments[index] = argument
		}
		if storage, ok := c.goTypeForNativeStorage(arguments[index]); ok {
			goBindings[parameter.GoType] = storage
		}
		nativeBindings[parameter.GoType] = arguments[index]
	}
	for index, parameter := range class.typeParameters {
		if bindings[parameter.GoType].Kind == Invalid {
			continue
		}
		if !c.nativeTypeArgumentSatisfies(parameter, arguments[index], goBindings) {
			return Type{}, false
		}
		if goParameter, ok := parameter.GoType.(*gotypes.TypeParam); ok {
			shape, hasShape := c.parameterRangeShape(goParameter)
			if hasShape && !sameConstraintNullability(substituteNativeTypeParameters(shape, nativeBindings), c.constraintArgumentShape(arguments[index])) || !c.collectionArgumentNullabilityMatches(goParameter, arguments[index], nativeBindings) {
				return Type{}, false
			}
		}
	}
	instance := Type{Kind: Class, Name: name, GoType: class.goNamed, TypeArguments: arguments}
	return instance, true
}

func (c *Checker) checkTextConversion(expr *ast.CallExpr, target, value Type) (Type, bool) {
	shape := c.constraintArgumentShape(target)
	if shape.Kind != String && shape.Kind != BString {
		return Type{}, false
	}
	targetGo, targetOK := goTypeOf(target)
	valueGo, valueOK := goTypeOf(value)
	if c.hasDeferredShift(expr.Arguments[0]) {
		return c.checkComplexConversion(expr, target, value), true
	}
	if !targetOK || !valueOK || value.Kind == Nullable || !gotypes.ConvertibleTo(valueGo, targetGo) {
		c.report(expr.Span, fmt.Sprintf("cannot convert %s to %s", value.String(), target.String()))
		return target, true
	}
	// Code-point conversions encode UTF-8; invalid runes become U+FFFD.
	if value.IsInteger() {
		return c.checkComplexConversion(expr, target, value), true
	}
	if shape.Kind == BString || c.constraintArgumentShape(value).Kind == String {
		return target, true
	}
	if slice, ok := gotypes.Unalias(valueGo).Underlying().(*gotypes.Slice); ok {
		if basic, ok := gotypes.Unalias(slice.Elem()).Underlying().(*gotypes.Basic); ok && basic.Kind() == gotypes.Int32 {
			return target, true
		}
	}
	expr.Conversion = false
	expr.Builtin = ast.DecodeUTF8Call
	ref := typeRefFromType(target, expr.Span)
	expr.ConversionType = &ref
	c.usesUTF8 = true
	result := Type{Kind: Result, Name: "Result", Element: &target}
	expr.Signature = &ast.CallableSignature{ParameterNames: []string{"raw"}, ParameterTypes: []string{value.String()}, Result: result.String()}
	return result, true
}
