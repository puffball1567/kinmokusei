package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func isSourceAnonymousInterface(value Type) bool {
	if value.Kind != GoInterface {
		return false
	}
	for _, method := range value.GoMethods {
		if method.GoName != "" {
			return true
		}
	}
	return false
}

// Preserve source method signatures when present; imported interfaces expose
// raw Go signatures. Match emitted names because source spelling is not ABI.
func anonymousInterfaceContractMethods(value Type) (map[string]Type, bool) {
	if value.Kind != GoInterface {
		storage := underlyingGoInterface(value.GoType)
		if storage == nil {
			return nil, false
		}
		var err error
		value, err = kinmokuseiTypeFromGo(storage)
		if err != nil {
			return nil, false
		}
	}
	methods := make(map[string]Type, len(value.GoMethods))
	for _, method := range value.GoMethods {
		name := method.Name
		if method.GoName != "" {
			name = method.GoName
		}
		methods[name] = method.Type
	}
	return methods, true
}

// Resolve source signatures independently of their Go storage. Validate names
// before constructing go/types interfaces, which panic on duplicate methods.
func (c *Checker) resolveAnonymousInterface(ref ast.TypeRef) Type {
	result := Type{Kind: GoInterface, Name: "interface{}"}
	names := map[string]GoInterfaceMethod{}
	valid := true
	add := func(method GoInterfaceMethod, span source.Span, embedded bool) {
		storageName := method.GoName
		if storageName == "" {
			storageName = method.Name
		}
		if previous, exists := names[storageName]; exists {
			if !embedded || previous.Name != method.Name || !sameComposedMethodContract(previous.Type, method.Type) || !c.identicalMethodStorage(previous.Type, method.Type) {
				c.report(span, fmt.Sprintf("anonymous interface has conflicting Go method name %q", storageName))
				valid = false
			}
			return
		}
		names[storageName] = method
		result.GoMethods = append(result.GoMethods, method)
	}
	for _, method := range ref.ObjectFields {
		if method.GoEmbedded {
			continue
		}
		name := method.Name
		goName := method.GoName
		if !ref.Go {
			goName = memberGoName(name, ast.Public)
		}
		callable := c.resolveType(method.Type)
		callable.ParameterNames = append([]string(nil), method.ParameterNames...)
		if callable.Kind != Function || callable.Result == nil {
			c.report(method.Span, fmt.Sprintf("anonymous interface method %q requires a callable signature", name))
			valid = false
		}
		add(GoInterfaceMethod{Name: name, GoName: goName, Type: callable, DeclarationSpan: method.NameSpan}, method.Span, false)
	}
	hasEmbeddings := false
	for _, field := range ref.ObjectFields {
		if !field.GoEmbedded {
			continue
		}
		hasEmbeddings = true
		methods, ok := c.anonymousInterfaceEmbedding(field.Type)
		valid = valid && ok
		for _, method := range methods {
			add(method, field.Span, true)
		}
	}
	if !valid {
		return Type{Kind: Invalid}
	}
	storage, ok := c.goTypeForNativeStorage(result)
	if !ok {
		c.report(ref.Span, "anonymous interface method signatures cannot be represented as Go types")
		return Type{Kind: Invalid}
	}
	result.GoType = storage
	result.Name = goTypeDisplayName(storage)
	if hasEmbeddings && ref.Span.Path != "" {
		if c.anonymousInterfaceMembers == nil {
			c.anonymousInterfaceMembers = map[source.Span][]ast.ObjectTypeField{}
		}
		if _, exists := c.anonymousInterfaceMembers[ref.Span]; !exists {
			c.anonymousInterfaceMembers[ref.Span] = typeRefFromType(result, ref.Span).ObjectFields
		}
	}
	return result
}

func (c *Checker) anonymousInterfaceEmbedding(ref ast.TypeRef) ([]GoInterfaceMethod, bool) {
	base := c.resolveType(ref)
	if base.Kind == Invalid {
		return nil, false
	}
	if isSourceAnonymousInterface(base) {
		return base.GoMethods, true
	}
	if base.Kind == GoNamed || base.Kind == GoInterface {
		if contract := underlyingGoInterface(base.GoType); contract != nil && contract.IsMethodSet() {
			if count := goTypeParameterCount(base.GoType); count > len(base.TypeArguments) {
				c.report(ref.Span, fmt.Sprintf("embedded Go interface %s requires %d type arguments", base.String(), count))
				return nil, false
			}
			return c.embeddedGoInterfaceMethods(base, contract, ref.Span, "anonymous interface cannot embed "+base.String())
		}
	}
	c.report(ref.Span, "anonymous interface embedding requires a Go method interface or another anonymous interface")
	return nil, false
}

func (c *Checker) anonymousInterfaceStorage(value Type) (gotypes.Type, bool) {
	functions := make([]*gotypes.Func, 0, len(value.GoMethods))
	for _, method := range value.GoMethods {
		storage, ok := c.goTypeForNativeStorage(method.Type)
		signature, callable := storage.(*gotypes.Signature)
		if !ok || !callable {
			return nil, false
		}
		name := method.Name
		if method.GoName != "" {
			name = method.GoName
		}
		functions = append(functions, gotypes.NewFunc(0, nil, name, signature))
	}
	return gotypes.NewInterfaceType(functions, nil).Complete(), true
}
