package lsp

import (
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func nativeMethodExpressionMembersAt(program *ast.Program, path, text string, dot int) ([]ast.ObjectTypeField, bool) {
	if dot < 0 || dot > len(text) {
		return nil, false
	}
	var members []ast.ObjectTypeField
	start, found := dot, false
	for span, methods := range program.NativeTypeMethodSets {
		if !samePath(span.Path, path) || span.End.Offset > dot || span.Start.Offset >= start || !goMethodExpressionReceiverSuffix(text[span.End.Offset:dot]) {
			continue
		}
		members, start, found = methods, span.Start.Offset, true
	}
	return members, found
}

func nativeMethodExpressionSignature(program *ast.Program, member ast.ObjectTypeField) (ast.CallableSignature, bool) {
	ref := member.Type
	if !ref.IsFunction() || ref.Return == nil || len(ref.Parameters) == 0 {
		return ast.CallableSignature{}, false
	}
	names := []string{"receiver"}
	parameters := parametersAtDeclaration(program, member.Span)
	if len(parameters)+1 == len(ref.Parameters) {
		names = append(names, parameterNames(parameters)...)
	}
	types := make([]string, len(ref.Parameters))
	for index, parameter := range ref.Parameters {
		types[index] = formatTypeRef(parameter)
	}
	return ast.CallableSignature{ParameterNames: names, ParameterTypes: types, Result: formatTypeRef(*ref.Return), Variadic: ref.Variadic}, true
}

func nativeMethodExpressionCompletions(members []ast.ObjectTypeField, prefix string) []completionItem {
	items := []completionItem{}
	for _, member := range members {
		if !strings.HasPrefix(member.Name, prefix) {
			continue
		}
		items = append(items, completionItem{Label: member.Name, Kind: 2, Detail: "method expression: " + formatTypeRef(member.Type), SortText: "0_" + member.Name})
	}
	sortCompletionItems(items)
	return items
}
