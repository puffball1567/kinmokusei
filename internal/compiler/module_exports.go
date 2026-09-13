package compiler

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/stdlib"
)

// Re-exports refer to the original declaration, never a copied binding or a
// generated forwarding function. The module key is the loader's stable identity.
type sourceExportBinding struct {
	module, name string
	span         source.Span
}

func (l *moduleLoader) resolveSourceExports(key string, program *ast.Program) {
	if l.exports == nil {
		l.exports = map[string]map[string]sourceExportBinding{}
	}
	locals := map[string]sourceExportBinding{}
	for _, declaration := range program.Declarations {
		if name, span := ast.DeclarationBinding(declaration); name != "" {
			locals[name] = sourceExportBinding{key, name, span}
		}
	}
	if len(program.Exports) == 0 {
		l.exports[key] = locals
		return
	}
	visible := map[string]sourceExportBinding{}
	for name, binding := range locals {
		visible[name] = binding
	}
	for _, imported := range program.Imports {
		if imported.Go {
			continue
		}
		for _, name := range imported.Names {
			if _, local := locals[name]; local {
				continue
			}
			if binding, ok := l.exports[imported.ResolvedPath][name]; ok {
				visible[name] = binding
			}
		}
	}
	result := map[string]sourceExportBinding{}
	for _, exported := range program.Exports {
		bindings := visible
		if exported.Path != "" {
			bindings = l.exports[exported.ResolvedPath]
		}
		for i := range exported.Names {
			name := &exported.Names[i]
			if binding, ok := bindings[name.Name]; ok {
				name.ResolvedDeclaration = binding.span
				result[name.Name] = binding
			}
		}
	}
	l.exports[key] = result
}

func (l *moduleLoader) loadSourceDependencies(path string, program *ast.Program, embedded bool) error {
	type dependency struct {
		imported *ast.ImportDecl
		exported *ast.ExportDecl
	}
	var dependencies []dependency
	for i := range program.Imports {
		if !program.Imports[i].Go {
			dependencies = append(dependencies, dependency{imported: &program.Imports[i]})
		}
	}
	for i := range program.Exports {
		exported := &program.Exports[i]
		if exported.Path == "" {
			continue
		}
		imported := &ast.ImportDecl{Path: exported.Path, PathSpan: exported.PathSpan, Span: exported.Span}
		for _, name := range exported.Names {
			imported.Names = append(imported.Names, name.Name)
			imported.NameSpans = append(imported.NameSpans, name.NameSpan)
		}
		dependencies = append(dependencies, dependency{imported, exported})
	}
	// A mixed import/export dependency list follows source order. Diamond paths
	// still initialize once through the loader's visited-module state.
	sort.SliceStable(dependencies, func(i, j int) bool {
		return dependencies[i].imported.Span.Start.Offset < dependencies[j].imported.Span.Start.Offset
	})
	for _, dependency := range dependencies {
		if err := l.loadSourceDependency(path, dependency.imported, embedded); err != nil {
			return err
		}
		if dependency.exported != nil {
			dependency.exported.ResolvedPath = dependency.imported.ResolvedPath
		}
	}
	return nil
}

// Imports and export-from declarations share dependency loading and visibility
// validation, but only imports introduce names into the importing module.
func (l *moduleLoader) loadSourceDependency(path string, imported *ast.ImportDecl, embedded bool) error {
	if !strings.HasPrefix(imported.Path, ".") {
		standardSource, found := stdlib.Lookup(imported.Path)
		if !found {
			message := "package imports are not supported in this compiler stage"
			if strings.HasPrefix(imported.Path, "kinmokusei/") {
				message = fmt.Sprintf("standard package %q is not available", imported.Path)
			}
			l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: message, Span: imported.PathSpan})
			return nil
		}
		imported.ResolvedPath = filepath.FromSlash(standardSource.VirtualPath)
		if err := l.loadSource(imported.ResolvedPath, imported.ResolvedPath, standardSource.Contents, imported, true); err != nil {
			return err
		}
	} else {
		if embedded {
			l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
				Message: fmt.Sprintf("compiler-managed module %q cannot use relative import %q", path, imported.Path), Span: imported.PathSpan,
			})
			return nil
		}
		target := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(imported.Path)))
		if filepath.Ext(target) == "" {
			target = sourcePathWithMigrationFallback(target)
		}
		absolute, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		imported.ResolvedPath = filepath.Clean(absolute)
		if err := l.load(target, imported); err != nil {
			return err
		}
	}
	// A cycle target is still being resolved. Its absent export table is not a
	// visibility failure; loadSource has already reported the cycle itself.
	if l.states[imported.ResolvedPath] == 2 {
		l.validateImport(*imported, l.programs[imported.ResolvedPath])
	}
	return nil
}
