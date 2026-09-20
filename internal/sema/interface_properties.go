package sema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Validate the completed contract, not only locally declared accessors: a
// getter and setter can come from different generic/diamond ancestors.
func (c *Checker) checkInterfacePropertyContracts(decl *ast.InterfaceDecl, symbol *interfaceSymbol) {
	keys := make([]string, 0, len(symbol.methods))
	for key := range symbol.methods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		accessor, name, property := strings.Cut(key, " ")
		if !property {
			continue
		}
		if _, exists := symbol.methods[name]; exists {
			c.report(decl.NameSpan, fmt.Sprintf("interface property %q conflicts with a method", name))
		}
		getter, get := symbol.methods["get "+name]
		setter, set := symbol.methods["set "+name]
		if accessor == "get" && get && set && getter.typeInfo.Result != nil && len(setter.typeInfo.Parameters) == 1 {
			left := Type{Kind: Function, Name: "function", Result: getter.typeInfo.Result}
			right := Type{Kind: Function, Name: "function", Result: &setter.typeInfo.Parameters[0]}
			if !identicalMethodSignature(left, right) {
				c.report(decl.NameSpan, fmt.Sprintf("getter and setter for %q must have identical types, including nullability", name))
			}
		}
		method := symbol.methods[key]
		for _, otherKey := range keys {
			other := symbol.methods[otherKey]
			// An imported Go method with the same signature may describe the
			// same exported accessor; it retains its ordinary Go spelling.
			if otherKey != key && other.goName == method.goName && !other.goInterfaceMethod {
				c.report(decl.NameSpan, fmt.Sprintf("generated property method %q conflicts with another interface member", method.goName))
			}
		}
	}
}
