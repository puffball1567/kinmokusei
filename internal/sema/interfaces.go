package sema

import (
	"fmt"
	gotypes "go/types"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
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
			if _, exists := symbol.methods[method.Name]; exists {
				c.report(method.Span, fmt.Sprintf("duplicate interface method %q", method.Name))
				continue
			}
			parameters := make([]Type, len(method.Parameters))
			for j, parameter := range method.Parameters {
				resolved := c.resolveType(parameter.Type)
				c.rejectResultValueType(resolved, parameter.Type.Span, "parameters")
				c.rejectTaskAPIType(resolved, parameter.Type.Span, "interface parameters")
				parameters[j] = c.callableParameterType(parameter, resolved)
			}
			result := c.resolveType(method.ReturnType)
			c.rejectTaskAPIType(result, method.ReturnType.Span, "interface return types")
			method.GoName = memberGoName(method.Name, ast.Public)
			symbol.methods[method.Name] = methodSymbol{
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
					if !exactType(existing.typeInfo, inherited.typeInfo) {
						c.report(ref.Span, fmt.Sprintf("interface %s inherits incompatible signatures for method %s", name, methodName))
					}
					continue
				}
				symbol.methods[methodName] = inherited
			}
		}
		state[name] = 2
	}
	for _, declaration := range program.Declarations {
		if decl, ok := declaration.(*ast.InterfaceDecl); ok && !decl.Constraint {
			complete(decl.Name)
		}
	}
}

// interfaceAncestors preserves declaration identity while substituting each
// edge separately; same-spelled generic parameters in different bases are not
// interchangeable. A visited path also bounds traversal of invalid cycles.
func (c *Checker) interfaceAncestors(value Type) []Type {
	var result []Type
	visiting := map[string]bool{}
	seen := map[string][]Type{}
	var visit func(Type)
	visit = func(current Type) {
		if visiting[current.Name] {
			return
		}
		for _, previous := range seen[current.Name] {
			if exactType(previous, current) {
				return
			}
		}
		seen[current.Name] = append(seen[current.Name], current)
		result = append(result, current)
		if current.Kind != Interface {
			return
		}
		symbol := c.interfaces[current.Name]
		if symbol == nil {
			return
		}
		visiting[current.Name] = true
		bindings := nativeInterfaceBindings(symbol, current)
		for _, base := range symbol.bases {
			visit(substituteNativeTypeParameters(base, bindings))
		}
		delete(visiting, current.Name)
	}
	visit(value)
	return result
}

func (c *Checker) interfaceExtends(value, target Type) bool {
	for _, ancestor := range c.interfaceAncestors(value) {
		if exactType(ancestor, target) {
			return true
		}
	}
	return false
}

func underlyingGoInterface(goType gotypes.Type) *gotypes.Interface {
	if goType == nil {
		return nil
	}
	contract, _ := gotypes.Unalias(goType).Underlying().(*gotypes.Interface)
	if contract != nil {
		contract.Complete()
	}
	return contract
}

func (c *Checker) validateGoInterfaceImplementation(className string, class *classSymbol, contract Type, goInterface *gotypes.Interface, span source.Span) {
	for i := 0; i < goInterface.NumMethods(); i++ {
		required := goInterface.Method(i)
		var actual methodSymbol
		found := false
		for _, candidate := range class.methods {
			if candidate.goName == required.Name() {
				actual = candidate
				found = true
				break
			}
		}
		if !found {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: missing exported method %s", className, contract.String(), required.Name()))
			continue
		}
		if actual.visibility != ast.Public {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: method %s must be public", className, contract.String(), required.Name()))
			continue
		}
		if actual.static {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: method %s cannot be static", className, contract.String(), required.Name()))
			continue
		}
		actualType, ok := goTypeOf(actual.typeInfo)
		if !ok || !gotypes.Identical(actualType, required.Type()) {
			c.report(span, fmt.Sprintf("class %s cannot implement Go interface %s: method %s has %s, expected %s", className, contract.String(), required.Name(), actual.typeInfo.String(), goTypeDisplayName(required.Type())))
		}
	}
}
