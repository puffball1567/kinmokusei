package sema

import (
	"fmt"
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
	member := embeddedStructMember{structEmbedding: structEmbedding{receiver: receiver}}
	if field, ok := structure.fields[expr.Name]; ok {
		member.field = &field
	} else if method, ok := structure.methods[expr.Name]; ok {
		member.method = &method
	} else if len(structure.embedded) != 0 {
		member = c.structMemberSet(receiver, false)[expr.Name]
	}
	if member.field == nil && member.method == nil {
		c.report(expr.Span, fmt.Sprintf("struct %s has no field %q or method with that name", base.Name, expr.Name))
		return invalid
	}
	if member.ambiguous || len(member.path) != 0 && !c.validPromotedStructMember(expr, receiver, member) {
		if member.ambiguous {
			c.report(expr.NameSpan, fmt.Sprintf("ambiguous promoted struct member %q", expr.Name))
		}
		return invalid
	}
	expr.EmbeddedPath = member.path
	owner := member.owner()
	bindings := nativeStructBindings(c.structs[owner.Name], owner)
	if member.field != nil {
		field := member.field
		expr.ResolvedName, expr.ResolvedDeclaration = field.goName, field.declarationSpan
		expr.Addressable = pointer || member.throughPointer || c.isAddressableExpression(expr.Object)
		expr.GoFieldViaPointer = member.throughPointer
		c.recordLayoutFieldReceiver(expr, receiver)
		return substituteNativeTypeParameters(field.typeInfo, bindings)
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
	callable := c.substituteNativeMethodOwnerTypeParameters(method.typeInfo, bindings)
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
	field  *fieldSymbol
	method *methodSymbol
}

// A selector sees the shallowest declaration. Reaching the same struct by
// different paths at that depth is still ambiguous. Coalescing those paths
// bounds the search through diamonds and terminates on recursive pointers.
func (c *Checker) structMemberSet(receiver Type, goNames bool) map[string]embeddedStructMember {
	root := structEmbedding{receiver: receiver}
	frontier := map[string]structEmbedding{root.owner().Name: root}
	seen := map[string]bool{}
	result := map[string]embeddedStructMember{}
	for len(frontier) != 0 {
		members := map[string]embeddedStructMember{}
		next := map[string]structEmbedding{}
		for name := range frontier {
			seen[name] = true
		}
		for _, branch := range frontier {
			owner := branch.owner()
			structure := c.structs[owner.Name]
			if structure == nil {
				continue
			}
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
			for name, field := range structure.fields {
				add(name, field.goName, embeddedStructMember{structEmbedding: branch, field: &field})
			}
			for name, method := range structure.methods {
				if !goNames || !method.typeInfo.Generic {
					add(name, method.goName, embeddedStructMember{structEmbedding: branch, method: &method})
				}
			}
			bindings := nativeStructBindings(structure, owner)
			for _, fieldName := range structure.embedded {
				field := structure.fields[fieldName]
				value := substituteNativeTypeParameters(field.typeInfo, bindings)
				child := structEmbedding{receiver: value, throughPointer: branch.throughPointer || value.Kind == GoPointer, ambiguous: branch.ambiguous}
				name := child.owner().Name
				if seen[name] {
					continue
				}
				if _, duplicate := next[name]; duplicate {
					child.ambiguous = true
				}
				child.path = append(slices.Clone(branch.path), field.goName)
				next[name] = child
			}
		}
		for name, member := range members {
			result[name] = member
		}
		frontier = next
	}
	return result
}

func sameStructMemberPath(member, emitted embeddedStructMember) bool {
	return !emitted.ambiguous && slices.Equal(member.path, emitted.path) &&
		(member.method != nil && emitted.method != nil || member.field != nil && emitted.field != nil)
}

func (c *Checker) effectiveStructMethods(base Type, pointer bool) (map[string]methodSymbol, nativeTypeBindings) {
	structure := c.structs[base.Name]
	if len(structure.embedded) == 0 {
		return structure.methods, nativeStructBindings(structure, base)
	}
	receiver := base
	if pointer {
		receiver = Type{Kind: GoPointer, Element: &base}
	}
	emitted := c.structMemberSet(receiver, true)
	methods := map[string]methodSymbol{}
	for name, member := range c.structMemberSet(receiver, false) {
		if member.method == nil || member.method.typeInfo.Generic || member.ambiguous || member.method.pointerReceiver && !pointer && !member.throughPointer {
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
		owner := member.owner()
		method.typeInfo = substituteNativeTypeParameters(method.typeInfo, nativeStructBindings(c.structs[owner.Name], owner))
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
			}
			if !generic && !sameStructMemberPath(member, emitted[generatedIdentifier(goName)]) {
				continue
			}
			owner := member.owner()
			info.Type = typeRefFromType(substituteNativeTypeParameters(value, nativeStructBindings(c.structs[owner.Name], owner)), info.Declaration)
			decl.PromotedMembers = append(decl.PromotedMembers, info)
		}
		sort.Slice(decl.PromotedMembers, func(i, j int) bool { return decl.PromotedMembers[i].Name < decl.PromotedMembers[j].Name })
	}
}
