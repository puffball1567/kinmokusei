package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkStructEmbeddingType(value Type, span source.Span) bool {
	pointer := value.Kind == GoPointer && value.Element != nil
	if pointer {
		value = *value.Element
	}
	if value.Kind == Struct {
		return true
	}
	if defined := c.nativeDefinedSymbol(value); defined != nil {
		// The origin may still be waiting for a source struct's layout.
		// Looking at the origin avoids expanding an unfinished instance.
		switch defined.goNamed.Underlying().(type) {
		case *gotypes.Pointer, *gotypes.Interface:
			c.report(span, "cannot embed a distinct type with pointer or interface storage")
			return false
		}
		return true
	}
	if name := goTypeNameObject(value.GoType); name != nil &&
		(name.Pkg() != nil || name == gotypes.Universe.Lookup("error")) {
		if arity := goTypeParameterCount(value.GoType); arity > len(value.TypeArguments) {
			c.report(span, fmt.Sprintf("embedded Go type %s requires %d type arguments", value.String(), arity))
			return false
		}
		storage := gotypes.Unalias(value.GoType)
		if aliasPointer, ok := storage.(*gotypes.Pointer); ok && !pointer {
			storage = aliasPointer.Elem()
			pointer = true
		}
		switch shape := storage.Underlying().(type) {
		case *gotypes.Pointer:
			c.report(span, "cannot embed a Go defined pointer type or a pointer to a pointer")
			return false
		case *gotypes.Basic:
			if shape.Kind() == gotypes.UnsafePointer {
				c.report(span, "cannot embed a Go type with unsafe.Pointer storage")
				return false
			}
		case *gotypes.Interface:
			if !shape.Complete().IsMethodSet() {
				c.report(span, "embedded Go interface must be usable as a runtime value")
				return false
			}
			if pointer {
				c.report(span, "cannot embed a pointer to a Go interface")
				return false
			}
		}
		return true
	}
	c.report(span, "embedded field must name a Kinmokusei struct or distinct type, a named Go type, or a supported pointer to one")
	return false
}

type structEmbeddingKey struct {
	source string
	goType gotypes.Type
}

func (e structEmbedding) key() structEmbeddingKey {
	owner := e.owner()
	if owner.Kind == Struct {
		return structEmbeddingKey{source: owner.Name}
	}
	identity := gotypes.Unalias(owner.GoType)
	if named, ok := identity.(*gotypes.Named); ok {
		identity = named.Origin()
	}
	return structEmbeddingKey{goType: identity}
}

// Include Go declarations at their actual embedding depth so collisions
// between source and Go members follow the same selector and method-set rules.
func (e structEmbedding) visitGoMembers(add func(string, string, embeddedStructMember), embed func(Type, string)) {
	owner := e.owner()
	if owner.GoType == nil {
		return
	}
	storage := gotypes.Unalias(owner.GoType)
	addExported := func(object gotypes.Object) {
		if object.Exported() {
			add(object.Name(), object.Name(), embeddedStructMember{structEmbedding: e, goMember: object})
		}
	}
	switch shape := storage.Underlying().(type) {
	case *gotypes.Interface:
		shape.Complete()
		for i := 0; i < shape.NumMethods(); i++ {
			addExported(shape.Method(i))
		}
		return
	case *gotypes.Struct:
		e.visitGoStructFields(shape, add, embed)
	}
	if named, ok := storage.(*gotypes.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			addExported(named.Method(i))
		}
	}
}

func (e structEmbedding) visitGoStructFields(shape *gotypes.Struct, add func(string, string, embeddedStructMember), embed func(Type, string)) {
	for i := 0; i < shape.NumFields(); i++ {
		field := shape.Field(i)
		if field.Exported() {
			add(field.Name(), field.Name(), embeddedStructMember{structEmbedding: e, goMember: field})
		}
		if field.Embedded() {
			embed(goStructEmbeddingReceiver(field.Type(), e.owner()), field.Name())
		}
	}
}

func goStructEmbeddingReceiver(storage gotypes.Type, parent Type) Type {
	pointer, indirect := gotypes.Unalias(storage).(*gotypes.Pointer)
	base := storage
	if indirect {
		base = pointer.Elem()
	}
	value := Type{Kind: GoNamed, Name: goTypeDisplayName(base), GoType: base}
	inheritGoQualifier(&value, parent)
	if indirect {
		return Type{Kind: GoPointer, Name: "*" + value.Name, GoType: storage, Element: &value, GoQualifier: value.GoQualifier}
	}
	return value
}

func goMethodHasPointerReceiver(method *gotypes.Func) bool {
	receiver := method.Type().(*gotypes.Signature).Recv()
	return receiver != nil && goTypeIsPointer(receiver.Type())
}

func (c *Checker) checkPromotedGoStructMember(expr *ast.MemberExpr, receiver, base Type, pointer bool, member embeddedStructMember) Type {
	addressable := pointer || member.throughPointer || c.isAddressableExpression(expr.Object)
	if method, ok := member.goMember.(*gotypes.Func); ok && goMethodHasPointerReceiver(method) && !addressable {
		c.report(expr.Span, fmt.Sprintf("pointer method %q requires an addressable %s value", expr.Name, base.String()))
		return Type{Kind: Invalid}
	}
	result := c.checkSelectedGoValueMember(expr, member.owner(), member.goMember)
	if _, field := member.goMember.(*gotypes.Var); field {
		expr.Addressable = addressable
		expr.GoField = true
		expr.GoFieldViaPointer = member.throughPointer
		c.recordLayoutFieldReceiver(expr, receiver)
	}
	return result
}

func (c *Checker) promotedGoMethod(member embeddedStructMember) (methodSymbol, bool) {
	method, ok := member.goMember.(*gotypes.Func)
	if !ok {
		return methodSymbol{}, false
	}
	callable, err := goValueMemberType(member.owner(), method)
	if err != nil || !c.allowUnsafeGo && goTypeContainsUnsafePointer(callable.GoType, nil) {
		return methodSymbol{}, false
	}
	return methodSymbol{
		typeInfo: callable, visibility: ast.Public, goName: method.Name(), goDeclaration: method,
		pointerReceiver: goMethodHasPointerReceiver(method) && !member.throughPointer,
	}, true
}
