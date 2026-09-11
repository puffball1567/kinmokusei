package lsp

import "github.com/puffball1567/kinmokusei/internal/ast"

// Completion consumes source spellings even after the linker has renamed private
// dependency declarations. Invalid imports must not advertise hidden bindings.
func sourceImportVisible(program *ast.Program, path, name string) bool {
	explicit := false
	for _, exported := range program.Exports {
		if !samePath(exported.Span.Path, path) {
			continue
		}
		explicit = true
		for _, selected := range exported.Names {
			if selected.Name == name {
				return true
			}
		}
	}
	return !explicit
}
