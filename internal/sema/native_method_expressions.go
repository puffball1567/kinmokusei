package sema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Recognize type receivers only in the source type namespace. A lexically
// shadowing value must retain ordinary selector/dereference semantics.
func (c *Checker) nativeMethodExpressionReceiver(expression ast.Expression) (Type, ast.TypeRef, bool) {
	var ref ast.TypeRef
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		// Enum names retain their existing member namespace and completion.
		if c.enums[expression.Name] != nil {
			return Type{}, ref, false
		}
		if _, exists := c.lookupTypeParameter(expression.Name); exists {
			return Type{}, ref, false
		}
		for _, scope := range c.scopes {
			if _, exists := scope[expression.Name]; exists {
				return Type{}, ref, false
			}
		}
		if !c.isTopLevelAllowed(expression.Span, expression.Name) {
			return Type{}, ref, false
		}
		switch {
		case c.structs[expression.Name] != nil:
			expression.ResolvedDeclaration = c.structs[expression.Name].declarationSpan
		case c.interfaces[expression.Name] != nil:
			expression.ResolvedDeclaration = c.interfaces[expression.Name].declarationSpan
		case c.nativeTypes[expression.Name] != nil:
			expression.ResolvedDeclaration = c.nativeTypes[expression.Name].declaration.NameSpan
		default:
			return Type{}, ref, false
		}
		ref = ast.TypeRef{Name: expression.Name, NameSpan: expression.Span, Span: expression.Span}
	case *ast.UnaryExpr:
		if expression.Operator != "*" {
			return Type{}, ref, false
		}
		_, pointee, found := c.nativeMethodExpressionReceiver(expression.Operand)
		if !found {
			return Type{}, ref, false
		}
		ref = ast.TypeRef{Pointee: &pointee, Span: expression.Span}
	default:
		return Type{}, ref, false
	}
	receiver := c.resolveType(ref)
	c.recordNativeMethodExpressionType(expression.GetSpan(), receiver)
	return receiver, ref, true
}

// Source metadata, rather than a round trip through Go storage, preserves
// verified text, nullable members, native identity and Result signatures.
func (c *Checker) nativeMethodExpressionSet(receiver Type) map[string]methodSymbol {
	base := receiver
	pointer := base.Kind == GoPointer && base.Element != nil
	if pointer {
		base = *base.Element
	}
	var methods map[string]methodSymbol
	var bindings nativeTypeBindings
	switch base.Kind {
	case Struct:
		if symbol := c.structs[base.Name]; symbol != nil {
			methods, bindings = symbol.methods, nativeStructBindings(symbol, base)
		}
	case Interface:
		// A pointer to an interface has no interface method set.
		if symbol := c.interfaces[base.Name]; symbol != nil && !pointer && !symbol.constraint {
			methods, bindings = symbol.methods, nativeInterfaceBindings(symbol, base)
		}
	case GoNamed:
		if object := goTypeNameObject(base.GoType); object != nil && base.GoQualifier == "" {
			if symbol := c.nativeTypes[object.Name()]; symbol != nil && !symbol.declaration.Alias {
				methods, bindings = symbol.methods, nativeDefinedTypeBindings(symbol, base)
			}
		}
	}
	result := map[string]methodSymbol{}
	for name, method := range methods {
		if strings.Contains(name, " ") || method.static || method.pointerReceiver && !pointer || method.typeInfo.Generic {
			continue
		}
		method.typeInfo = substituteNativeTypeParameters(method.typeInfo, bindings)
		method.typeInfo.Parameters = append([]Type{receiver}, method.typeInfo.Parameters...)
		// The stored signature (if any) belongs to the bound method, not this
		// explicit-receiver function. Reconstruct storage lazily when needed.
		method.typeInfo.GoType = nil
		result[name] = method
	}
	return result
}

func (c *Checker) nativeMethodExpressionVisible(method methodSymbol, span source.Span) bool {
	return method.visibility == ast.Public || method.declarationSpan.Path == span.Path
}

func (c *Checker) checkNativeMethodExpression(expr *ast.MemberExpr, receiver Type, ref ast.TypeRef) Type {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	if receiver.Kind == Invalid {
		return invalid
	}
	method, exists := c.nativeMethodExpressionSet(receiver)[expr.Name]
	if !exists {
		c.report(expr.NameSpan, fmt.Sprintf("native type %s has no non-generic method %q in its method set", receiver.String(), expr.Name))
		return invalid
	}
	if !c.nativeMethodExpressionVisible(method, expr.Span) {
		c.report(expr.NameSpan, fmt.Sprintf("method %q is private on native type %s", expr.Name, receiver.String()))
		return invalid
	}
	expr.MethodExpressionReceiver = &ref
	expr.ResolvedName, expr.ResolvedDeclaration = method.goName, method.declarationSpan
	// Go's initialization dependency graph follows concrete method bodies,
	// even for stored method expressions; interface dispatch remains indirect.
	if receiver.Kind != Interface {
		c.recordGlobalDependency(nativeMethodDependency(method.declarationSpan))
	}
	return method.typeInfo
}

func (c *Checker) recordNativeMethodExpressionType(span source.Span, receiver Type) {
	if c.nativeTypeMethodSets == nil {
		c.nativeTypeMethodSets = map[source.Span][]ast.ObjectTypeField{}
	}
	members := []ast.ObjectTypeField{}
	for name, method := range c.nativeMethodExpressionSet(receiver) {
		if c.nativeMethodExpressionVisible(method, span) {
			members = append(members, ast.ObjectTypeField{Name: name, Span: method.declarationSpan, Type: typeRefFromType(method.typeInfo, span)})
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	c.nativeTypeMethodSets[span] = members
}
