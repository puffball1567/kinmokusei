package lsp

import (
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func sourcePublicExport(program *ast.Program, path, name string) (ast.ExportName, bool) {
	for _, exported := range program.Exports {
		if !samePath(exported.Span.Path, path) {
			continue
		}
		for _, selected := range exported.Names {
			if selected.PublicName() == name && selected.ResolvedDeclaration.Path != "" {
				return selected, true
			}
		}
	}
	return ast.ExportName{}, false
}

func (s *Server) publicDeclarationSpan(program *ast.Program, path, name string, textByPath map[string]string) (source.Span, bool) {
	if selected, ok := sourcePublicExport(program, path, name); ok {
		return selected.PublicDeclaration(), true
	}
	return s.topLevelDeclarationSpan(program, path, name, textByPath)
}

type exportAliasReference struct {
	path, name string
	origin     sourceSpanKey
}

// Runtime references retain the original declaration. Rename instead follows
// the imported public spelling, and stops at each explicit `as` boundary.
func (s *Server) importedExportAliases(program *ast.Program) map[exportAliasReference]source.Span {
	result := map[exportAliasReference]source.Span{}
	for _, imported := range program.Imports {
		if imported.Go {
			continue
		}
		for _, name := range imported.Names {
			if selected, ok := sourcePublicExport(program, imported.ResolvedPath, name); ok {
				public := selected.PublicDeclaration()
				if !sameSourceSpan(public, selected.ResolvedDeclaration) {
					result[exportAliasReference{cleanPath(imported.Span.Path), name, spanKey(selected.ResolvedDeclaration)}] = public
				}
			}
		}
	}
	return result
}

func (s *Server) exportAliasInfo(program *ast.Program, target source.Span) (declarationInfo, bool) {
	for _, exported := range program.Exports {
		for _, name := range exported.Names {
			if name.Alias == "" || !sameSourceSpan(name.AliasSpan, target) {
				continue
			}
			info := declarationInfo{Name: name.Alias, Detail: "export alias " + name.Alias, Kind: 13, Span: exported.Span, Selection: name.AliasSpan}
			for _, original := range flattenDeclarations(collectDeclarations(program)) {
				if sameSourceSpan(original.Selection, name.ResolvedDeclaration) {
					info.Kind = original.Kind
					info.Detail = strings.Replace(original.Detail, original.Name, name.Alias, 1)
					break
				}
			}
			return info, true
		}
	}
	return declarationInfo{}, false
}
