package lsp

import "github.com/puffball1567/kinmokusei/internal/ast"

func addArrowDeclarations(program *ast.Program, declarations []declarationInfo) {
	visitProgramExpressions(program, func(expression ast.Expression) {
		arrow, ok := expression.(*ast.ArrowExpr)
		if !ok {
			return
		}
		for i := range declarations {
			owner := &declarations[i]
			if !spanContains(owner.Span, arrow.Span.Path, arrow.Span.Start.Offset) {
				continue
			}
			for _, parameter := range arrow.Parameters {
				owner.Children = append(owner.Children, declarationInfo{Name: parameter.Name, Detail: parameter.Name + ": " + formatTypeRef(parameter.Type), Kind: 13, Span: parameter.Span, Selection: nameSpan(parameter.Span, parameter.Name)})
			}
			collectBlockDeclarations(arrow.BlockBody, &owner.Children)
			break
		}
	})
}

func addArrowCompletions(program *ast.Program, path string, offset int, add func(completionItem)) {
	var arrows []*ast.ArrowExpr
	visitProgramExpressions(program, func(expression ast.Expression) {
		if arrow, ok := expression.(*ast.ArrowExpr); ok && spanContains(arrow.Span, path, offset) {
			arrows = append(arrows, arrow)
		}
	})
	// Completion replaces earlier entries, so the preorder visitor adds inner
	// bindings last while retaining unshadowed outer captures.
	for i := 0; i < len(arrows); i++ {
		addParameters(arrows[i].Parameters, add)
		addVisibleBlock(arrows[i].BlockBody, path, offset, add)
	}
}
