package sema

import (
	"fmt"
	gotoken "go/token"
	gotypes "go/types"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type nativeInterfaceApplication struct {
	parameters, arguments []Type
	refs                  []ast.TypeRef
	span                  source.Span
	owner                 string
}

func (c *Checker) sourceInterfaceBoundIsDeferred(ref ast.TypeRef) bool {
	if c.nativeInterfacesReady {
		return false
	}
	seen := map[string]bool{}
	for ref.Qualifier == "" && !ref.Nullable && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() && !ref.IsGoStruct() {
		if seen[ref.Name] {
			return false
		}
		seen[ref.Name] = true
		if _, parameter := c.lookupTypeParameter(ref.Name); parameter {
			return false
		}
		if c.nativeInterfaceDeclarations[ref.Name] != nil {
			return true
		}
		alias, ok := c.nativeInterfaceAliases[ref.Name]
		if !ok {
			return false
		}
		ref = alias
	}
	return false
}

func (c *Checker) hasSourceInterfaceBounds(parameters []Type) bool {
	for _, parameter := range parameters {
		if identity, ok := parameter.GoType.(*gotypes.TypeParam); ok {
			if _, source := c.nativeInterfaceBounds[identity]; source || c.deferredParameterBounds[identity] {
				return true
			}
		}
	}
	return false
}

func (c *Checker) validatePendingNativeInterfaceArguments() {
	applications := c.pendingNativeInterfaceArguments
	c.pendingNativeInterfaceArguments = nil
	for _, application := range applications {
		c.validateNativeTypeArguments(application.parameters, application.arguments, application.refs, application.span, application.owner)
	}
}

// Keep source contracts separately from Go constraint storage. Native interface
// storage is provisional and Go signatures erase verified text and nullability.
func (c *Checker) nativeSourceInterfaceConstraint(bound Type, span source.Span) (gotypes.Type, bool) {
	if !c.nativeInterfacesReady {
		c.report(span, "source interface constraint is not ready; use a direct interface bound")
		return nil, false
	}
	methods, bindings, _, native := c.nativeConstraintMethods(bound)
	if !native {
		return nil, false
	}
	var functions []*gotypes.Func
	goNames := map[string]bool{}
	for _, name := range sortedNativeConstraintMethodNames(methods) {
		method := methods[name]
		if goNames[method.goName] {
			c.report(span, fmt.Sprintf("source interface constraint %s has conflicting Go method name %q", bound.String(), method.goName))
			return nil, false
		}
		goNames[method.goName] = true
		callable := substituteNativeTypeParameters(method.typeInfo, bindings)
		storage, ok := c.goTypeForNativeStorage(callable)
		signature, signatureOK := storage.(*gotypes.Signature)
		if !ok || !signatureOK {
			c.report(span, fmt.Sprintf("cannot represent source interface constraint method %q in Go", name))
			return nil, false
		}
		functions = append(functions, gotypes.NewFunc(gotoken.NoPos, nil, method.goName, signature))
	}
	return gotypes.NewInterfaceType(functions, nil).Complete(), true
}

func sortedNativeConstraintMethodNames(methods map[string]methodSymbol) []string {
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Match the actual method set, not addressable-call conveniences. In
// particular, a value struct cannot satisfy a pointer-receiver requirement.
func (c *Checker) sourceConstraintArgumentMethod(argument Type, goName string) (Type, bool) {
	if argument.Kind == Nullable {
		return Type{}, false
	}
	if methods, bindings, pointer, native := c.nativeConstraintMethods(argument); native {
		return nativeConstraintMethod(methods, bindings, pointer, goName)
	}
	storage, ok := goTypeOf(argument)
	if !ok {
		return Type{}, false
	}
	set := gotypes.NewMethodSet(storage)
	for i := 0; i < set.Len(); i++ {
		method := set.At(i).Obj()
		if method.Name() == goName && method.Exported() {
			callable, err := kinmokuseiTypeFromGo(method.Type())
			return callable, err == nil
		}
	}
	return Type{}, false
}

func (c *Checker) nativeSourceInterfaceArgumentSatisfies(bound, argument Type) bool {
	methods, bindings, _, native := c.nativeConstraintMethods(bound)
	if !native {
		return false
	}
	for _, name := range sortedNativeConstraintMethodNames(methods) {
		required := methods[name]
		provided, ok := c.sourceConstraintArgumentMethod(argument, required.goName)
		if !ok || !identicalMethodSignature(substituteNativeTypeParameters(required.typeInfo, bindings), provided) {
			return false
		}
	}
	return true
}

// Infer dependent arguments using source signatures, then commit only after
// every required method matches invariantly. A failed probe must not leak an
// erased or partially inferred string/nullable contract into a later fallback.
func (c *Checker) inferNativeSourceInterfaceArguments(bound, argument Type, bindings nativeTypeBindings) {
	methods, ownerBindings, _, native := c.nativeConstraintMethods(bound)
	if !native || argument.Kind == Nullable {
		return
	}
	trial := make(nativeTypeBindings, len(bindings))
	for parameter, value := range bindings {
		trial[parameter] = value
	}
	for _, name := range sortedNativeConstraintMethodNames(methods) {
		required := methods[name]
		formal := substituteNativeTypeParameters(required.typeInfo, ownerBindings)
		actual, ok := c.sourceConstraintArgumentMethod(argument, required.goName)
		if !ok || c.inferNativeTypeArguments(formal, actual, trial) != nil || !identicalMethodSignature(substituteNativeTypeParameters(formal, trial), actual) {
			return
		}
	}
	for parameter, value := range trial {
		bindings[parameter] = value
	}
}

func (c *Checker) checkSourceConstraintMember(expr *ast.MemberExpr, bound Type, write bool) Type {
	methods, bindings, _, _ := c.nativeConstraintMethods(bound)
	if _, getter := methods["get "+expr.Name]; getter {
		return c.checkPropertyAccess(expr, methods, bindings, write)
	}
	if _, setter := methods["set "+expr.Name]; setter {
		return c.checkPropertyAccess(expr, methods, bindings, write)
	}
	method, exists := methods[expr.Name]
	if !exists {
		c.report(expr.NameSpan, fmt.Sprintf("source interface constraint %s has no member %q", bound.String(), expr.Name))
		return Type{Kind: Invalid}
	}
	expr.ResolvedName, expr.ResolvedDeclaration = method.goName, method.declarationSpan
	return substituteNativeTypeParameters(method.typeInfo, bindings)
}
