package lsp

import "github.com/puffball1567/kinmokusei/internal/ast"

func addNamedResultDeclarations(ref ast.TypeRef, declarations *[]declarationInfo) {
	for i, binding := range ref.NamedResults {
		if binding.Name != "_" {
			*declarations = append(*declarations, declarationInfo{Name: binding.Name, Detail: binding.Name + ": " + formatTypeRef(ref.NamedResultType(i)), Kind: 13, Span: binding.Span, Selection: binding.Span})
		}
	}
}

func addNamedResultCompletions(ref ast.TypeRef, add func(completionItem)) {
	for i, binding := range ref.NamedResults {
		if binding.Name != "_" {
			add(completionItem{Label: binding.Name, Kind: 6, Detail: binding.Name + ": " + formatTypeRef(ref.NamedResultType(i)), SortText: "0_" + binding.Name})
		}
	}
}

func callableBindingType(parameters []ast.Parameter, result ast.TypeRef, name string) (ast.TypeRef, bool) {
	if ref, ok := parameterType(parameters, name); ok {
		return ref, true
	}
	for i, binding := range result.NamedResults {
		if binding.Name != "_" && binding.Name == name {
			return result.NamedResultType(i), true
		}
	}
	return ast.TypeRef{}, false
}
