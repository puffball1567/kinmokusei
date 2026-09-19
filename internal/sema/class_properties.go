package sema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Accessors share the method representation, but not the source method namespace.
// Spaces cannot occur in source identifiers, so these keys cannot be called or
// accidentally satisfy source method contracts.
func hasClassProperty(class *classSymbol, name string) bool {
	_, get := class.methods["get "+name]
	_, set := class.methods["set "+name]
	return get || set
}

func (c *Checker) declareClassAccessor(decl *ast.ClassDecl, class *classSymbol, method *ast.MethodDecl) {
	if method.Static || method.Virtual || method.Override || method.Final || method.Abstract {
		c.report(method.Span, "properties currently require concrete nonvirtual instance accessors")
	}
	c.validateLabels(method.Body)
	key := method.Accessor + " " + method.Name
	if _, exists := class.methods[key]; exists {
		c.report(method.NameSpan, fmt.Sprintf("duplicate or inherited %s property %q cannot be redeclared", method.Accessor, method.Name))
	}
	for _, accessor := range []string{"get ", "set "} {
		if inherited, exists := class.methods[accessor+method.Name]; exists && inherited.declaringClass != decl.Name {
			c.report(method.NameSpan, fmt.Sprintf("inherited property %q cannot be redeclared", method.Name))
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
		c.rejectResultValueType(parameters[i], parameter.Span, "properties")
		c.rejectTaskAPIType(parameters[i], parameter.Span, "properties")
	}
	result := c.resolveType(method.ReturnType)
	c.rejectResultValueType(result, method.ReturnType.Span, "properties")
	c.rejectTaskAPIType(result, method.ReturnType.Span, "properties")
	if method.Accessor == "get" {
		if len(parameters) != 0 || result.Kind == Void {
			c.report(method.Span, "getter must have no parameters and return a value")
		}
	} else if len(parameters) != 1 || hasVariadicParameter(method.Parameters) || result.Kind != Void {
		c.report(method.Span, "setter must have exactly one non-rest parameter and return void")
	}
	method.GoName = memberGoName(method.Accessor+memberGoName(method.Name, ast.Public), method.Visibility)
	class.methods[key] = methodSymbol{typeInfo: Type{Kind: Function, Name: "function", Parameters: parameters, Result: &result},
		visibility: method.Visibility, goName: method.GoName, declarationSpan: method.NameSpan, declaringClass: decl.Name}
}

func (c *Checker) checkClassPropertyContracts(decl *ast.ClassDecl, class *classSymbol) {
	for _, method := range decl.Methods {
		if method.Accessor == "" {
			continue
		}
		getter, get := class.methods["get "+method.Name]
		setter, set := class.methods["set "+method.Name]
		if method.Accessor == "get" && get && set && len(setter.typeInfo.Parameters) == 1 && getter.typeInfo.Result != nil {
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
	expr.Property = true
	expr.PropertyGetter, expr.PropertySetter = "", ""
	getter, get := class.methods["get "+expr.Name]
	setter, set := class.methods["set "+expr.Name]
	if get && c.canAccessClassMember(getter.visibility, getter.declaringClass) {
		expr.PropertyGetter = getter.goName
	}
	if set && c.canAccessClassMember(setter.visibility, setter.declaringClass) {
		expr.PropertySetter = setter.goName
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
	return substituteNativeTypeParameters(result, nativeClassBindings(class, object))
}
