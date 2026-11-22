package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func isImportedGoStruct(value Type) bool {
	if value.Kind != GoNamed {
		return false
	}
	name := goTypeNameObject(value.GoType)
	if name == nil || name.Pkg() == nil {
		return false
	}
	_, structure := gotypes.Unalias(value.GoType).Underlying().(*gotypes.Struct)
	return structure
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
		for i := 0; i < shape.NumFields(); i++ {
			field := shape.Field(i)
			addExported(field)
			if field.Embedded() {
				embed(goStructEmbeddingReceiver(field.Type(), owner), field.Name())
			}
		}
	}
	if named, ok := storage.(*gotypes.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			addExported(named.Method(i))
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
