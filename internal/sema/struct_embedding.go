package sema

import (
	"fmt"
	gotypes "go/types"
	"slices"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkNativeStructMember(expr *ast.MemberExpr, receiver, base Type, pointer bool) Type {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	structure := c.structs[base.Name]
	if structure == nil {
		return invalid
	}
	member := embeddedStructMember{structEmbedding: structEmbedding{receiver: receiver}, bindings: nativeStructBindings(structure, base)}
	if field, ok := structure.fields[expr.Name]; ok {
		member.field = &field
	} else if method, ok := structure.methods[expr.Name]; ok {
		member.method = &method
	} else if len(structure.embedded) != 0 {
		member = c.structMemberSet(receiver, false)[expr.Name]
	}
	if !member.found() {
		c.report(expr.Span, fmt.Sprintf("struct %s has no field %q or method with that name", base.Name, expr.Name))
		return invalid
	}
	return c.checkSelectedStructMember(expr, receiver, base, pointer, member)
}

func (c *Checker) checkSelectedStructMember(expr *ast.MemberExpr, receiver, base Type, pointer bool, member embeddedStructMember) Type {
	invalid := Type{Kind: Invalid, Name: "<invalid>"}
	if member.ambiguous || len(member.path) != 0 && !c.validPromotedStructMember(expr, receiver, member) {
		if member.ambiguous {
			c.report(expr.NameSpan, fmt.Sprintf("ambiguous promoted struct member %q", expr.Name))
		}
		return invalid
	}
	expr.EmbeddedPath = member.path
	if member.goMember != nil {
		return c.checkPromotedGoStructMember(expr, receiver, base, pointer, member)
	}
	owner := member.owner()
	if member.field != nil {
		field := member.field
		expr.ResolvedName, expr.ResolvedDeclaration = field.goName, field.declarationSpan
		expr.Addressable = pointer || member.throughPointer || c.isAddressableExpression(expr.Object)
		expr.GoFieldViaPointer = member.throughPointer
		c.recordLayoutFieldReceiver(expr, receiver)
		return substituteNativeTypeParameters(field.typeInfo, member.bindings)
	}
	method := *member.method
	c.recordGlobalDependency(nativeMethodDependency(method.declarationSpan))
	if method.pointerReceiver && !pointer && !member.throughPointer && !c.isAddressableExpression(expr.Object) {
		c.report(expr.Span, fmt.Sprintf("pointer method %q requires an addressable %s value", expr.Name, base.String()))
		return invalid
	}
	expr.ResolvedName, expr.ResolvedDeclaration, expr.BoundMethod = method.goName, method.declarationSpan, true
	if len(method.typeInfo.TypeParameters) != 0 {
		expr.GenericMethod = true
		expr.ResolvedName = staticMethodGoName(owner.Name, method.goName, method.visibility)
		expr.GenericReceiverAddress = method.pointerReceiver && member.receiver.Kind != GoPointer
		if c.directCallCallee != expr {
			c.report(expr.Span, "generic methods must be called directly; Go cannot represent an uninstantiated generic method value")
		}
	}
	callable := c.substituteNativeMethodOwnerTypeParameters(method.typeInfo, member.bindings)
	recordReceiverContracts(expr, owner, member.receiver, callable, method)
	return callable
}

func (c *Checker) validPromotedStructMember(expr *ast.MemberExpr, receiver Type, member embeddedStructMember) bool {
	goName, visible, generic := "", false, false
	if field := member.field; field != nil {
		goName, visible = field.goName, field.visibility == ast.Public || field.declarationSpan.Path == expr.Span.Path
	} else if method := member.method; method != nil {
		goName, visible = method.goName, method.visibility == ast.Public || method.declarationSpan.Path == expr.Span.Path
		generic = method.typeInfo.Generic
	} else if object := member.goMember; object != nil {
		goName, visible = object.Name(), object.Exported()
	}
	if !visible {
		c.report(expr.NameSpan, fmt.Sprintf("promoted struct member %q is private", expr.Name))
		return false
	}
	if !generic && !sameStructMemberPath(member, c.structMemberSet(receiver, true)[generatedIdentifier(goName)]) {
		c.report(expr.NameSpan, fmt.Sprintf("promoted struct member %q is shadowed or ambiguous in Go; select the embedded field explicitly", expr.Name))
		return false
	}
	return true
}

func (c *Checker) embeddedStructGoName(ref ast.TypeRef) string {
	// An embedded field uses the name of the type that Go actually sees.
	// Generic source aliases are expanded by the Go 1.23 emitter.
	seen := map[string]bool{}
	for {
		if ref.Pointee != nil {
			ref = *ref.Pointee
		}
		if ref.Name == "" || seen[ref.Name] {
			return ""
		}
		alias := c.nativeTypes[ref.Name]
		if alias == nil || !alias.declaration.Alias || len(ref.GenericArguments) == 0 {
			if ref.Qualifier == "" {
				if imported, ok := c.goNamedImports[ref.Span.Path][ref.Name]; ok {
					return imported.name
				}
			}
			return ref.Name
		}
		seen[ref.Name] = true
		ref = instantiateGenericAliasTypeRef(ref, alias.declaration)
	}
}

type structEmbedding struct {
	receiver       Type
	path           []string
	throughPointer bool // An embedded pointer, excluding the outer receiver.
	ambiguous      bool
}

func (e structEmbedding) owner() Type {
	if e.receiver.Kind == GoPointer && e.receiver.Element != nil {
		return *e.receiver.Element
	}
	return e.receiver
}

type embeddedStructMember struct {
	structEmbedding
	bindings nativeTypeBindings
	field    *fieldSymbol
	method   *methodSymbol
	goMember gotypes.Object
}

func (m embeddedStructMember) found() bool {
	return m.field != nil || m.method != nil || m.goMember != nil
}

// A selector sees the shallowest declaration. Reaching the same struct by
// different paths at that depth is still ambiguous. Coalescing those paths
// bounds the search through diamonds and terminates on recursive pointers.
func (c *Checker) structMemberSet(receiver Type, goNames bool) map[string]embeddedStructMember {
	root := structEmbedding{receiver: receiver}
	frontier := map[structEmbeddingKey]structEmbedding{root.key(): root}
	seen := map[structEmbeddingKey]bool{}
	result := map[string]embeddedStructMember{}
	for len(frontier) != 0 {
		members := map[string]embeddedStructMember{}
		next := map[structEmbeddingKey]structEmbedding{}
		for name := range frontier {
			seen[name] = true
		}
		for _, branch := range frontier {
			owner := branch.owner()
			add := func(name, goName string, member embeddedStructMember) {
				if goNames {
					name = generatedIdentifier(goName)
				}
				if _, shallower := result[name]; shallower {
					return
				}
				if _, duplicate := members[name]; duplicate {
					member.ambiguous = true
				}
				members[name] = member
			}
			addChild := func(value Type, goName string) {
				child := structEmbedding{
					receiver: value, throughPointer: branch.throughPointer || value.Kind == GoPointer, ambiguous: branch.ambiguous,
					path: append(slices.Clone(branch.path), goName),
				}
				key := child.key()
				if seen[key] {
					return
				}
				if _, duplicate := next[key]; duplicate {
					child.ambiguous = true
				}
				next[key] = child
			}
			c.visitStructEmbedding(branch, owner, goNames, add, addChild)
		}
		for name, member := range members {
			result[name] = member
		}
		frontier = next
	}
	return result
}

func (c *Checker) visitStructEmbedding(branch structEmbedding, owner Type, goNames bool, add func(string, string, embeddedStructMember), embed func(Type, string)) {
	shape := owner
	var methods map[string]methodSymbol
	var methodBindings nativeTypeBindings
	if owner.Kind == Struct {
		structure := c.structs[owner.Name]
		if structure == nil {
			return
		}
		methods, methodBindings = structure.methods, nativeStructBindings(structure, owner)
	} else if defined := c.nativeDefinedSymbol(owner); defined != nil {
		methods, methodBindings = defined.methods, nativeDefinedTypeBindings(defined, owner)
		// A defined type retains its underlying fields, including embedded
		// fields. Only its own declarations supply direct methods.
		shape = c.nativeDefinedUnderlying(defined, owner)
		if shape.Kind == Object {
			for name, value := range shape.Fields {
				field := fieldSymbol{typeInfo: value, visibility: ast.Public, goName: shape.FieldNames[name]}
				add(name, field.goName, embeddedStructMember{structEmbedding: branch, field: &field})
			}
		} else if shape.Kind != Struct && owner.GoType != nil {
			if layout, ok := owner.GoType.Underlying().(*gotypes.Struct); ok {
				branch.visitGoStructFields(layout, add, embed)
			}
		}
	} else {
		branch.visitGoMembers(add, embed)
		return
	}
	for name, method := range methods {
		if !goNames || !method.typeInfo.Generic {
			add(name, method.goName, embeddedStructMember{structEmbedding: branch, bindings: methodBindings, method: &method})
		}
	}
	if shape.Kind != Struct {
		return
	}
	if structure := c.structs[shape.Name]; structure != nil {
		bindings := nativeStructBindings(structure, shape)
		for name, field := range structure.fields {
			add(name, field.goName, embeddedStructMember{structEmbedding: branch, bindings: bindings, field: &field})
		}
		for _, name := range structure.embedded {
			field := structure.fields[name]
			embed(substituteNativeTypeParameters(field.typeInfo, bindings), field.goName)
		}
	}
}

func sameStructMemberPath(member, emitted embeddedStructMember) bool {
	if emitted.ambiguous || !slices.Equal(member.path, emitted.path) {
		return false
	}
	if member.goMember != nil && emitted.goMember != nil {
		return member.goMember.Name() == emitted.goMember.Name()
	}
	return member.method != nil && emitted.method != nil || member.field != nil && emitted.field != nil
}

func (c *Checker) effectiveStructMethods(base Type, pointer bool) (map[string]methodSymbol, nativeTypeBindings) {
	if structure := c.structs[base.Name]; base.Kind == Struct && structure != nil && len(structure.embedded) == 0 {
		return structure.methods, nativeStructBindings(structure, base)
	}
	receiver := base
	if pointer {
		receiver = Type{Kind: GoPointer, Element: &base}
	}
	emitted := c.structMemberSet(receiver, true)
	methods := map[string]methodSymbol{}
	for name, member := range c.structMemberSet(receiver, false) {
		if member.ambiguous {
			continue
		}
		if member.goMember != nil {
			if method, ok := c.promotedGoMethod(member); ok && (pointer || !method.pointerReceiver) &&
				sameStructMemberPath(member, emitted[method.goName]) {
				methods[name] = method
			}
			continue
		}
		if member.method == nil || member.method.typeInfo.Generic || member.method.pointerReceiver && !pointer && !member.throughPointer {
			continue
		}
		method := *member.method
		if len(member.path) != 0 {
			if !sameStructMemberPath(member, emitted[generatedIdentifier(method.goName)]) {
				continue
			}
			method.embeddedMember = &member
			method.pointerReceiver = method.pointerReceiver && !member.throughPointer
		}
		method.typeInfo = substituteNativeTypeParameters(method.typeInfo, member.bindings)
		methods[name] = method
	}
	return methods, nil
}

func (c *Checker) recordPromotedStructMembers(program *ast.Program) {
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.StructDecl)
		if !ok {
			continue
		}
		decl.PromotedMembers = nil
		symbol := c.structs[decl.Name]
		if symbol == nil || len(symbol.embedded) == 0 {
			continue
		}
		receiver := symbol.typeInfo
		receiver.TypeArguments = symbol.typeParameters
		emitted := c.structMemberSet(receiver, true)
		for name, member := range c.structMemberSet(receiver, false) {
			if member.ambiguous || len(member.path) == 0 {
				continue
			}
			var goName string
			var value Type
			var generic bool
			info := ast.PromotedStructMember{Name: name, Method: member.method != nil}
			if field := member.field; field != nil {
				goName, value = field.goName, field.typeInfo
				info.Visibility, info.Declaration = field.visibility, field.declarationSpan
			} else if method := member.method; method != nil {
				goName, value, generic = method.goName, method.typeInfo, method.typeInfo.Generic
				info.Visibility, info.Declaration = method.visibility, method.declarationSpan
			} else if object := member.goMember; object != nil {
				var err error
				value, err = goValueMemberType(member.owner(), object)
				if err != nil || !c.allowUnsafeGo && goTypeContainsUnsafePointer(value.GoType, nil) {
					continue
				}
				_, info.Method = object.(*gotypes.Func)
				goName, info.Visibility = object.Name(), ast.Public
			}
			if !generic && !sameStructMemberPath(member, emitted[generatedIdentifier(goName)]) {
				continue
			}
			value = substituteNativeTypeParameters(value, member.bindings)
			info.Type = typeRefFromType(value, info.Declaration)
			decl.PromotedMembers = append(decl.PromotedMembers, info)
		}
		sort.Slice(decl.PromotedMembers, func(i, j int) bool { return decl.PromotedMembers[i].Name < decl.PromotedMembers[j].Name })
	}
}
