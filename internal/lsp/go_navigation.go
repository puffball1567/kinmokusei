package lsp

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/compiler"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (s *Server) analyzeNavigation(doc document) compiler.Result {
	overlay := make(map[string]string, len(s.documents))
	for _, open := range s.documents {
		overlay[open.Path] = open.Text
	}
	result, err := s.checkDocument(doc, overlay)
	if err != nil || result.Program == nil {
		return compiler.Result{Program: &ast.Program{}}
	}
	return result
}

func goReferenceAt(program *ast.Program, doc document, offset int) (ast.GoReference, bool) {
	if program == nil || insideCommentOrString(doc.Text, offset) {
		return ast.GoReference{}, false
	}
	var selected source.Span
	var reference ast.GoReference
	found := false
	for span, candidate := range program.GoReferences {
		if !samePath(span.Path, doc.Path) || offset < span.Start.Offset || offset >= span.End.Offset {
			continue
		}
		if !found || span.End.Offset-span.Start.Offset < selected.End.Offset-selected.Start.Offset {
			selected, reference, found = span, candidate, true
		}
	}
	return reference, found
}
