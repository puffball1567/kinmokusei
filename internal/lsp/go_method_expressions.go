package lsp

import (
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/compiler"
)

// Use semantic identity rather than guessing a type from an identifier's
// spelling. This also keeps namespace/named-import shadowing lexical.
func goMethodExpressionReceiverAt(program *ast.Program, path, text string, dot int) (ast.TypeRef, bool) {
	if dot < 0 || dot > len(text) {
		return ast.TypeRef{}, false
	}
	var receiver ast.TypeRef
	start := dot
	found := false
	for span, ref := range program.GoTypeExpressions {
		if !samePath(span.Path, path) || span.End.Offset > dot || span.Start.Offset >= start {
			continue
		}
		// Parentheses are absent from the checked expression's span. Only
		// trailing closing parentheses/whitespace may separate it from '.';
		// an intervening operation is a different receiver expression.
		if !goMethodExpressionReceiverSuffix(text[span.End.Offset:dot]) {
			continue
		}
		receiver, start, found = ref, span.Start.Offset, true
	}
	return receiver, found
}

func goMethodExpressionReceiverSuffix(suffix string) bool {
	for {
		suffix = strings.TrimLeft(suffix, ") \t\r\n")
		switch {
		case suffix == "":
			return true
		case strings.HasPrefix(suffix, "/*"):
			end := strings.Index(suffix[2:], "*/")
			if end < 0 {
				return false
			}
			suffix = suffix[end+4:]
		case strings.HasPrefix(suffix, "//"):
			end := strings.IndexByte(suffix, '\n')
			if end < 0 {
				return false
			}
			suffix = suffix[end+1:]
		default:
			return false
		}
	}
}

func goMethodExpressionCompletions(result compiler.Result, path string, ref ast.TypeRef, prefix string) []completionItem {
	ref, pointer, ok := goCompletionTypeInfo(ref)
	if !ok {
		return []completionItem{}
	}
	importPath := goImportPathForQualifier(result.Program, path, ref.Qualifier)
	members, found, err := result.GoTypeMembers(importPath, ref.Name, pointer, false)
	if err != nil || !found {
		return []completionItem{}
	}
	items := []completionItem{}
	for _, member := range members {
		if member.Kind != "method" || !strings.HasPrefix(member.Name, prefix) {
			continue
		}
		items = append(items, completionItem{Label: member.Name, Kind: 2, Detail: "method expression: " + member.Detail, SortText: "0_" + member.Name})
	}
	sortCompletionItems(items)
	return items
}
