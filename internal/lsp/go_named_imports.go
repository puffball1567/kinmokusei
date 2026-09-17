package lsp

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func namedGoImportDeclaration(program *ast.Program, target source.Span) bool {
	for _, imported := range program.Imports {
		if !imported.Go {
			continue
		}
		for _, span := range imported.NameSpans {
			if sameSourceSpan(span, target) {
				return true
			}
		}
	}
	return false
}
