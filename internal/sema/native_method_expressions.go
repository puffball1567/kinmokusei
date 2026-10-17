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
	ref, found := c.nativeMethodExpressionReceiverRef(expression)
	if !found {
		return Type{}, ref, false
	}
	receiver := c.resolveType(ref)
	c.recordNativeMethodExpressionType(expression.GetSpan(), receiver, ref)
	return receiver, ref, true
}

// Build the complete application before resolving it: resolving Box on the
// way to Box<int> would incorrectly diagnose missing type arguments.
func (c *Checker) nativeMethodExpressionReceiverRef(expression ast.Expression) (ast.TypeRef, bool) {
	var ref ast.TypeRef
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		if _, exists := c.lookupTypeParameter(expression.Name); exists {
			return ref, false
		}
		for _, scope := range c.scopes {
			if _, exists := scope[expression.Name]; exists {
				return ref, false
			}
		}
		if !c.isTopLevelAllowed(expression.Span, expression.Name) {
			return ref, false
		}
		switch {
		case c.structs[expression.Name] != nil:
			expression.ResolvedDeclaration = c.structs[expression.Name].declarationSpan
		case c.interfaces[expression.Name] != nil:
			expression.ResolvedDeclaration = c.interfaces[expression.Name].declarationSpan
		case c.nativeTypes[expression.Name] != nil:
			expression.ResolvedDeclaration = c.nativeTypes[expression.Name].declaration.NameSpan
		default:
			return ref, false
		}
		ref = ast.TypeRef{Name: expression.Name, NameSpan: expression.Span, Span: expression.Span}
	case *ast.IndexExpr:
		identifier, ok := expression.Object.(*ast.IdentifierExpr)
		if !ok || len(expression.TypeArguments) == 0 {
			return ref, false
		}
		base, found := c.nativeMethodExpressionReceiverRef(identifier)
		if !found {
			return ref, false
		}
		ref = base
		ref.GenericArguments, ref.Span = expression.TypeArguments, expression.Span
		expression.Instantiated, expression.ResolvedType = true, ref
	case *ast.UnaryExpr:
		if expression.Operator != "*" {
			return ref, false
		}
		pointee, found := c.nativeMethodExpressionReceiverRef(expression.Operand)
		if !found {
			return ref, false
		}
		ref = ast.TypeRef{Pointee: &pointee, Span: expression.Span}
	default:
		return ref, false
	}
	return ref, true
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

func (c *Checker) recordNativeMethodExpressionType(span source.Span, receiver Type, ref ast.TypeRef) {
	if c.nativeTypeMethodSets == nil {
		c.nativeTypeMethodSets = map[source.Span]ast.NativeTypeMethodSet{}
	}
	members := []ast.ObjectTypeField{}
	for name, method := range c.nativeMethodExpressionSet(receiver) {
		if c.nativeMethodExpressionVisible(method, span) {
			members = append(members, ast.ObjectTypeField{Name: name, Span: method.declarationSpan, Type: typeRefFromType(method.typeInfo, span)})
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	set := ast.NativeTypeMethodSet{Methods: members}
	// Only the enum declaration's namespace exposes its constants; aliases
	// share the method set, but do not create new enum constant namespaces.
	if enumeration := c.enums[ref.Name]; enumeration != nil && !ref.IsPointer() {
		visibleMethods := set.Methods[:0]
		for _, method := range set.Methods {
			if enumeration.members[method.Name] == nil {
				visibleMethods = append(visibleMethods, method)
			}
		}
		set.Methods = visibleMethods
		for _, member := range enumeration.declaration.Members {
			set.Constants = append(set.Constants, ast.ObjectTypeField{Name: member.Name, Span: member.NameSpan, Type: ast.TypeRef{Name: ref.Name}})
		}
	}
	c.nativeTypeMethodSets[span] = set
}
