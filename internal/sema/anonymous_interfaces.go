package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
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
	names := map[string]bool{}
	valid := true
	for _, method := range ref.ObjectFields {
		name := method.Name
		goName := method.GoName
		if !ref.Go {
			goName = memberGoName(name, ast.Public)
		}
		storageName := name
		if goName != "" {
			storageName = goName
		}
		if names[storageName] {
			c.report(method.Span, fmt.Sprintf("anonymous interface has conflicting Go method name %q", storageName))
			valid = false
		}
		names[storageName] = true
		callable := c.resolveType(method.Type)
		callable.ParameterNames = append([]string(nil), method.ParameterNames...)
		if callable.Kind != Function || callable.Result == nil {
			c.report(method.Span, fmt.Sprintf("anonymous interface method %q requires a callable signature", name))
			valid = false
		}
		result.GoMethods = append(result.GoMethods, GoInterfaceMethod{Name: name, GoName: goName, Type: callable, DeclarationSpan: method.NameSpan})
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
	return result
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
