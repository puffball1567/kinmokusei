package sema

import (
	"fmt"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) declareInterfaces(program *ast.Program) {
	declarations := map[string]*ast.InterfaceDecl{}
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.InterfaceDecl)
		if !ok || decl.Constraint {
			continue
		}
		declarations[decl.Name] = decl
		decl.InheritedGoMethods = nil
		symbol := c.interfaces[decl.Name]
		if symbol == nil {
			symbol = &interfaceSymbol{methods: map[string]methodSymbol{}, declarationSpan: decl.NameSpan}
			c.interfaces[decl.Name] = symbol
		}
		symbol.methods = map[string]methodSymbol{}
		c.pushTypeParameterScope(symbol.typeParamScope)
		for i := range decl.Methods {
			method := &decl.Methods[i]
			key := method.Name
			if method.Accessor != "" {
				key = method.Accessor + " " + key
			}
			if _, exists := symbol.methods[key]; exists {
				c.report(method.Span, fmt.Sprintf("duplicate interface method %q", method.Name))
				continue
			}
			parameters := make([]Type, len(method.Parameters))
			for j, parameter := range method.Parameters {
				resolved := c.resolveType(parameter.Type)
				if method.Accessor == "" {
					c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
					c.rejectTaskAPIType(resolved, parameter.Type.Span, "interface parameters")
				}
				parameters[j] = c.callableParameterType(parameter, resolved)
			}
			result := c.resolveType(method.ReturnType)
			if method.Accessor == "" {
				c.rejectTaskAPIType(result, method.ReturnType.Span, "interface return types")
			}
			method.GoName = memberGoName(method.Name, ast.Public)
			if method.Accessor != "" {
				method.GoName = memberGoName(method.Accessor+memberGoName(method.Name, ast.Public), ast.Public)
				c.checkAccessorSignature(method.Accessor, parameters, result, hasVariadicParameter(method.Parameters), method.Span)
			}
			symbol.methods[key] = methodSymbol{
				typeInfo:   Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: hasVariadicParameter(method.Parameters), Result: &result},
				visibility: ast.Public, goName: method.GoName, declarationSpan: method.NameSpan,
			}
		}
		c.popTypeParameterScope()
	}
	// Declare all own methods before following bases, so forward declarations
	// and diamond inheritance have the same contracts regardless of file order.
	state := map[string]uint8{}
	var complete func(string)
	complete = func(name string) {
		if state[name] != 0 {
			return
		}
		state[name] = 1
		decl, symbol := declarations[name], c.interfaces[name]
		previousScopes := c.typeParameterScopes
		c.typeParameterScopes = nil
		c.pushTypeParameterScope(symbol.typeParamScope)
		defer func() { c.typeParameterScopes = previousScopes }()
		seen := map[string]bool{}
		for _, ref := range decl.Bases {
			base := c.resolveType(ref)
			if base.Kind == Invalid {
				continue
			}
			if base.Kind == GoNamed && underlyingGoInterface(base.GoType) != nil {
				if seen[base.String()] {
					c.report(ref.Span, fmt.Sprintf("duplicate extended interface %s", base.String()))
					continue
				}
				seen[base.String()] = true
				if c.inheritGoInterface(decl, symbol, base, ref.Span) {
					symbol.bases = append(symbol.bases, base)
				}
				continue
			}
			if base.Kind != Interface {
				c.report(ref.Span, "interface extends expects a source interface")
				continue
			}
			if seen[base.String()] {
				c.report(ref.Span, fmt.Sprintf("duplicate extended interface %s", base.String()))
				continue
			}
			seen[base.String()] = true
			if state[base.Name] == 1 {
				c.report(ref.Span, fmt.Sprintf("interface inheritance cycle involving %s", base.Name))
				continue
			}
			complete(base.Name)
			symbol.bases = append(symbol.bases, base)
			parent := c.interfaces[base.Name]
			bindings := nativeInterfaceBindings(parent, base)
			names := make([]string, 0, len(parent.methods))
			for methodName := range parent.methods {
				names = append(names, methodName)
			}
			sort.Strings(names)
			for _, methodName := range names {
				inherited := parent.methods[methodName]
				inherited.typeInfo = substituteNativeTypeParameters(inherited.typeInfo, bindings)
				c.checkInheritedGoMethodConflict(name, symbol, inherited, ref.Span)
				if existing, exists := symbol.methods[methodName]; exists {
					if !identicalMethodSignature(existing.typeInfo, inherited.typeInfo) {
						c.report(ref.Span, fmt.Sprintf("interface %s inherits incompatible signatures for method %s", name, methodName))
					}
					continue
				}
				symbol.methods[methodName] = inherited
			}
		}
		c.checkInterfacePropertyContracts(decl, symbol)
		state[name] = 2
	}
	for _, declaration := range program.Declarations {
		if decl, ok := declaration.(*ast.InterfaceDecl); ok && !decl.Constraint {
			complete(decl.Name)
		}
	}
}
