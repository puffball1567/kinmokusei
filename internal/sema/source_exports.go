package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkSourceExports(program *ast.Program) {
	type binding struct{ path, name string }
	locals := map[binding]source.Span{}
	for _, declaration := range program.Declarations {
		name, span := ast.DeclarationBinding(declaration)
		if name != "" {
			locals[binding{declaration.GetSpan().Path, name}] = span
		}
	}
	seen := map[binding]bool{}
	for _, exported := range program.Exports {
		for i := range exported.Names {
			name := &exported.Names[i]
			key := binding{exported.Span.Path, name.Name}
			if exported.Path != "" && name.ResolvedDeclaration.Path == "" {
				c.diagnostics = append(c.diagnostics, diagnostic.Diagnostic{Message: "re-export requires a resolved source module", Span: name.NameSpan})
				continue
			}
			if name.ResolvedName != "" {
				key.name = name.ResolvedName
			}
			if name.ResolvedDeclaration.Path != "" {
				key.path = name.ResolvedDeclaration.Path
			}
			name.ResolvedDeclaration = source.Span{}
			if span, exists := locals[key]; exists {
				name.ResolvedDeclaration = span
			} else {
				c.diagnostics = append(c.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("exported name %q is not a local top-level declaration", name.Name), Span: name.NameSpan})
			}
			exportKey := binding{exported.Span.Path, name.Name}
			if seen[exportKey] {
				c.diagnostics = append(c.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate exported name %q", name.Name), Span: name.NameSpan})
			}
			seen[exportKey] = true
		}
	}
}
