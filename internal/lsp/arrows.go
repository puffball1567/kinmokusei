package lsp

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func addExpressionDeclarations(program *ast.Program, declarations []declarationInfo) {
	visitProgramExpressions(program, func(expression ast.Expression) {
		switch expression.(type) {
		case *ast.ArrowExpr, *ast.MatchExpr:
		default:
			return
		}
		for i := range declarations {
			owner := &declarations[i]
			span := expression.GetSpan()
			if !spanContains(owner.Span, span.Path, span.Start.Offset) {
				continue
			}
			switch expression := expression.(type) {
			case *ast.ArrowExpr:
				for _, parameter := range expression.Parameters {
					owner.Children = append(owner.Children, declarationInfo{Name: parameter.Name, Detail: parameter.Name + ": " + formatTypeRef(parameter.Type), Kind: 13, Span: parameter.Span, Selection: nameSpan(parameter.Span, parameter.Name)})
				}
				addNamedResultDeclarations(expression.ResolvedReturnType, &owner.Children)
				collectBlockDeclarations(expression.BlockBody, &owner.Children)
			case *ast.MatchExpr:
				for i := range expression.Arms {
					visitPatternBindings(&expression.Arms[i].Pattern, func(binding ast.Binding) {
						detail := variableCompletion(binding.Name, binding.ResolvedType, true).Detail
						owner.Children = append(owner.Children, declarationInfo{Name: binding.Name, Detail: detail, Kind: 14, Span: binding.Span, Selection: binding.Span})
					})
				}
			}
			break
		}
	})
}

func addExpressionCompletions(program *ast.Program, path string, offset int, add func(completionItem)) {
	// The preorder visitor adds inner bindings last, retaining outer captures.
	visitProgramExpressions(program, func(expression ast.Expression) {
		switch expression := expression.(type) {
		case *ast.ArrowExpr:
			if spanContains(expression.Span, path, offset) {
				addParameters(expression.Parameters, add)
				addNamedResultCompletions(expression.ResolvedReturnType, add)
				addVisibleBlock(expression.BlockBody, path, offset, add)
			}
		case *ast.MatchExpr:
			visitMatchBindingsAt(expression, path, offset, func(binding ast.Binding) {
				add(variableCompletion(binding.Name, binding.ResolvedType, true))
			})
		}
	})
}

func visitMatchBindingsAt(match *ast.MatchExpr, path string, offset int, visit func(ast.Binding)) {
	for i := range match.Arms {
		arm := &match.Arms[i]
		start := arm.Value.GetSpan().Start
		if arm.Guard != nil {
			start = arm.Guard.GetSpan().Start
		}
		if spanContains(source.Span{Path: arm.Span.Path, Start: start, End: arm.Span.End}, path, offset) {
			visitPatternBindings(&arm.Pattern, visit)
			return
		}
	}
}

func visitPatternBindings(root *ast.MatchPattern, visit func(ast.Binding)) {
	root.Walk(func(pattern *ast.MatchPattern) {
		if binding := pattern.Binding; binding.Name != "" && binding.Name != "_" {
			visit(binding)
		}
	})
}
