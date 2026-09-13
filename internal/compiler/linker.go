package compiler

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type moduleNames map[string]string

func (l *moduleLoader) linkModules(rootPaths []string) map[string]map[string]bool {
	roots := map[string]bool{}
	for _, path := range rootPaths {
		absolute, err := filepath.Abs(path)
		if err == nil {
			roots[filepath.Clean(absolute)] = true
		}
	}

	paths := make([]string, 0, len(l.programs))
	nameCounts := map[string]int{}
	for path, program := range l.programs {
		paths = append(paths, path)
		for _, declaration := range program.Declarations {
			if name := topLevelName(declaration); name != "" {
				nameCounts[name]++
			}
		}
	}
	sort.Strings(paths)

	bindings := map[string]moduleNames{}
	declarationNames := map[source.Span]string{}
	for _, path := range paths {
		bindings[path] = moduleNames{}
		for _, declaration := range l.programs[path].Declarations {
			name := topLevelName(declaration)
			if name == "" {
				continue
			}
			linked := name
			if nameCounts[name] > 1 && !roots[path] {
				linked = l.linkedModuleName(path, name)
			}
			bindings[path][name] = linked
			_, span := ast.DeclarationBinding(declaration)
			declarationNames[span] = linked
		}
	}
	// Capture visibility before linking mutates declaration and export names.
	exportedBindings := map[string]moduleNames{}
	for _, path := range paths {
		exportedBindings[path] = moduleNames{}
		for name, origin := range l.exports[path] {
			exportedBindings[path][name] = bindings[origin.module][origin.name]
		}
		for _, exported := range l.programs[path].Exports {
			for i := range exported.Names {
				name := &exported.Names[i]
				name.ResolvedName = declarationNames[name.ResolvedDeclaration]
			}
		}
	}
	goAliasBindings, canonicalGoAliases := l.linkGoAliases(paths, bindings)

	allowed := map[string]map[string]bool{}
	linker := &sourceLinker{unimported: map[source.Span]bool{}}
	for _, path := range paths {
		program := l.programs[path]
		moduleBindings := moduleNames{}
		moduleAllowed := map[string]bool{}
		for sourceName, linked := range bindings[path] {
			moduleBindings[sourceName] = linked
			moduleAllowed[linked] = true
		}
		for sourceAlias, linked := range goAliasBindings[path] {
			moduleBindings[sourceAlias] = linked
		}
		seenImports := map[string]bool{}
		for sourceAlias := range goAliasBindings[path] {
			if _, local := bindings[path][sourceAlias]; local {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
					Message: fmt.Sprintf("Go package alias %q conflicts with a declaration in the same module", sourceAlias), Span: importAliasSpan(program, sourceAlias),
				})
			}
			seenImports[sourceAlias] = true
		}
		for _, imported := range program.Imports {
			if imported.Go {
				for _, name := range imported.Names {
					if _, local := bindings[path][name]; local {
						l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("imported name %q conflicts with a declaration in the same module", name), Span: imported.Span})
					}
					if seenImports[name] {
						l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate import binding %q", name), Span: imported.Span})
					}
					seenImports[name] = true
				}
				continue
			}
			if imported.ResolvedPath == "" {
				continue
			}
			targetBindings := exportedBindings[imported.ResolvedPath]
			for _, name := range imported.Names {
				if _, local := bindings[path][name]; local {
					l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
						Message: fmt.Sprintf("imported name %q conflicts with a declaration in the same module", name), Span: imported.Span,
					})
					continue
				}
				if seenImports[name] {
					l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate import binding %q", name), Span: imported.Span})
					continue
				}
				seenImports[name] = true
				if linked, exists := targetBindings[name]; exists {
					moduleBindings[name] = linked
					moduleAllowed[linked] = true
				}
			}
		}
		// The lexical linker records unimported runtime spellings, while sema
		// retains ordinary local/type-parameter and built-in resolution.
		var targets []string
		for _, linked := range moduleBindings {
			targets = append(targets, linked)
		}
		for _, linked := range targets {
			if _, visible := moduleBindings[linked]; !visible {
				moduleBindings[linked] = ""
			}
		}
		linker.linkProgram(program, bindings[path], moduleBindings)
		allowed[l.paths[path]] = moduleAllowed
	}
	for i := range l.merged.Imports {
		if imported := &l.merged.Imports[i]; imported.Go {
			imported.ResolvedAlias = canonicalGoAliases[imported.Path]
		}
	}
	l.merged.UnimportedReferences = linker.unimported
	return allowed
}

func (l *moduleLoader) linkGoAliases(paths []string, declarationBindings map[string]moduleNames) (map[string]moduleNames, map[string]string) {
	bindings := map[string]moduleNames{}
	canonical := map[string]string{}
	namedPaths := map[string]bool{}
	for _, path := range paths {
		for _, imported := range l.programs[path].Imports {
			if imported.Go && len(imported.Names) != 0 {
				namedPaths[imported.Path] = true
			}
		}
	}
	usedAliases := map[string]string{
		"bool": "<Go built-in>", "string": "<Go built-in>", "int": "<Go built-in>", "int32": "<Go built-in>",
		"int64": "<Go built-in>", "float32": "<Go built-in>", "float64": "<Go built-in>", "byte": "<Go built-in>",
	}
	for _, moduleBindings := range declarationBindings {
		for _, linked := range moduleBindings {
			usedAliases[linked] = "<language declaration>"
		}
	}
	for _, path := range paths {
		bindings[path] = moduleNames{}
		seenPaths := map[string]bool{}
		seenAliases := map[string]bool{}
		for i := range l.programs[path].Imports {
			imported := &l.programs[path].Imports[i]
			if !imported.Go {
				continue
			}
			named := len(imported.Names) != 0
			if named {
				imported.Alias = ast.GoImportAlias(imported.Path)
			}
			if isReservedLanguageTypeName(imported.Alias) {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("Go package alias %q conflicts with a built-in type", imported.Alias), Span: imported.Span})
				continue
			}
			if !named && seenPaths[imported.Path] {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate Go package import %q", imported.Path), Span: imported.Span})
				continue
			}
			if !named {
				seenPaths[imported.Path] = true
			}
			if !named && seenAliases[imported.Alias] {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate import binding %q", imported.Alias), Span: imported.Span})
				continue
			}
			seenAliases[imported.Alias] = true
			linked, exists := canonical[imported.Path]
			if !exists {
				linked = imported.Alias
				if namedPaths[imported.Path] {
					linked = ast.GoImportAlias(imported.Path)
				}
				if previousPath, used := usedAliases[linked]; used && previousPath != imported.Path {
					linked = linkedGoAlias(imported.Path, imported.Alias)
				}
				canonical[imported.Path] = linked
				usedAliases[linked] = imported.Path
			}
			if !named {
				bindings[path][imported.Alias] = linked
			}
			imported.ResolvedAlias = linked
		}
	}
	return bindings, canonical
}

func importAliasSpan(program *ast.Program, alias string) source.Span {
	for _, imported := range program.Imports {
		if imported.Go && imported.Alias == alias {
			return imported.Span
		}
	}
	return source.Span{}
}

func isReservedLanguageTypeName(name string) bool {
	switch name {
	case "void", "boolean", "string", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float", "number", "float64", "byte", "error", "Map", "Result":
		return true
	default:
		return false
	}
}

func linkedGoAlias(importPath, alias string) string {
	digest := sha256.Sum256([]byte(importPath))
	return fmt.Sprintf("%s_%x", alias, digest[:4])
}

func topLevelName(declaration ast.Declaration) string {
	name, _ := ast.DeclarationBinding(declaration)
	return name
}

func (l *moduleLoader) linkedModuleName(path, name string) string {
	stablePath := path
	if l.linkBase != "" {
		if relative, err := filepath.Rel(l.linkBase, path); err == nil {
			stablePath = relative
		}
	}
	base := strings.TrimSuffix(filepath.Base(stablePath), filepath.Ext(stablePath))
	var cleaned strings.Builder
	for _, character := range base {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' {
			cleaned.WriteRune(character)
		} else {
			cleaned.WriteByte('_')
		}
	}
	digest := sha256.Sum256([]byte(filepath.ToSlash(stablePath)))
	return fmt.Sprintf("_kinmokusei_%s_%x_%s", cleaned.String(), digest[:4], name)
}
