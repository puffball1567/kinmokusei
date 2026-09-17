package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

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
