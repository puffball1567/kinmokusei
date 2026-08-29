package lsp

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/compiler"
	"ontama.local/ontama/internal/source"
)

type completionItem struct {
	Label    string `json:"label"`
	Kind     int    `json:"kind,omitempty"`
	Detail   string `json:"detail,omitempty"`
	SortText string `json:"sortText,omitempty"`
}

func (s *Server) completion(id json.RawMessage, raw json.RawMessage) error {
	doc, offset, ok := s.navigationParams(raw)
	if !ok || insideCommentOrString(doc.Text, offset) {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: []completionItem{}})
	}
	qualifier, prefix, member := completionContext(doc.Text, offset)
	overlay := make(map[string]string, len(s.documents))
	for _, open := range s.documents {
		overlay[open.Path] = open.Text
	}
	if member {
		overlay[doc.Path] = memberCompletionAnalysisText(doc.Text, offset, prefix)
	}
	result, err := compiler.CheckFilesWithOverlay([]string{doc.Path}, overlay)
	if err != nil || result.Program == nil {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: []completionItem{}})
	}
	var items []completionItem
	if member {
		items = goMemberCompletions(result, doc.Path, qualifier, prefix)
		if !isGoPackageQualifier(result.Program, doc.Path, qualifier) {
			if receiver, ok := completionReceiverAt(result.Program, doc.Path, offset, qualifier); ok && goCompletionType(receiver.typeRef) {
				items = goValueMemberCompletions(result, result.Program, doc.Path, receiver.typeRef, prefix)
			} else {
				items = sourceMemberCompletions(result.Program, doc.Path, offset, qualifier, prefix)
			}
		}
	} else {
		items = lexicalCompletions(result.Program, doc.Path, offset, prefix)
	}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: items})
}

func memberCompletionAnalysisText(value string, offset int, prefix string) string {
	start := offset - len(prefix) - 1
	if start < 0 || offset > len(value) || value[start] != '.' {
		return value
	}
	result := []byte(value)
	for index := start; index < offset; index++ {
		if result[index] != '\r' && result[index] != '\n' {
			result[index] = ' '
		}
	}
	return string(result)
}

func goCompletionType(ref ast.TypeRef) bool {
	_, _, ok := goCompletionTypeInfo(ref)
	return ok
}

func goCompletionTypeInfo(ref ast.TypeRef) (ast.TypeRef, bool, bool) {
	pointer := false
	if ref.Nullable {
		ref.Nullable = false
	}
	if ref.IsPointer() && ref.Pointee != nil {
		pointer = true
		ref = *ref.Pointee
	}
	return ref, pointer, ref.Go && ref.Qualifier != "" && ref.Name != ""
}

func goValueMemberCompletions(result compiler.Result, program *ast.Program, path string, ref ast.TypeRef, prefix string) []completionItem {
	ref, pointer, ok := goCompletionTypeInfo(ref)
	if !ok {
		return []completionItem{}
	}
	importPath := goImportPathForQualifier(program, path, ref.Qualifier)
	if importPath == "" {
		return []completionItem{}
	}
	members, found, err := result.GoTypeMembers(importPath, ref.Name, pointer, true)
	if err != nil || !found {
		return []completionItem{}
	}
	items := make([]completionItem, 0, len(members))
	for _, member := range members {
		if !strings.HasPrefix(member.Name, prefix) {
			continue
		}
		kind := 5
		if member.Kind == "method" {
			kind = 2
		}
		items = append(items, completionItem{Label: member.Name, Kind: kind, Detail: member.Detail, SortText: "0_" + member.Name})
	}
	sortCompletionItems(items)
	return items
}

func goImportPathForQualifier(program *ast.Program, path, qualifier string) string {
	for _, imported := range program.Imports {
		if imported.Go && imported.Alias == qualifier && samePath(imported.Span.Path, path) {
			return imported.Path
		}
	}
	return ""
}

func isGoPackageQualifier(program *ast.Program, path, qualifier string) bool {
	for _, imported := range program.Imports {
		if imported.Go && imported.Alias == qualifier && samePath(imported.Span.Path, path) {
			return true
		}
	}
	return false
}

func goMemberCompletions(result compiler.Result, path, qualifier, prefix string) []completionItem {
	importPath := ""
	for _, imported := range result.Program.Imports {
		if imported.Go && imported.Alias == qualifier && samePath(imported.Span.Path, path) {
			importPath = imported.Path
			break
		}
	}
	if importPath == "" {
		return []completionItem{}
	}
	exports, err := result.GoPackageExports(importPath)
	if err != nil {
		return []completionItem{}
	}
	items := make([]completionItem, 0, len(exports))
	for _, exported := range exports {
		if !strings.HasPrefix(exported.Name, prefix) {
			continue
		}
		kind := 12
		switch exported.Kind {
		case "function":
			kind = 3
		case "constant":
			kind = 21
		case "variable":
			kind = 6
		case "type":
			kind = 7
		}
		items = append(items, completionItem{Label: exported.Name, Kind: kind, Detail: exported.Detail, SortText: exported.Name})
	}
	sortCompletionItems(items)
	return items
}

func lexicalCompletions(program *ast.Program, path string, offset int, prefix string) []completionItem {
	candidates := map[string]completionItem{}
	add := func(item completionItem) {
		if item.Label != "" && strings.HasPrefix(item.Label, prefix) {
			candidates[item.Label] = item
		}
	}
	for _, keyword := range []string{"alias", "await", "break", "case", "catch", "class", "const", "continue", "default", "defer", "detach", "distinct", "else", "extends", "final", "finally", "for", "function", "go", "if", "implements", "import", "interface", "let", "new", "nil", "null", "override", "pointer", "private", "protected", "public", "return", "select", "static", "struct", "super", "switch", "throw", "try", "type", "virtual", "while"} {
		add(completionItem{Label: keyword, Kind: 14, Detail: "keyword", SortText: "3_" + keyword})
	}
	for _, name := range []string{"void", "boolean", "string", "int", "int32", "int64", "uint16", "uint32", "uint64", "float32", "float", "number", "float64", "byte", "error", "Exception", "Map", "Result", "Task", "GoChannel", "GoSendChannel", "GoReceiveChannel"} {
		add(completionItem{Label: name, Kind: 7, Detail: "built-in type", SortText: "2_" + name})
	}
	for _, name := range []string{"len", "cap", "append", "copy", "delete", "clear", "min", "max", "makeSlice", "makeMap", "copyArray", "viewArray", "goChannel", "closeGoChannel", "ok", "fail"} {
		add(completionItem{Label: name, Kind: 3, Detail: "compiler built-in", SortText: "2_" + name})
	}
	for _, imported := range program.Imports {
		if !samePath(imported.Span.Path, path) {
			continue
		}
		if imported.Go {
			add(completionItem{Label: imported.Alias, Kind: 9, Detail: "Go package " + imported.Path, SortText: "1_" + imported.Alias})
			continue
		}
		for _, name := range imported.Names {
			add(completionItem{Label: name, Kind: 9, Detail: "imported from " + imported.Path, SortText: "1_" + name})
		}
	}
	for _, declaration := range program.Declarations {
		if !samePath(declaration.GetSpan().Path, path) {
			continue
		}
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			add(completionItem{Label: declaration.Name, Kind: 3, Detail: functionDeclarationDetail(declaration), SortText: "1_" + declaration.Name})
		case *ast.ClassDecl:
			add(completionItem{Label: declaration.Name, Kind: 7, Detail: "class " + declaration.Name, SortText: "1_" + declaration.Name})
		case *ast.StructDecl:
			detail := "struct " + declaration.Name
			if len(declaration.TypeParameters) != 0 {
				names := make([]string, len(declaration.TypeParameters))
				for index, parameter := range declaration.TypeParameters {
					names[index] = parameter.Name
				}
				detail += "<" + strings.Join(names, ", ") + ">"
			}
			add(completionItem{Label: declaration.Name, Kind: 22, Detail: detail, SortText: "1_" + declaration.Name})
		case *ast.TypeDecl:
			detail := "type " + declaration.Name + " = distinct " + formatTypeRef(declaration.Underlying)
			if declaration.Alias {
				detail = "alias " + declaration.Name + " = " + formatTypeRef(declaration.Underlying)
			}
			add(completionItem{Label: declaration.Name, Kind: 7, Detail: detail, SortText: "1_" + declaration.Name})
		case *ast.InterfaceDecl:
			add(completionItem{Label: declaration.Name, Kind: 8, Detail: "interface " + declaration.Name, SortText: "1_" + declaration.Name})
		case *ast.VariableDecl:
			add(variableCompletion(declaration.Name, declaration.Type, declaration.Constant))
		}
	}
	addLocalCompletions(program, path, offset, add)
	items := make([]completionItem, 0, len(candidates))
	for _, item := range candidates {
		items = append(items, item)
	}
	sortCompletionItems(items)
	return items
}

func addLocalCompletions(program *ast.Program, path string, offset int, add func(completionItem)) {
	for _, declaration := range program.Declarations {
		if !spanContains(declaration.GetSpan(), path, offset) {
			continue
		}
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			for _, parameter := range declaration.TypeParameters {
				add(completionItem{Label: parameter.Name, Kind: 25, Detail: "type parameter " + parameter.Name, SortText: "0_" + parameter.Name})
			}
			addParameters(declaration.Parameters, add)
			addVisibleBlock(declaration.Body, path, offset, add)
		case *ast.MethodDecl:
			add(completionItem{Label: declaration.ReceiverName, Kind: 6, Detail: formatTypeRef(declaration.ReceiverType), SortText: "0_" + declaration.ReceiverName})
			addParameters(declaration.Parameters, add)
			addVisibleBlock(declaration.Body, path, offset, add)
		case *ast.ClassDecl:
			for _, method := range declaration.Methods {
				if spanContains(method.Span, path, offset) {
					if !method.Static {
						add(completionItem{Label: "this", Kind: 6, Detail: declaration.Name, SortText: "0_this"})
					}
					addParameters(method.Parameters, add)
					addVisibleBlock(method.Body, path, offset, add)
					return
				}
			}
			if declaration.Constructor != nil && spanContains(declaration.Constructor.Span, path, offset) {
				add(completionItem{Label: "this", Kind: 6, Detail: declaration.Name, SortText: "0_this"})
				addParameters(declaration.Constructor.Parameters, add)
				addVisibleBlock(declaration.Constructor.Body, path, offset, add)
			}
		case *ast.StructDecl:
			for _, parameter := range declaration.TypeParameters {
				add(completionItem{Label: parameter.Name, Kind: 25, Detail: "type parameter " + parameter.Name, SortText: "0_" + parameter.Name})
			}
			for _, method := range declaration.Methods {
				if spanContains(method.Span, path, offset) {
					detail := declaration.Name
					if method.PointerReceiver {
						detail = "*" + detail
					}
					add(completionItem{Label: "this", Kind: 6, Detail: detail, SortText: "0_this"})
					addParameters(method.Parameters, add)
					addVisibleBlock(method.Body, path, offset, add)
					return
				}
			}
		}
		return
	}
}

func addParameters(parameters []ast.Parameter, add func(completionItem)) {
	for _, parameter := range parameters {
		add(completionItem{Label: parameter.Name, Kind: 6, Detail: parameter.Name + ": " + formatTypeRef(parameter.Type), SortText: "0_" + parameter.Name})
	}
}

func addVisibleBlock(block *ast.BlockStmt, path string, offset int, add func(completionItem)) {
	if block == nil || !spanContains(block.Span, path, offset) {
		return
	}
	for _, statement := range block.Statements {
		span := statement.GetSpan()
		if span.Start.Offset >= offset {
			return
		}
		if span.End.Offset <= offset {
			addStatementBindings(statement, add)
			continue
		}
		switch statement := statement.(type) {
		case *ast.BlockStmt:
			addVisibleBlock(statement, path, offset, add)
		case *ast.IfStmt:
			if spanContains(statement.Then.Span, path, offset) {
				addVisibleBlock(statement.Then, path, offset, add)
			} else if branch, ok := statement.Else.(*ast.BlockStmt); ok && spanContains(branch.Span, path, offset) {
				addVisibleBlock(branch, path, offset, add)
			}
		case *ast.TryStmt:
			if spanContains(statement.Body.Span, path, offset) {
				addVisibleBlock(statement.Body, path, offset, add)
				break
			}
			matched := false
			for _, clause := range statement.Catches {
				if spanContains(clause.Body.Span, path, offset) {
					if clause.Name != "_" {
						add(variableCompletion(clause.Name, clause.Type, true))
					}
					addVisibleBlock(clause.Body, path, offset, add)
					matched = true
					break
				}
			}
			if !matched && statement.FinallyBody != nil && spanContains(statement.FinallyBody.Span, path, offset) {
				addVisibleBlock(statement.FinallyBody, path, offset, add)
			}
		case *ast.WhileStmt:
			addVisibleBlock(statement.Body, path, offset, add)
		case *ast.ForStmt:
			if statement.Initializer != nil && statement.Initializer.GetSpan().End.Offset <= offset {
				addStatementBindings(statement.Initializer, add)
			}
			addVisibleBlock(statement.Body, path, offset, add)
		case *ast.ForRangeStmt:
			if spanContains(statement.Body.Span, path, offset) {
				for _, binding := range statement.Bindings {
					if binding.Name != "_" {
						add(variableCompletion(binding.Name, binding.Type, statement.Constant))
					}
				}
			}
			addVisibleBlock(statement.Body, path, offset, add)
		case *ast.SelectStmt:
			for index := range statement.Cases {
				clause := &statement.Cases[index]
				if !spanContains(clause.Body.Span, path, offset) {
					continue
				}
				if clause.Declare {
					for _, binding := range clause.Bindings {
						if binding.Name != "_" {
							add(completionItem{Label: binding.Name, Kind: 6, Detail: "select binding", SortText: "0_" + binding.Name})
						}
					}
				}
				addVisibleBlock(clause.Body, path, offset, add)
			}
		case *ast.ValueSwitchStmt:
			for index := range statement.Cases {
				clause := &statement.Cases[index]
				if spanContains(clause.Body.Span, path, offset) {
					addVisibleBlock(clause.Body, path, offset, add)
				}
			}
		case *ast.TypeSwitchStmt:
			for index := range statement.Cases {
				clause := &statement.Cases[index]
				if !spanContains(clause.Body.Span, path, offset) {
					continue
				}
				if !clause.Nil && !clause.Default && clause.Name != "_" {
					add(variableCompletion(clause.Name, clause.Type, clause.Constant))
				}
				addVisibleBlock(clause.Body, path, offset, add)
			}
		}
		return
	}
}

func addStatementBindings(statement ast.Statement, add func(completionItem)) {
	switch statement := statement.(type) {
	case *ast.VariableDecl:
		add(variableCompletion(statement.Name, statement.Type, statement.Constant))
	case *ast.MultiVariableDecl:
		for _, binding := range statement.Bindings {
			if binding.Name != "_" {
				add(completionItem{Label: binding.Name, Kind: 6, Detail: "local binding", SortText: "0_" + binding.Name})
			}
		}
	}
}

func variableCompletion(name string, ref ast.TypeRef, constant bool) completionItem {
	detail := "let " + name
	kind := 6
	if constant {
		detail = "const " + name
		kind = 21
	}
	if ref.IsSpecified() {
		detail += ": " + formatTypeRef(ref)
	}
	return completionItem{Label: name, Kind: kind, Detail: detail, SortText: "0_" + name}
}

func spanContains(span source.Span, path string, offset int) bool {
	return samePath(span.Path, path) && span.Start.Offset <= offset && offset <= span.End.Offset
}

func completionContext(text string, offset int) (qualifier, prefix string, member bool) {
	if offset < 0 || offset > len(text) {
		return "", "", false
	}
	start := identifierStart(text, offset)
	prefix = text[start:offset]
	if start == 0 || text[start-1] != '.' {
		return "", prefix, false
	}
	qualifierEnd := start - 1
	qualifierStart := identifierStart(text, qualifierEnd)
	if qualifierStart == qualifierEnd {
		return "", prefix, true
	}
	return text[qualifierStart:qualifierEnd], prefix, true
}

func identifierStart(text string, end int) int {
	start := end
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		start -= size
	}
	return start
}

func insideCommentOrString(text string, offset int) bool {
	if offset < 0 || offset > len(text) {
		return true
	}
	inString, escaped, lineComment, blockComment := false, false, false, false
	for index := 0; index < offset; index++ {
		current := text[index]
		next := byte(0)
		if index+1 < offset {
			next = text[index+1]
		}
		if lineComment {
			if current == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if current == '*' && next == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
		} else if current == '/' && next == '/' {
			lineComment = true
			index++
		} else if current == '/' && next == '*' {
			blockComment = true
			index++
		}
	}
	return inString || lineComment || blockComment
}

func sortCompletionItems(items []completionItem) {
	sort.Slice(items, func(left, right int) bool {
		if items[left].SortText != items[right].SortText {
			return items[left].SortText < items[right].SortText
		}
		return items[left].Label < items[right].Label
	})
}
