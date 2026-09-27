package lsp

import (
	"fmt"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (s *Server) importAliasInfo(program *ast.Program, target source.Span) (declarationInfo, bool) {
	for _, imported := range program.Imports {
		for index, name := range imported.Names {
			if !imported.HasNameAlias(index) || !sameSourceSpan(imported.BindingSpan(index), target) {
				continue
			}
			alias := imported.BindingName(index)
			prefix := "import"
			if imported.Go {
				prefix += " go"
			}
			info := declarationInfo{Name: alias, Detail: fmt.Sprintf("%s { %s as %s } from %q", prefix, name, alias, imported.Path), Kind: 13, Span: imported.Span, Selection: target}
			if origin, ok := s.topLevelDeclarationSpan(program, imported.ResolvedPath, name, nil); !imported.Go && ok {
				for _, original := range flattenDeclarations(collectDeclarations(program)) {
					if sameSourceSpan(original.Selection, origin) {
						info.Kind = original.Kind
						info.Detail = strings.Replace(original.Detail, original.Name, alias, 1)
						break
					}
				}
			}
			return info, true
		}
	}
	return declarationInfo{}, false
}
