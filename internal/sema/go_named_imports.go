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
}

func (c *Checker) declareNamedGoImports(imported *goPackageSymbol, program *ast.Program) {
	declaration := imported.declaration
	bindings := c.goNamedImports[declaration.Span.Path]
	if bindings == nil {
		bindings = map[string]goNamedImport{}
		c.goNamedImports[declaration.Span.Path] = bindings
	}
	for i, name := range declaration.Names {
		span := declaration.Span
		if i < len(declaration.NameSpans) {
			span = declaration.NameSpans[i]
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
				for _, otherName := range other.Names {
					if otherName == name {
						c.report(span, fmt.Sprintf("duplicate import binding %q", name))
					}
				}
			}
		}
		object := imported.packageInfo.Scope().Lookup(name)
		if object == nil || !object.Exported() {
			c.report(span, fmt.Sprintf("Go package %q has no exported member %q", imported.path, name))
			continue
		}
		bindings[name] = goNamedImport{pack: imported, span: span}
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
		Name:   identifier.Name, Span: identifier.Span,
	}
	result := c.checkGoMember(member, imported.pack)
	identifier.GoMember = member
	identifier.ResolvedDeclaration = imported.span
	if value, ok := c.numericValues[member]; ok {
		c.numericValues[identifier] = value
	}
	return result
}

func (c *Checker) namedGoConstant(identifier *ast.IdentifierExpr) *gotypes.Const {
	imported, ok := c.lookupNamedGoImport(identifier.Name, identifier.Span)
	if !ok {
		return nil
	}
	value, _ := imported.pack.packageInfo.Scope().Lookup(identifier.Name).(*gotypes.Const)
	return value
}

func (c *Checker) namedGoConstantValue(identifier *ast.IdentifierExpr) constant.Value {
	if value := c.namedGoConstant(identifier); value != nil {
		return value.Val()
	}
	return nil
}
