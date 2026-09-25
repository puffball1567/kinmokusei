package sema

import (
	"fmt"
	"go/constant"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type goNamedImport struct {
	pack *goPackageSymbol
	span source.Span
	name string
}

// IsReservedImportAlias identifies names whose compiler-defined meaning cannot
// be shadowed by a source or Go import. Ordinary built-in functions remain
// shadowable, just like local function declarations.
func IsReservedImportAlias(name string) bool {
	switch name {
	case "Map", "GoChannel", "GoSendChannel", "GoReceiveChannel", "comparable":
		return true
	}
	return isBuiltinTypeName(name) || isBuiltinValueName(name)
}

func (c *Checker) declareNamedGoImports(imported *goPackageSymbol, program *ast.Program) {
	declaration := imported.declaration
	bindings := c.goNamedImports[declaration.Span.Path]
	if bindings == nil {
		bindings = map[string]goNamedImport{}
		c.goNamedImports[declaration.Span.Path] = bindings
	}
	for i, selected := range declaration.Names {
		name, span := declaration.BindingName(i), declaration.BindingSpan(i)
		if declaration.HasNameAlias(i) && IsReservedImportAlias(name) {
			c.report(span, fmt.Sprintf("import alias %q conflicts with a compiler built-in", name))
			continue
		}
		if _, duplicate := bindings[name]; duplicate {
			c.report(span, fmt.Sprintf("duplicate import binding %q", name))
			continue
		}
		for _, other := range program.Imports {
			if other.Span.Path != span.Path {
				continue
			}
			if other.Go && len(other.Names) == 0 && other.Alias == name {
				c.report(span, fmt.Sprintf("imported name %q conflicts with a Go package alias", name))
			}
			if !other.Go {
				for index := range other.Names {
					if other.BindingName(index) == name {
						c.report(span, fmt.Sprintf("duplicate import binding %q", name))
					}
				}
			}
		}
		object := imported.packageInfo.Scope().Lookup(selected)
		if object == nil || !object.Exported() {
			c.report(span, fmt.Sprintf("Go package %q has no exported member %q", imported.path, selected))
			continue
		}
		bindings[name] = goNamedImport{pack: imported, span: span, name: selected}
	}
}

func (c *Checker) lookupNamedGoImport(name string, span source.Span) (goNamedImport, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if _, local := c.scopes[i][name]; local {
			return goNamedImport{}, false
		}
	}
	imported, ok := c.goNamedImports[span.Path][name]
	return imported, ok
}

func (c *Checker) checkNamedGoIdentifier(identifier *ast.IdentifierExpr, imported goNamedImport) Type {
	alias := imported.pack.declaration.ResolvedAlias
	if alias == "" {
		alias = imported.pack.declaration.Alias
	}
	member := &ast.MemberExpr{
		Object: &ast.IdentifierExpr{Name: alias, Span: identifier.Span},
		Name:   imported.name, Span: identifier.Span,
	}
	result := c.checkGoMember(member, imported.pack)
	identifier.GoMember = member
	identifier.ResolvedDeclaration = imported.span
	if value, ok := c.constantValues[member]; ok {
		c.constantValues[identifier] = value
	}
	return result
}

func (c *Checker) namedGoConstant(identifier *ast.IdentifierExpr) *gotypes.Const {
	imported, ok := c.lookupNamedGoImport(identifier.Name, identifier.Span)
	if !ok {
		return nil
	}
	value, _ := imported.pack.packageInfo.Scope().Lookup(imported.name).(*gotypes.Const)
	return value
}

func (c *Checker) namedGoConstantValue(identifier *ast.IdentifierExpr) constant.Value {
	if value := c.namedGoConstant(identifier); value != nil {
		return value.Val()
	}
	return nil
}
