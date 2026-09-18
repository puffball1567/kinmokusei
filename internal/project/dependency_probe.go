package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/stdlib"
)

func writeDependencyProbe(root, directory string, graphs ...*PackageGraph) error {
	imports := map[string]bool{}
	var graph *PackageGraph
	if len(graphs) != 0 {
		graph = graphs[0]
	}
	// Preserve the application's existing all-source discovery. Dependencies
	// nested inside it still use their public source boundary, not this walk.
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == product.StateDirectoryName || graph != nil && graph.owner(path) != nil) {
				return filepath.SkipDir
			}
			return nil
		}
		if !product.IsSourceExtension(filepath.Ext(path)) {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Local source may be unfinished while adding dependencies.
		tokens, _ := lexer.Lex(path, string(contents))
		program, _ := parser.Parse(tokens)
		for _, imported := range program.Imports {
			if imported.Go {
				imports[imported.Path] = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if graph != nil {
		if err := graph.collectPackageGoImports(imports); err != nil {
			return err
		}
	}
	var source strings.Builder
	source.WriteString("package kinmokuseidependencies\n")
	if len(imports) != 0 {
		source.WriteString("\nimport (\n")
		for _, path := range sortedKeys(imports) {
			fmt.Fprintf(&source, "\t_ %s\n", strconv.Quote(path))
		}
		source.WriteString(")\n")
	}
	return os.WriteFile(filepath.Join(directory, "kinmokusei_dependencies.go"), []byte(source.String()), 0o644)
}

// A source package's entry and every public submodule are supported APIs, even
// if the current consumer imports only one. Follow their source dependencies;
// unrelated examples, tests and nested projects are not part of the API.
func (g *PackageGraph) collectPackageGoImports(imports map[string]bool) error {
	states := map[string]int{}
	var visit func(string, *stdlib.Source) error
	visit = func(file string, embedded *stdlib.Source) error {
		if embedded == nil {
			file = canonicalPackageFile(file)
		}
		if states[file] == 1 {
			return fmt.Errorf("package source import cycle through %s", file)
		}
		if states[file] == 2 {
			return nil
		}
		states[file] = 1
		var contents string
		if embedded != nil {
			contents = embedded.Contents
		} else {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("cannot read package source %s: %w", file, err)
			}
			contents = string(data)
		}
		tokens, diagnostics := lexer.Lex(file, contents)
		program, parseDiagnostics := parser.Parse(tokens)
		diagnostics = append(diagnostics, parseDiagnostics...)
		if len(diagnostics) != 0 {
			return fmt.Errorf("invalid package source: %s", diagnostics[0])
		}
		var sources []ast.ImportDecl
		for _, imported := range program.Imports {
			if !imported.Go {
				sources = append(sources, imported)
				continue
			}
			if embedded == nil {
				if err := g.ValidateGoImport(file, imported.Path); err != nil {
					return err
				}
			}
			imports[imported.Path] = true
		}
		for _, exported := range program.Exports {
			if exported.Path != "" {
				sources = append(sources, ast.ImportDecl{Path: exported.Path})
			}
		}
		for _, imported := range sources {
			if standard, found := stdlib.Lookup(imported.Path); found {
				if err := visit(standard.VirtualPath, &standard); err != nil {
					return err
				}
				continue
			}
			var target string
			var err error
			if strings.HasPrefix(imported.Path, ".") {
				if embedded != nil {
					return fmt.Errorf("compiler-managed module cannot use relative import %q", imported.Path)
				}
				target = filepath.Join(filepath.Dir(file), filepath.FromSlash(imported.Path))
				if filepath.Ext(target) == "" {
					target += product.SourceExtension
				}
				err = g.ValidateRelativeImport(file, target)
			} else {
				target, err = g.ResolveImport(file, imported.Path)
			}
			if err != nil {
				return fmt.Errorf("package source %s: %w", file, err)
			}
			if err := visit(target, nil); err != nil {
				return err
			}
		}
		states[file] = 2
		return nil
	}
	for _, module := range sortedKeys(g.Packages) {
		pkg := g.Packages[module]
		for _, source := range append([]string{pkg.Manifest.Package.Entry}, mapValues(pkg.Manifest.Exports)...) {
			file, err := packageSourceFile(pkg.Directory, source)
			if err != nil {
				return err
			}
			if err := visit(file, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
