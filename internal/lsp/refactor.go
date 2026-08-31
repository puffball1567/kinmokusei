package lsp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/compiler"
	"ontama.local/ontama/internal/lexer"
	"ontama.local/ontama/internal/source"
	"ontama.local/ontama/internal/token"
)

type symbolOccurrence struct {
	Name        string
	Span        source.Span
	Declaration source.Span
}

type location struct {
	URI   string        `json:"uri"`
	Range protocolRange `json:"range"`
}

type textEdit struct {
	Range   protocolRange `json:"range"`
	NewText string        `json:"newText"`
}

func (s *Server) references(id json.RawMessage, raw json.RawMessage) error {
	var params struct {
		textDocumentPositionParams
		Context struct {
			IncludeDeclaration bool `json:"includeDeclaration"`
		} `json:"context"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: []location{}})
	}
	doc, offset, ok := s.navigationParams(raw)
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: []location{}})
	}
	program := s.analyze(doc)
	occurrences := s.symbolOccurrences(program)
	target, ok := occurrenceAt(occurrences, doc.Path, offset)
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: []location{}})
	}
	declarations := relatedDeclarations(program, target.Declaration)
	result := []location{}
	for _, occurrence := range occurrences {
		if !declarations.contains(occurrence.Declaration) {
			continue
		}
		if !params.Context.IncludeDeclaration && sameSourceSpan(occurrence.Span, occurrence.Declaration) {
			continue
		}
		absolute, err := filepath.Abs(occurrence.Span.Path)
		if err != nil {
			continue
		}
		result = append(result, location{URI: pathURI(filepath.Clean(absolute)), Range: s.protocolRangeFor(occurrence.Span.Path, occurrence.Span)})
	}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) prepareRename(id json.RawMessage, raw json.RawMessage) error {
	doc, offset, ok := s.navigationParams(raw)
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	program := s.analyze(doc)
	occurrences := s.symbolOccurrences(program)
	target, ok := occurrenceAt(occurrences, doc.Path, offset)
	if !ok || !hasDeclarationOccurrence(occurrences, target.Declaration) || fixedReceiverDeclaration(program, target.Declaration) {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	result := map[string]any{"range": s.protocolRangeFor(target.Span.Path, target.Span), "placeholder": target.Name}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) rename(id json.RawMessage, raw json.RawMessage) error {
	var params struct {
		textDocumentPositionParams
		NewName string `json:"newName"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return s.renameError(id, "invalid rename parameters")
	}
	doc, offset, ok := s.navigationParams(raw)
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	if !validRenameIdentifier(params.NewName) {
		return s.renameError(id, fmt.Sprintf("%q is not a valid OnsenTamago identifier", params.NewName))
	}

	program, diagnostics, err := s.analyzeForRename(doc, nil)
	if err != nil || len(diagnostics) != 0 {
		return s.renameError(id, "rename requires a program without existing diagnostics")
	}
	occurrences := s.symbolOccurrences(program)
	target, ok := occurrenceAt(occurrences, doc.Path, offset)
	if !ok || !hasDeclarationOccurrence(occurrences, target.Declaration) {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	if fixedReceiverDeclaration(program, target.Declaration) {
		return s.renameError(id, "external method receiver 'this' is fixed syntax and cannot be renamed")
	}
	if params.NewName == target.Name {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: map[string]any{"changes": map[string][]textEdit{}}})
	}
	declarations := relatedDeclarations(program, target.Declaration)
	byPath := map[string][]symbolOccurrence{}
	for _, occurrence := range occurrences {
		if declarations.contains(occurrence.Declaration) {
			path := cleanPath(occurrence.Span.Path)
			byPath[path] = append(byPath[path], occurrence)
		}
	}
	overlay := s.documentOverlay()
	changes := map[string][]textEdit{}
	transformed := map[sourceSpanKey]source.Span{}
	for path, items := range byPath {
		text := s.textForPath(path)
		ascending := append([]symbolOccurrence(nil), items...)
		sort.Slice(ascending, func(i, j int) bool { return ascending[i].Span.Start.Offset < ascending[j].Span.Start.Offset })
		delta := 0
		for _, item := range ascending {
			span := item.Span
			span.Start.Offset += delta
			span.End.Offset = span.Start.Offset + len(params.NewName)
			transformed[spanKey(item.Span)] = span
			delta += len(params.NewName) - (item.Span.End.Offset - item.Span.Start.Offset)
		}
		descending := append([]symbolOccurrence(nil), items...)
		sort.Slice(descending, func(i, j int) bool { return descending[i].Span.Start.Offset > descending[j].Span.Start.Offset })
		for _, item := range descending {
			if item.Span.Start.Offset < 0 || item.Span.End.Offset > len(text) || item.Span.Start.Offset > item.Span.End.Offset {
				return s.renameError(id, "rename encountered a stale source span")
			}
			text = text[:item.Span.Start.Offset] + params.NewName + text[item.Span.End.Offset:]
		}
		overlay[path] = text
		uri := pathURI(path)
		for _, item := range ascending {
			changes[uri] = append(changes[uri], textEdit{Range: s.protocolRangeFor(item.Span.Path, item.Span), NewText: params.NewName})
		}
	}
	renamedProgram, renamedDiagnostics, renamedErr := s.analyzeForRename(doc, overlay)
	if renamedErr != nil {
		return s.renameError(id, fmt.Sprintf("rename validation failed: %v", renamedErr))
	}
	if len(renamedDiagnostics) != 0 {
		return s.renameError(id, "rename would make the program invalid: "+renamedDiagnostics[0])
	}
	for declaration := range declarations {
		if _, ok := transformed[declaration]; !ok {
			return s.renameError(id, "rename could not track a declaration after editing")
		}
	}
	renamedOccurrences := s.symbolOccurrencesWithText(renamedProgram, overlay)
	for _, items := range byPath {
		for _, item := range items {
			renamedSpan := transformed[spanKey(item.Span)]
			renamedDeclaration := transformed[spanKey(item.Declaration)]
			resolved, found := occurrenceAt(renamedOccurrences, renamedSpan.Path, renamedSpan.Start.Offset)
			if !found || !sameSourceSpan(resolved.Span, renamedSpan) || !sameSourceSpan(resolved.Declaration, renamedDeclaration) {
				return s.renameError(id, fmt.Sprintf("rename to %q would change symbol resolution", params.NewName))
			}
		}
	}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: map[string]any{"changes": changes}})
}

type declarationSet map[sourceSpanKey]bool

func fixedReceiverDeclaration(program *ast.Program, target source.Span) bool {
	for _, declaration := range program.Declarations {
		method, ok := declaration.(*ast.MethodDecl)
		if ok && method.External && sameSourceSpan(method.ReceiverNameSpan, target) {
			return true
		}
	}
	return false
}

func (set declarationSet) contains(span source.Span) bool {
	return set[spanKey(span)]
}

func relatedDeclarations(program *ast.Program, target source.Span) declarationSet {
	result := declarationSet{spanKey(target): true}
	interfaces := map[sourceSpanKey]*ast.InterfaceDecl{}
	classes := map[sourceSpanKey]*ast.ClassDecl{}
	for _, declaration := range program.Declarations {
		if contract, ok := declaration.(*ast.InterfaceDecl); ok {
			interfaces[spanKey(contract.NameSpan)] = contract
		}
		if class, ok := declaration.(*ast.ClassDecl); ok {
			classes[spanKey(class.NameSpan)] = class
		}
	}
	changed := true
	for changed {
		changed = false
		for _, declaration := range program.Declarations {
			class, ok := declaration.(*ast.ClassDecl)
			if !ok {
				continue
			}
			for _, method := range class.Methods {
				family := []source.Span{method.NameSpan}
				for baseRef := class.Base; baseRef != nil; {
					base := classes[spanKey(baseRef.ResolvedDeclaration)]
					if base == nil {
						break
					}
					for _, inherited := range base.Methods {
						if inherited.Name == method.Name {
							family = append(family, inherited.NameSpan)
							break
						}
					}
					baseRef = base.Base
				}
				for _, implemented := range class.Implements {
					contract := interfaces[spanKey(implemented.ResolvedDeclaration)]
					if contract == nil {
						continue
					}
					for index := range contract.Methods {
						if contract.Methods[index].Name == method.Name {
							family = append(family, contract.Methods[index].NameSpan)
						}
					}
				}
				connected := false
				for _, member := range family {
					if result.contains(member) {
						connected = true
						break
					}
				}
				if !connected {
					continue
				}
				for _, member := range family {
					key := spanKey(member)
					if !result[key] {
						result[key] = true
						changed = true
					}
				}
			}
		}
	}
	return result
}

func (s *Server) renameError(id json.RawMessage, message string) error {
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Error: &responseError{Code: -32602, Message: message}})
}

func (s *Server) analyzeForRename(doc document, additional map[string]string) (*ast.Program, []string, error) {
	overlay := s.documentOverlay()
	for path, text := range additional {
		overlay[path] = text
	}
	result, err := compiler.CheckFilesWithOverlay([]string{doc.Path}, overlay)
	if err != nil {
		return nil, nil, err
	}
	messages := make([]string, len(result.Diagnostics))
	for index, item := range result.Diagnostics {
		messages[index] = item.Message
	}
	return result.Program, messages, nil
}

func (s *Server) documentOverlay() map[string]string {
	overlay := make(map[string]string, len(s.documents))
	for _, open := range s.documents {
		overlay[cleanPath(open.Path)] = open.Text
	}
	return overlay
}

func validRenameIdentifier(name string) bool {
	tokens, diagnostics := lexer.Lex("<rename>", name)
	return len(diagnostics) == 0 && len(tokens) == 2 && tokens[0].Kind == token.Identifier && tokens[0].Lexeme == name && tokens[1].Kind == token.EOF
}

func hasDeclarationOccurrence(occurrences []symbolOccurrence, declaration source.Span) bool {
	for _, occurrence := range occurrences {
		if sameSourceSpan(occurrence.Span, declaration) && sameSourceSpan(occurrence.Declaration, declaration) {
			return true
		}
	}
	return false
}

type sourceSpanKey struct {
	Path       string
	Start, End int
}

func spanKey(span source.Span) sourceSpanKey {
	return sourceSpanKey{Path: cleanPath(span.Path), Start: span.Start.Offset, End: span.End.Offset}
}

func (s *Server) symbolOccurrences(program *ast.Program) []symbolOccurrence {
	return s.symbolOccurrencesWithText(program, nil)
}

func (s *Server) symbolOccurrencesWithText(program *ast.Program, textByPath map[string]string) []symbolOccurrence {
	var result []symbolOccurrence
	add := func(span, declaration source.Span) {
		if span.Path == "" || declaration.Path == "" {
			return
		}
		result = append(result, symbolOccurrence{Name: s.sourceTextWithOverlay(span, textByPath), Span: span, Declaration: declaration})
	}
	declare := func(span source.Span) { add(span, span) }

	for _, imported := range program.Imports {
		if imported.Go {
			declare(imported.AliasSpan)
			continue
		}
		for index, nameSpan := range imported.NameSpans {
			if index >= len(imported.Names) {
				continue
			}
			if target, ok := s.topLevelDeclarationSpan(program, imported.ResolvedPath, imported.Names[index], textByPath); ok {
				add(nameSpan, target)
			}
		}
	}

	var walkType func(*ast.TypeRef)
	var walkExpression func(ast.Expression)
	var walkStatement func(ast.Statement)
	var walkBlock func(*ast.BlockStmt)
	walkType = func(ref *ast.TypeRef) {
		if ref == nil {
			return
		}
		add(ref.QualifierSpan, ref.QualifierDeclaration)
		add(ref.NameSpan, ref.ResolvedDeclaration)
		for index := range ref.GenericArguments {
			walkType(&ref.GenericArguments[index])
		}
		walkType(ref.Element)
		walkType(ref.Pointee)
		for index := range ref.Parameters {
			walkType(&ref.Parameters[index])
		}
		walkType(ref.Return)
		for index := range ref.ObjectFields {
			walkType(&ref.ObjectFields[index].Type)
		}
	}
	walkExpression = func(expression ast.Expression) {
		switch expression := expression.(type) {
		case nil, *ast.LiteralExpr:
		case *ast.IdentifierExpr:
			add(expression.Span, expression.ResolvedDeclaration)
		case *ast.UnaryExpr:
			walkExpression(expression.Operand)
		case *ast.PropagateExpr:
			walkExpression(expression.Value)
		case *ast.TaskStartExpr:
			walkExpression(expression.Call)
		case *ast.AwaitExpr:
			walkExpression(expression.Value)
		case *ast.BinaryExpr:
			walkExpression(expression.Left)
			walkExpression(expression.Right)
		case *ast.GoTypeAssertionExpr:
			walkExpression(expression.Value)
			walkType(&expression.Type)
		case *ast.CallExpr:
			walkExpression(expression.Callee)
			for index := range expression.TypeArguments {
				walkType(&expression.TypeArguments[index])
			}
			for _, argument := range expression.Arguments {
				walkExpression(argument)
			}
		case *ast.ArrowExpr:
			for _, parameter := range expression.Parameters {
				declare(parameterNameSpan(parameter))
			}
			for index := range expression.Parameters {
				walkType(&expression.Parameters[index].Type)
			}
			walkType(expression.ReturnType)
			walkExpression(expression.ExpressionBody)
			walkBlock(expression.BlockBody)
		case *ast.ArrayLiteralExpr:
			for _, element := range expression.Elements {
				walkExpression(element)
			}
		case *ast.ObjectLiteralExpr:
			for _, field := range expression.Fields {
				walkExpression(field.Value)
			}
		case *ast.GoCompositeLiteralExpr:
			walkType(&expression.Type)
			for _, field := range expression.Fields {
				add(field.NameSpan, field.ResolvedDeclaration)
				walkExpression(field.Value)
			}
		case *ast.MemberExpr:
			walkExpression(expression.Object)
			add(expression.NameSpan, expression.ResolvedDeclaration)
		case *ast.IndexExpr:
			walkExpression(expression.Object)
			walkExpression(expression.Index)
		case *ast.SliceExpr:
			walkExpression(expression.Object)
			walkExpression(expression.Low)
			walkExpression(expression.High)
			walkExpression(expression.Max)
		case *ast.NewExpr:
			add(expression.ClassNameSpan, expression.ResolvedDeclaration)
			for index := range expression.TypeArguments {
				walkType(&expression.TypeArguments[index])
			}
			for _, argument := range expression.Arguments {
				walkExpression(argument)
			}
		case *ast.ClassUpcastExpr:
			walkExpression(expression.Value)
		}
	}
	walkBlock = func(block *ast.BlockStmt) {
		if block == nil {
			return
		}
		for _, statement := range block.Statements {
			walkStatement(statement)
		}
	}
	walkStatement = func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.LabeledStmt:
			declare(statement.LabelSpan)
			walkStatement(statement.Statement)
		case *ast.BranchStmt:
			add(statement.LabelSpan, statement.ResolvedDeclaration)
		case *ast.VariableDecl:
			declare(statement.NameSpan)
			walkType(&statement.Type)
			walkExpression(statement.Value)
		case *ast.MultiVariableDecl:
			for _, binding := range statement.Bindings {
				if binding.Name != "_" {
					declare(binding.Span)
				}
			}
			walkExpression(statement.Value)
		case *ast.BlockStmt:
			walkBlock(statement)
		case *ast.ReturnStmt:
			walkExpression(statement.Value)
		case *ast.ThrowStmt:
			walkExpression(statement.Value)
		case *ast.TryStmt:
			walkBlock(statement.Body)
			for _, clause := range statement.Catches {
				walkType(&clause.Type)
				if clause.Name != "_" {
					declare(clause.NameSpan)
				}
				walkBlock(clause.Body)
			}
			walkBlock(statement.FinallyBody)
		case *ast.IfStmt:
			walkExpression(statement.Condition)
			walkBlock(statement.Then)
			walkStatement(statement.Else)
		case *ast.ExpressionStmt:
			walkExpression(statement.Value)
		case *ast.AssignmentStmt:
			walkExpression(statement.Target)
			walkExpression(statement.Value)
		case *ast.IncDecStmt:
			walkExpression(statement.Target)
		case *ast.MultiAssignmentStmt:
			for _, binding := range statement.Bindings {
				add(binding.Span, binding.ResolvedDeclaration)
			}
			walkExpression(statement.Value)
		case *ast.WhileStmt:
			walkExpression(statement.Condition)
			walkBlock(statement.Body)
		case *ast.ForStmt:
			walkStatement(statement.Initializer)
			walkExpression(statement.Condition)
			walkStatement(statement.Post)
			walkBlock(statement.Body)
		case *ast.ForRangeStmt:
			walkExpression(statement.Source)
			for index, binding := range statement.Bindings {
				walkType(&statement.Bindings[index].Type)
				if binding.Name != "_" {
					declare(binding.NameSpan)
				}
			}
			walkBlock(statement.Body)
		case *ast.SelectStmt:
			for _, clause := range statement.Cases {
				walkExpression(clause.Channel)
				walkExpression(clause.Value)
				for _, target := range clause.Targets {
					walkExpression(target)
				}
				if clause.Declare {
					for _, binding := range clause.Bindings {
						if binding.Name != "_" {
							declare(binding.Span)
						}
					}
				}
				walkBlock(clause.Body)
			}
		case *ast.ValueSwitchStmt:
			walkExpression(statement.Value)
			for _, clause := range statement.Cases {
				for _, value := range clause.Values {
					walkExpression(value)
				}
				walkBlock(clause.Body)
			}
		case *ast.TypeSwitchStmt:
			walkExpression(statement.Value)
			for index, clause := range statement.Cases {
				walkType(&statement.Cases[index].Type)
				if !clause.Nil && !clause.Default && clause.Name != "_" {
					declare(clause.NameSpan)
				}
				walkBlock(clause.Body)
			}
		case *ast.CallControlStmt:
			walkExpression(statement.Value)
		case *ast.DetachStmt:
			walkExpression(statement.Value)
		case *ast.ChannelSendStmt:
			walkExpression(statement.Channel)
			walkExpression(statement.Value)
		}
	}

	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.CABIExportDecl:
			for index, span := range declaration.NameSpans {
				if index < len(declaration.ResolvedDeclarations) {
					add(span, declaration.ResolvedDeclarations[index])
				}
			}
		case *ast.FunctionDecl:
			declare(declaration.NameSpan)
			for _, parameter := range declaration.TypeParameters {
				declare(parameter.NameSpan)
			}
			for index, parameter := range declaration.Parameters {
				declare(parameterNameSpan(parameter))
				walkType(&declaration.Parameters[index].Type)
			}
			walkType(&declaration.ReturnType)
			walkBlock(declaration.Body)
		case *ast.MethodDecl:
			declare(declaration.ReceiverNameSpan)
			declare(declaration.NameSpan)
			for _, parameter := range declaration.TypeParameters {
				declare(parameter.NameSpan)
			}
			walkType(&declaration.ReceiverType)
			for index, parameter := range declaration.Parameters {
				declare(parameterNameSpan(parameter))
				walkType(&declaration.Parameters[index].Type)
			}
			walkType(&declaration.ReturnType)
			walkBlock(declaration.Body)
		case *ast.VariableDecl:
			declare(declaration.NameSpan)
			walkType(&declaration.Type)
			walkExpression(declaration.Value)
		case *ast.ClassDecl:
			declare(declaration.NameSpan)
			for _, parameter := range declaration.TypeParameters {
				declare(parameter.NameSpan)
			}
			if declaration.Base != nil {
				walkType(declaration.Base)
			}
			for index := range declaration.Implements {
				walkType(&declaration.Implements[index])
			}
			for index := range declaration.Fields {
				field := &declaration.Fields[index]
				declare(field.NameSpan)
				walkType(&field.Type)
			}
			if declaration.Constructor != nil {
				for index, parameter := range declaration.Constructor.Parameters {
					declare(parameterNameSpan(parameter))
					walkType(&declaration.Constructor.Parameters[index].Type)
				}
				walkBlock(declaration.Constructor.Body)
			}
			for _, method := range declaration.Methods {
				declare(method.NameSpan)
				for index, parameter := range method.Parameters {
					declare(parameterNameSpan(parameter))
					walkType(&method.Parameters[index].Type)
				}
				walkType(&method.ReturnType)
				walkBlock(method.Body)
			}
		case *ast.StructDecl:
			declare(declaration.NameSpan)
			for _, parameter := range declaration.TypeParameters {
				declare(parameter.NameSpan)
			}
			for index := range declaration.Fields {
				field := &declaration.Fields[index]
				declare(field.NameSpan)
				walkType(&field.Type)
			}
			for _, method := range declaration.Methods {
				declare(method.NameSpan)
				for index, parameter := range method.Parameters {
					declare(parameterNameSpan(parameter))
					walkType(&method.Parameters[index].Type)
				}
				walkType(&method.ReturnType)
				walkBlock(method.Body)
			}
		case *ast.TypeDecl:
			declare(declaration.NameSpan)
			for _, parameter := range declaration.TypeParameters {
				declare(parameter.NameSpan)
			}
			walkType(&declaration.Underlying)
		case *ast.EnumDecl:
			declare(declaration.NameSpan)
			walkType(&declaration.Underlying)
			for index := range declaration.Members {
				member := &declaration.Members[index]
				declare(member.NameSpan)
				walkExpression(member.Value)
			}
		case *ast.InterfaceDecl:
			declare(declaration.NameSpan)
			for _, parameter := range declaration.TypeParameters {
				declare(parameter.NameSpan)
			}
			for index := range declaration.Methods {
				method := &declaration.Methods[index]
				declare(method.NameSpan)
				for parameterIndex, parameter := range method.Parameters {
					declare(parameterNameSpan(parameter))
					walkType(&method.Parameters[parameterIndex].Type)
				}
				walkType(&method.ReturnType)
			}
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		left, right := cleanPath(result[i].Span.Path), cleanPath(result[j].Span.Path)
		if left != right {
			return left < right
		}
		return result[i].Span.Start.Offset < result[j].Span.Start.Offset
	})
	return result
}

func (s *Server) topLevelDeclarationSpan(program *ast.Program, path, name string, textByPath map[string]string) (source.Span, bool) {
	for _, declaration := range program.Declarations {
		var span source.Span
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			span = declaration.NameSpan
		case *ast.VariableDecl:
			span = declaration.NameSpan
		case *ast.ClassDecl:
			span = declaration.NameSpan
		case *ast.StructDecl:
			span = declaration.NameSpan
		case *ast.TypeDecl:
			span = declaration.NameSpan
		case *ast.EnumDecl:
			span = declaration.NameSpan
		case *ast.InterfaceDecl:
			span = declaration.NameSpan
		default:
			continue
		}
		if samePath(span.Path, path) && s.sourceTextWithOverlay(span, textByPath) == name {
			return span, true
		}
	}
	return source.Span{}, false
}

func (s *Server) sourceText(span source.Span) string {
	return s.sourceTextWithOverlay(span, nil)
}

func (s *Server) sourceTextWithOverlay(span source.Span, textByPath map[string]string) string {
	text, ok := textByPath[cleanPath(span.Path)]
	if !ok {
		text = s.textForPath(span.Path)
	}
	if span.Start.Offset < 0 || span.End.Offset < span.Start.Offset || span.End.Offset > len(text) {
		return ""
	}
	return text[span.Start.Offset:span.End.Offset]
}

func parameterNameSpan(parameter ast.Parameter) source.Span {
	return nameSpan(parameter.Span, parameter.Name)
}

func cleanPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(absolute)
}

func sameSourceSpan(left, right source.Span) bool {
	return samePath(left.Path, right.Path) && left.Start.Offset == right.Start.Offset && left.End.Offset == right.End.Offset
}

func occurrenceAt(items []symbolOccurrence, path string, offset int) (symbolOccurrence, bool) {
	for _, item := range items {
		if samePath(item.Span.Path, path) && item.Span.Start.Offset <= offset && offset <= item.Span.End.Offset {
			return item, true
		}
	}
	return symbolOccurrence{}, false
}
