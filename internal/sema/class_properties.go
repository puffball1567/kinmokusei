package sema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Accessors share the method representation, but not the source method namespace.
// Spaces cannot occur in source identifiers, so these keys cannot be called or
// accidentally satisfy source method contracts.
func hasClassProperty(class *classSymbol, name string) bool {
	return hasProperty(class.methods, name)
}

func hasProperty(methods map[string]methodSymbol, name string) bool {
	_, get := methods["get "+name]
	_, set := methods["set "+name]
	return get || set
}

func (c *Checker) declareClassAccessor(decl *ast.ClassDecl, class *classSymbol, method *ast.MethodDecl) {
	if method.Static {
		c.report(method.Span, "static properties are not supported; use an instance accessor or static method")
	}
	c.declareAbstractMethod(decl, method)
	c.validateLabels(method.Body)
	key := method.Accessor + " " + method.Name
	inherited, replaces := class.methods[key]
	for _, accessor := range []string{"get ", "set "} {
		if other, exists := class.methods[accessor+method.Name]; !replaces && exists && other.declaringClass != decl.Name {
			c.report(method.NameSpan, fmt.Sprintf("inherited property %q cannot be redeclared by adding an accessor", method.Name))
		}
	}
	if _, exists := class.fields[method.Name]; exists {
		c.report(method.NameSpan, fmt.Sprintf("property %q conflicts with a field", method.Name))
	}
	if _, exists := class.methods[method.Name]; exists {
		c.report(method.NameSpan, fmt.Sprintf("property %q conflicts with a method", method.Name))
	}
	parameters := make([]Type, len(method.Parameters))
	for i, parameter := range method.Parameters {
		parameters[i] = c.resolveType(parameter.Type)
	}
	result := c.resolveType(method.ReturnType)
	c.checkAccessorSignature(method.Accessor, parameters, result, hasVariadicParameter(method.Parameters), method.Span)
	method.GoName = memberGoName(method.Accessor+memberGoName(method.Name, ast.Public), method.Visibility)
	signature := Type{Kind: Function, Name: "function", Parameters: parameters, Result: &result}
	owner, valid := c.methodDispatchOwner(decl, method, signature, inherited, replaces)
	if !valid {
		return
	}
	method.VirtualOwner = owner
	class.methods[key] = methodSymbol{typeInfo: signature,
		visibility: method.Visibility, goName: method.GoName, declarationSpan: method.NameSpan, declaringClass: decl.Name,
		virtual: method.Virtual || method.Override, abstract: method.Abstract, final: method.Final, virtualOwner: owner}
}

func (c *Checker) checkClassPropertyContracts(decl *ast.ClassDecl, class *classSymbol) {
	for _, method := range decl.Methods {
		if method.Accessor == "" {
			continue
		}
		getter, get := class.methods["get "+method.Name]
		setter, set := class.methods["set "+method.Name]
		if get && set && len(setter.typeInfo.Parameters) == 1 && getter.typeInfo.Result != nil {
			left := Type{Kind: Function, Name: "function", Result: getter.typeInfo.Result}
			right := Type{Kind: Function, Name: "function", Result: &setter.typeInfo.Parameters[0]}
			if !identicalMethodSignature(left, right) {
				c.report(method.NameSpan, fmt.Sprintf("getter and setter for %q must have identical types, including nullability", method.Name))
			}
		}
	}
	// A new ordinary method/field must not shadow the Go implementation of an
	// inherited accessor. Include promoted members, not only local properties.
	keys := make([]string, 0, len(class.methods))
	for key := range class.methods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !strings.HasPrefix(key, "get ") && !strings.HasPrefix(key, "set ") {
			continue
		}
		method := class.methods[key]
		for _, otherKey := range keys {
			if otherKey != key && class.methods[otherKey].goName == method.goName {
				c.report(decl.NameSpan, fmt.Sprintf("generated property method %q conflicts with another member", method.goName))
			}
		}
		for _, field := range class.fields {
			if field.goName == method.goName {
				c.report(decl.NameSpan, fmt.Sprintf("generated property method %q conflicts with a field", method.goName))
			}
		}
	}
}

func (c *Checker) checkClassProperty(expr *ast.MemberExpr, class *classSymbol, object Type, write bool) Type {
	return c.checkPropertyAccess(expr, class.methods, nativeClassBindings(class, object), write)
}

func (c *Checker) checkPropertyAccess(expr *ast.MemberExpr, methods map[string]methodSymbol, bindings nativeTypeBindings, write bool) Type {
	expr.Property = true
	expr.PropertyGetter, expr.PropertySetter = "", ""
	expr.PropertyGetterOwner, expr.PropertySetterOwner = "", ""
	getter, get := methods["get "+expr.Name]
	setter, set := methods["set "+expr.Name]
	if get && c.canAccessClassMember(getter.visibility, getter.declaringClass) {
		expr.PropertyGetter = getter.goName
		expr.PropertyGetterOwner = getter.virtualOwner
	}
	expr.PropertyGetterAbstract = get && getter.abstract
	if set && c.canAccessClassMember(setter.visibility, setter.declaringClass) {
		expr.PropertySetter = setter.goName
		expr.PropertySetterOwner = setter.virtualOwner
	}
	selected, exists, kind := getter, get, "getter"
	if write {
		selected, exists, kind = setter, set, "setter"
	}
	if !exists {
		c.report(expr.Span, fmt.Sprintf("property %q has no %s", expr.Name, kind))
		return Type{Kind: Invalid}
	}
	if !c.canAccessClassMember(selected.visibility, selected.declaringClass) {
		c.reportInaccessibleClassMember(expr.Span, kind, expr.Name, selected.visibility)
	}
	c.checkAbstractPropertyAccess(expr, selected.abstract, kind)
	expr.ResolvedDeclaration = selected.declarationSpan
	c.recordMemberWrite(expr.Span)
	c.invalidateAllMemberFacts(expr.Span, "a property accessor with unknown mutation effects")
	result := Type{Kind: Invalid}
	if write && len(selected.typeInfo.Parameters) == 1 {
		result = selected.typeInfo.Parameters[0]
	}
	if !write && selected.typeInfo.Result != nil {
		result = *selected.typeInfo.Result
	}
	return substituteNativeTypeParameters(result, bindings)
}

func (c *Checker) checkAccessorSignature(accessor string, parameters []Type, result Type, variadic bool, span source.Span) {
	for _, parameter := range parameters {
		c.rejectResultValueType(parameter, span, "properties")
		c.rejectTaskAPIType(parameter, span, "properties")
	}
	c.rejectResultValueType(result, span, "properties")
	c.rejectTaskAPIType(result, span, "properties")
	if accessor == "get" {
		if len(parameters) != 0 || result.Kind == Void {
			c.report(span, "getter must have no parameters and return a value")
		}
	} else if len(parameters) != 1 || variadic || result.Kind != Void {
		c.report(span, "setter must have exactly one non-rest parameter and return void")
	}
}

func (c *Checker) checkAbstractPropertyAccess(expr *ast.MemberExpr, abstract bool, kind string) {
	if !abstract {
		return
	}
	if expr.Super {
		c.report(expr.Span, fmt.Sprintf("super cannot access abstract %s %q without an implementation", kind, expr.Name))
	} else if receiver, direct := expr.Object.(*ast.IdentifierExpr); direct && receiver.Name == "this" && c.inConstructor {
		c.report(expr.Span, fmt.Sprintf("cannot access an abstract %s on this during construction", kind))
	}
}
