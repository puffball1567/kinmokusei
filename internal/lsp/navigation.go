package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/compiler"
	"ontama.local/ontama/internal/lexer"
	"ontama.local/ontama/internal/source"
	"ontama.local/ontama/internal/token"
)

type textDocumentPositionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position position `json:"position"`
}

type declarationInfo struct {
	Name      string
	Detail    string
	Kind      int
	Span      source.Span
	Selection source.Span
	Children  []declarationInfo
}

type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          protocolRange    `json:"range"`
	SelectionRange protocolRange    `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}

func (s *Server) hover(id json.RawMessage, raw json.RawMessage) error {
	doc, offset, ok := s.navigationParams(raw)
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	program := s.analyze(doc)
	if detail, builtin := builtinExceptionHover(program, doc, offset); builtin {
		result := map[string]any{
			"contents": map[string]any{"kind": "markdown", "value": "```ontama\n" + detail + "\n```"},
			"range":    s.protocolRangeFor(doc.Path, identifierSpanAt(doc, offset, identifierAt(doc, offset))),
		}
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: result})
	}
	declaration, found := s.declarationAtProgram(doc, offset, program)
	if !found {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	result := map[string]any{
		"contents": map[string]any{"kind": "markdown", "value": "```ontama\n" + declaration.Detail + "\n```"},
		"range":    s.protocolRangeFor(doc.Path, identifierSpanAt(doc, offset, identifierAt(doc, offset))),
	}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: result})
}

func builtinExceptionHover(program *ast.Program, doc document, offset int) (string, bool) {
	name := identifierAt(doc, offset)
	if name == "Exception" {
		return "class Exception {\n  public message: string;\n  public function error(): string;\n}", true
	}
	var detail string
	visitProgramExpressions(program, func(expression ast.Expression) {
		if detail != "" {
			return
		}
		member, ok := expression.(*ast.MemberExpr)
		if !ok || !samePath(member.NameSpan.Path, doc.Path) || offset < member.NameSpan.Start.Offset || offset > member.NameSpan.End.Offset {
			return
		}
		// Built-in members deliberately have no source declaration. User-defined
		// members with the same generated Go name always carry a declaration span.
		if member.ResolvedDeclaration.Path != "" {
			return
		}
		switch member.ResolvedName {
		case "Message":
			detail = "public message: string"
		case "Error":
			detail = "public function error(): string"
		}
	})
	return detail, detail != ""
}

func (s *Server) definition(id json.RawMessage, raw json.RawMessage) error {
	doc, offset, ok := s.navigationParams(raw)
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	program := s.analyze(doc)
	if _, builtin := builtinExceptionHover(program, doc, offset); builtin {
		// Compiler-provided declarations have no source file to navigate to.
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	declaration, found := s.declarationAtProgram(doc, offset, program)
	if !found {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	absolute, err := filepath.Abs(declaration.Selection.Path)
	if err != nil {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	result := map[string]any{"uri": pathURI(filepath.Clean(absolute)), "range": s.protocolRangeFor(declaration.Selection.Path, declaration.Selection)}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) documentSymbols(id json.RawMessage, raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: []documentSymbol{}})
	}
	doc, ok := s.documents[params.TextDocument.URI]
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: []documentSymbol{}})
	}
	declarations := collectDeclarations(s.analyze(doc))
	result := make([]documentSymbol, 0, len(declarations))
	for _, declaration := range declarations {
		if samePath(declaration.Span.Path, doc.Path) {
			result = append(result, s.toDocumentSymbol(declaration))
		}
	}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) toDocumentSymbol(declaration declarationInfo) documentSymbol {
	result := documentSymbol{
		Name: declaration.Name, Detail: declaration.Detail, Kind: declaration.Kind,
		Range:          s.protocolRangeFor(declaration.Span.Path, declaration.Span),
		SelectionRange: s.protocolRangeFor(declaration.Selection.Path, declaration.Selection),
	}
	for _, child := range declaration.Children {
		result.Children = append(result.Children, s.toDocumentSymbol(child))
	}
	return result
}

func (s *Server) navigationParams(raw json.RawMessage) (document, int, bool) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return document{}, 0, false
	}
	doc, ok := s.documents[params.TextDocument.URI]
	if !ok {
		return document{}, 0, false
	}
	offset, err := byteOffsetAtPosition(doc.Text, params.Position)
	if err != nil {
		return document{}, 0, false
	}
	return doc, offset, true
}

func (s *Server) declarationAt(doc document, offset int) (declarationInfo, bool) {
	return s.declarationAtProgram(doc, offset, s.analyze(doc))
}

func (s *Server) declarationAtProgram(doc document, offset int, program *ast.Program) (declarationInfo, bool) {
	name := identifierAt(doc, offset)
	if name == "" {
		return declarationInfo{}, false
	}
	if occurrence, ok := occurrenceAt(s.symbolOccurrences(program), doc.Path, offset); ok {
		for _, declaration := range flattenDeclarations(collectDeclarations(program)) {
			if sameSourceSpan(declaration.Selection, occurrence.Declaration) {
				if displayName := s.sourceText(declaration.Selection); displayName != "" && displayName != declaration.Name {
					declaration.Detail = strings.Replace(declaration.Detail, declaration.Name, displayName, 1)
					declaration.Name = displayName
				}
				return declaration, true
			}
		}
		for _, imported := range program.Imports {
			if imported.Go && sameSourceSpan(imported.AliasSpan, occurrence.Declaration) {
				return declarationInfo{
					Name: imported.Alias, Detail: fmt.Sprintf("import go %s from %q", imported.Alias, imported.Path), Kind: 2,
					Span: imported.Span, Selection: imported.AliasSpan,
				}, true
			}
		}
	}
	var candidates []declarationInfo
	for _, declaration := range flattenDeclarations(collectDeclarations(program)) {
		if declaration.Name == name {
			candidates = append(candidates, declaration)
		}
	}
	if len(candidates) == 0 {
		for _, imported := range program.Imports {
			if imported.Go && imported.Alias == name {
				return declarationInfo{Name: name, Detail: fmt.Sprintf("import go %s from %q", name, imported.Path), Kind: 2, Span: imported.Span, Selection: imported.AliasSpan}, true
			}
		}
		return declarationInfo{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		leftSame := samePath(candidates[i].Selection.Path, doc.Path)
		rightSame := samePath(candidates[j].Selection.Path, doc.Path)
		if leftSame != rightSame {
			return leftSame
		}
		leftBefore := candidates[i].Selection.Start.Offset <= offset
		rightBefore := candidates[j].Selection.Start.Offset <= offset
		if leftBefore != rightBefore {
			return leftBefore
		}
		return candidates[i].Selection.Start.Offset > candidates[j].Selection.Start.Offset
	})
	return candidates[0], true
}

func (s *Server) analyze(doc document) *ast.Program {
	overlay := make(map[string]string, len(s.documents))
	for _, open := range s.documents {
		overlay[open.Path] = open.Text
	}
	result, err := compiler.CheckFilesWithOverlay([]string{doc.Path}, overlay)
	if err != nil || result.Program == nil {
		return &ast.Program{}
	}
	return result.Program
}

func collectDeclarations(program *ast.Program) []declarationInfo {
	var result []declarationInfo
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			info := declarationInfo{Name: declaration.Name, Detail: functionDeclarationDetail(declaration), Kind: 12, Span: declaration.Span, Selection: declaration.NameSpan}
			for _, parameter := range declaration.TypeParameters {
				info.Children = append(info.Children, declarationInfo{Name: parameter.Name, Detail: "type parameter " + parameter.Name, Kind: 26, Span: parameter.Span, Selection: parameter.NameSpan})
			}
			for _, parameter := range declaration.Parameters {
				info.Children = append(info.Children, declarationInfo{Name: parameter.Name, Detail: parameter.Name + ": " + formatTypeRef(parameter.Type), Kind: 13, Span: parameter.Span, Selection: nameSpan(parameter.Span, parameter.Name)})
			}
			collectBlockDeclarations(declaration.Body, &info.Children)
			result = append(result, info)
		case *ast.MethodDecl:
			info := declarationInfo{Name: declaration.Name, Detail: functionDetail(declaration.Name, declaration.Parameters, declaration.ReturnType), Kind: 6, Span: declaration.Span, Selection: declaration.NameSpan}
			info.Children = append(info.Children, declarationInfo{Name: declaration.ReceiverName, Detail: declaration.ReceiverName + ": " + formatTypeRef(declaration.ReceiverType), Kind: 13, Span: declaration.ReceiverNameSpan, Selection: declaration.ReceiverNameSpan})
			for _, parameter := range declaration.Parameters {
				info.Children = append(info.Children, declarationInfo{Name: parameter.Name, Detail: parameter.Name + ": " + formatTypeRef(parameter.Type), Kind: 13, Span: parameter.Span, Selection: nameSpan(parameter.Span, parameter.Name)})
			}
			collectBlockDeclarations(declaration.Body, &info.Children)
			result = append(result, info)
		case *ast.ClassDecl:
			info := declarationInfo{Name: declaration.Name, Detail: "class " + declaration.Name, Kind: 5, Span: declaration.Span, Selection: declaration.NameSpan}
			for _, field := range declaration.Fields {
				info.Children = append(info.Children, declarationInfo{Name: field.Name, Detail: field.Name + ": " + formatTypeRef(field.Type), Kind: 8, Span: field.Span, Selection: field.NameSpan})
			}
			for _, method := range declaration.Methods {
				child := declarationInfo{Name: method.Name, Detail: functionDetail(method.Name, method.Parameters, method.ReturnType), Kind: 6, Span: method.Span, Selection: method.NameSpan}
				collectBlockDeclarations(method.Body, &child.Children)
				info.Children = append(info.Children, child)
			}
			result = append(result, info)
		case *ast.StructDecl:
			detail := "struct " + declaration.Name
			if len(declaration.TypeParameters) != 0 {
				names := make([]string, len(declaration.TypeParameters))
				for index, parameter := range declaration.TypeParameters {
					names[index] = parameter.Name
				}
				detail += "<" + strings.Join(names, ", ") + ">"
			}
			info := declarationInfo{Name: declaration.Name, Detail: detail, Kind: 23, Span: declaration.Span, Selection: declaration.NameSpan}
			for _, parameter := range declaration.TypeParameters {
				info.Children = append(info.Children, declarationInfo{Name: parameter.Name, Detail: "type parameter " + parameter.Name, Kind: 26, Span: parameter.Span, Selection: parameter.NameSpan})
			}
			for _, field := range declaration.Fields {
				info.Children = append(info.Children, declarationInfo{Name: field.Name, Detail: field.Name + ": " + formatTypeRef(field.Type), Kind: 8, Span: field.Span, Selection: field.NameSpan})
			}
			for _, method := range declaration.Methods {
				child := declarationInfo{Name: method.Name, Detail: functionDetail(method.Name, method.Parameters, method.ReturnType), Kind: 6, Span: method.Span, Selection: method.NameSpan}
				collectBlockDeclarations(method.Body, &child.Children)
				info.Children = append(info.Children, child)
			}
			result = append(result, info)
		case *ast.TypeDecl:
			prefix := "type " + declaration.Name + " = distinct "
			if declaration.Alias {
				prefix = "alias " + declaration.Name + " = "
			}
			result = append(result, declarationInfo{
				Name: declaration.Name, Detail: prefix + formatTypeRef(declaration.Underlying),
				Kind: 5, Span: declaration.Span, Selection: declaration.NameSpan,
			})
		case *ast.InterfaceDecl:
			info := declarationInfo{Name: declaration.Name, Detail: "interface " + declaration.Name, Kind: 11, Span: declaration.Span, Selection: declaration.NameSpan}
			for _, method := range declaration.Methods {
				info.Children = append(info.Children, declarationInfo{Name: method.Name, Detail: functionDetail(method.Name, method.Parameters, method.ReturnType), Kind: 6, Span: method.Span, Selection: method.NameSpan})
			}
			result = append(result, info)
		case *ast.VariableDecl:
			kind, prefix := 13, "let "
			if declaration.Constant {
				kind, prefix = 14, "const "
			}
			detail := prefix + declaration.Name
			if declaration.Type.IsSpecified() {
				detail += ": " + formatTypeRef(declaration.Type)
			}
			result = append(result, declarationInfo{Name: declaration.Name, Detail: detail, Kind: kind, Span: declaration.Span, Selection: declaration.NameSpan})
		}
	}
	return result
}

func collectBlockDeclarations(block *ast.BlockStmt, result *[]declarationInfo) {
	if block == nil {
		return
	}
	for _, statement := range block.Statements {
		switch statement := statement.(type) {
		case *ast.VariableDecl:
			kind, prefix := 13, "let "
			if statement.Constant {
				kind, prefix = 14, "const "
			}
			detail := prefix + statement.Name
			if statement.Type.IsSpecified() {
				detail += ": " + formatTypeRef(statement.Type)
			}
			*result = append(*result, declarationInfo{Name: statement.Name, Detail: detail, Kind: kind, Span: statement.Span, Selection: statement.NameSpan})
		case *ast.MultiVariableDecl:
			for _, binding := range statement.Bindings {
				if binding.Name != "_" {
					*result = append(*result, declarationInfo{Name: binding.Name, Detail: binding.Name, Kind: 13, Span: binding.Span, Selection: binding.Span})
				}
			}
		case *ast.BlockStmt:
			collectBlockDeclarations(statement, result)
		case *ast.IfStmt:
			collectBlockDeclarations(statement.Then, result)
			if branch, ok := statement.Else.(*ast.BlockStmt); ok {
				collectBlockDeclarations(branch, result)
			}
		case *ast.TryStmt:
			collectBlockDeclarations(statement.Body, result)
			for _, clause := range statement.Catches {
				if clause.Name != "_" {
					*result = append(*result, declarationInfo{Name: clause.Name, Detail: "catch " + clause.Name + ": " + formatTypeRef(clause.Type), Kind: 13, Span: clause.Body.Span, Selection: clause.NameSpan})
				}
				collectBlockDeclarations(clause.Body, result)
			}
			collectBlockDeclarations(statement.FinallyBody, result)
		case *ast.WhileStmt:
			collectBlockDeclarations(statement.Body, result)
		case *ast.ForStmt:
			if variable, ok := statement.Initializer.(*ast.VariableDecl); ok {
				collectBlockDeclarations(&ast.BlockStmt{Statements: []ast.Statement{variable}}, result)
			}
			collectBlockDeclarations(statement.Body, result)
		case *ast.ForRangeStmt:
			for _, binding := range statement.Bindings {
				if binding.Name == "_" {
					continue
				}
				detail := "const " + binding.Name
				if !statement.Constant {
					detail = "let " + binding.Name
				}
				if binding.Type.IsSpecified() {
					detail += ": " + formatTypeRef(binding.Type)
				}
				*result = append(*result, declarationInfo{Name: binding.Name, Detail: detail, Kind: 13, Span: statement.Span, Selection: binding.NameSpan})
			}
			collectBlockDeclarations(statement.Body, result)
		case *ast.SelectStmt:
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				if clause.Declare {
					for _, binding := range clause.Bindings {
						if binding.Name == "_" {
							continue
						}
						prefix := "const "
						if !clause.Constant {
							prefix = "let "
						}
						*result = append(*result, declarationInfo{Name: binding.Name, Detail: prefix + binding.Name, Kind: 13, Span: clause.Span, Selection: binding.Span})
					}
				}
				collectBlockDeclarations(clause.Body, result)
			}
		case *ast.ValueSwitchStmt:
			for i := range statement.Cases {
				collectBlockDeclarations(statement.Cases[i].Body, result)
			}
		case *ast.TypeSwitchStmt:
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				if !clause.Nil && !clause.Default && clause.Name != "_" {
					prefix := "const "
					if !clause.Constant {
						prefix = "let "
					}
					detail := prefix + clause.Name + " as " + formatTypeRef(clause.Type)
					*result = append(*result, declarationInfo{Name: clause.Name, Detail: detail, Kind: 13, Span: clause.Span, Selection: clause.NameSpan})
				}
				collectBlockDeclarations(clause.Body, result)
			}
		case *ast.CallControlStmt:
			// Calls do not introduce declarations into the surrounding scope.
		case *ast.DetachStmt:
			// Detaching a task does not introduce declarations.
		case *ast.ChannelSendStmt:
			// Channel sends do not introduce declarations.
		}
	}
}

func flattenDeclarations(declarations []declarationInfo) []declarationInfo {
	var result []declarationInfo
	var visit func([]declarationInfo)
	visit = func(items []declarationInfo) {
		for _, item := range items {
			result = append(result, item)
			visit(item.Children)
		}
	}
	visit(declarations)
	return result
}

func functionDetail(name string, parameters []ast.Parameter, result ast.TypeRef) string {
	items := make([]string, len(parameters))
	for i, parameter := range parameters {
		items[i] = parameter.Name + ": " + formatTypeRef(parameter.Type)
	}
	return "function " + name + "(" + strings.Join(items, ", ") + "): " + formatTypeRef(result)
}

func functionDeclarationDetail(function *ast.FunctionDecl) string {
	name := function.Name
	if len(function.TypeParameters) != 0 {
		parameters := make([]string, len(function.TypeParameters))
		for index, parameter := range function.TypeParameters {
			parameters[index] = parameter.Name
		}
		name += "<" + strings.Join(parameters, ", ") + ">"
	}
	return functionDetail(name, function.Parameters, function.ReturnType)
}

func formatTypeRef(ref ast.TypeRef) string {
	if ref.Nullable {
		ref.Nullable = false
		return formatTypeRef(ref) + " | null"
	}
	if ref.IsPointer() {
		return "*" + formatTypeRef(*ref.Pointee)
	}
	if ref.IsArray() {
		if ref.IsFixedArray() {
			return fmt.Sprintf("[%d]%s", *ref.FixedLength, formatTypeRef(*ref.Element))
		}
		return formatTypeRef(*ref.Element) + "[]"
	}
	if ref.IsFunction() {
		parameters := make([]string, len(ref.Parameters))
		for i := range ref.Parameters {
			parameters[i] = formatTypeRef(ref.Parameters[i])
		}
		return "(" + strings.Join(parameters, ", ") + ") => " + formatTypeRef(*ref.Return)
	}
	if ref.IsObject() {
		fields := make([]string, len(ref.ObjectFields))
		for index, field := range ref.ObjectFields {
			name := field.Name
			if field.JSONName != "" {
				name = field.JSONName
			}
			fields[index] = name + ": " + formatTypeRef(field.Type)
		}
		return "{ " + strings.Join(fields, ", ") + " }"
	}
	name := ref.Name
	if ref.Qualifier != "" {
		name = ref.Qualifier + "." + name
	}
	if len(ref.GenericArguments) != 0 {
		arguments := make([]string, len(ref.GenericArguments))
		for i := range ref.GenericArguments {
			arguments[i] = formatTypeRef(ref.GenericArguments[i])
		}
		name += "<" + strings.Join(arguments, ", ") + ">"
	}
	if name == "" {
		return "<inferred>"
	}
	return name
}

func identifierAt(doc document, offset int) string {
	tokens, _ := lexer.Lex(doc.Path, doc.Text)
	for _, item := range tokens {
		if item.Kind != token.Identifier && item.Kind != token.This {
			continue
		}
		if item.Span.Start.Offset <= offset && offset <= item.Span.End.Offset {
			return item.Lexeme
		}
	}
	return ""
}

func identifierSpanAt(doc document, offset int, name string) source.Span {
	tokens, _ := lexer.Lex(doc.Path, doc.Text)
	for _, item := range tokens {
		if item.Lexeme == name && item.Span.Start.Offset <= offset && offset <= item.Span.End.Offset {
			return item.Span
		}
	}
	return source.Span{Path: doc.Path}
}

func nameSpan(span source.Span, name string) source.Span {
	span.End.Offset = span.Start.Offset + len(name)
	span.End.Line = span.Start.Line
	span.End.Column = span.Start.Column + utf8.RuneCountInString(name)
	return span
}

func byteOffsetAtPosition(text string, target position) (int, error) {
	if target.Line < 0 || target.Character < 0 {
		return 0, fmt.Errorf("negative position")
	}
	line, character, offset := 0, 0, 0
	for offset < len(text) {
		if line == target.Line && character == target.Character {
			return offset, nil
		}
		r, size := utf8.DecodeRuneInString(text[offset:])
		if r == '\r' {
			if line == target.Line {
				return 0, fmt.Errorf("character is outside line")
			}
			offset += size
			if offset < len(text) && text[offset] == '\n' {
				offset++
			}
			line, character = line+1, 0
			continue
		}
		if r == '\n' {
			if line == target.Line {
				return 0, fmt.Errorf("character is outside line")
			}
			line, character, offset = line+1, 0, offset+size
			continue
		}
		units := len(utf16.Encode([]rune{r}))
		if line == target.Line && character+units > target.Character {
			return 0, fmt.Errorf("character splits a UTF-16 surrogate pair")
		}
		character += units
		offset += size
	}
	if line == target.Line && character == target.Character {
		return offset, nil
	}
	return 0, fmt.Errorf("position is outside document")
}

func utf16Length(text string) int {
	length := 0
	for _, r := range text {
		if r > 0xffff {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func (s *Server) protocolRangeFor(path string, span source.Span) protocolRange {
	text := s.textForPath(path)
	return protocolRange{Start: protocolPosition(text, span.Start), End: protocolPosition(text, span.End)}
}

func (s *Server) textForPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err == nil {
		if doc, ok := s.documentByPath(filepath.Clean(absolute)); ok {
			return doc.Text
		}
	}
	contents, _ := os.ReadFile(path)
	return string(contents)
}

func samePath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbsolute) == filepath.Clean(rightAbsolute)
}
