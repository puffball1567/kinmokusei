package lsp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/compiler"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/source"
	"github.com/puffball1567/kinmokusei/internal/token"
)

type parameterInformation struct {
	Label string `json:"label"`
}

type signatureInformation struct {
	Label      string                 `json:"label"`
	Parameters []parameterInformation `json:"parameters,omitempty"`
}

type signatureHelpResult struct {
	Signatures      []signatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type callContext struct {
	Name            string
	Qualifier       string
	DisplayName     string
	Constructor     bool
	CalleeOffset    int
	OpenOffset      int
	ActiveParameter int
}

type delimiterFrame struct {
	opening token.Kind
	call    *callContext
}

func (s *Server) signatureHelp(id json.RawMessage, raw json.RawMessage) error {
	doc, offset, ok := s.navigationParams(raw)
	if !ok || insideComment(doc.Text, offset) {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	context, ok := callContextAt(doc.Path, doc.Text, offset)
	if !ok {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	result, err := compiler.CheckFilesWithOverlay([]string{doc.Path}, s.documentOverlay())
	if err != nil || result.Program == nil {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	signature, found := s.resolvedSignature(result, doc, context)
	if !found && context.Qualifier != "" {
		overlay := s.documentOverlay()
		overlay[doc.Path] = qualifiedCallAnalysisText(doc.Text, offset, context)
		if recovered, recoveryErr := compiler.CheckFilesWithOverlay([]string{doc.Path}, overlay); recoveryErr == nil && recovered.Program != nil {
			signature, found = s.goValueMethodSignature(recovered, doc, context)
		}
	}
	if !found {
		return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	}
	parameters := make([]parameterInformation, len(signature.ParameterTypes))
	labels := make([]string, len(signature.ParameterTypes))
	for index, parameterType := range signature.ParameterTypes {
		name := ""
		if index < len(signature.ParameterNames) {
			name = signature.ParameterNames[index]
		}
		if name == "" {
			name = fmt.Sprintf("arg%d", index+1)
		}
		label := name + ": " + parameterType
		if signature.Variadic && index == len(signature.ParameterTypes)-1 {
			label = "..." + label
		}
		labels[index] = label
		parameters[index] = parameterInformation{Label: label}
	}
	active := context.ActiveParameter
	if len(parameters) == 0 {
		active = 0
	} else if active >= len(parameters) {
		active = len(parameters) - 1
	}
	label := context.DisplayName + "(" + strings.Join(labels, ", ") + "): " + signature.Result
	help := signatureHelpResult{
		Signatures:      []signatureInformation{{Label: label, Parameters: parameters}},
		ActiveSignature: 0,
		ActiveParameter: active,
	}
	return s.writeResponse(response{JSONRPC: "2.0", ID: id, Result: help})
}

func qualifiedCallAnalysisText(value string, offset int, context callContext) string {
	start := context.CalleeOffset - 1
	if context.Qualifier == "" || start < 0 || offset > len(value) || value[start] != '.' {
		return value
	}
	result := []byte(value)
	for index := start; index < offset; index++ {
		if result[index] != '\r' && result[index] != '\n' {
			result[index] = ' '
		}
	}
	recovered := string(result)
	if offset == len(value) {
		depth := 0
		tokens, _ := lexer.Lex("<signature-recovery>", value[:start])
		for _, item := range tokens {
			switch item.Kind {
			case token.LeftBrace:
				depth++
			case token.RightBrace:
				if depth > 0 {
					depth--
				}
			}
		}
		recovered += ";" + strings.Repeat(" }", depth)
	}
	return recovered
}

func (s *Server) resolvedSignature(result compiler.Result, doc document, context callContext) (ast.CallableSignature, bool) {
	if context.Constructor {
		if expression := findNewExpression(result.Program, doc.Path, context); expression != nil {
			if class := sourceClassDeclaration(s, result.Program, doc.Path, context.Name); class != nil {
				return signatureFromGenericClass(class, expression.TypeArguments), true
			}
		}
	}
	if call := findCallExpression(result.Program, doc.Path, context); call != nil && call.Signature != nil {
		signature := *call.Signature
		var declaration source.Span
		switch callee := call.Callee.(type) {
		case *ast.IdentifierExpr:
			declaration = callee.ResolvedDeclaration
		case *ast.MemberExpr:
			declaration = callee.ResolvedDeclaration
		}
		named := false
		if declaration.Path != "" {
			if parameters := parametersAtDeclaration(result.Program, declaration); len(parameters) == len(signature.ParameterTypes) {
				signature.ParameterNames = parameterNames(parameters)
				if hasVariadicParameter(parameters) {
					signature.Variadic = true
					signature.ParameterTypes[len(parameters)-1] = formatTypeRef(parameters[len(parameters)-1].Type)
				}
				named = true
			}
		}
		if !named {
			if _, arrow, visible := visibleCallable(result.Program, doc.Path, context.OpenOffset, context.Name); visible && arrow != nil && len(arrow.Parameters) == len(signature.ParameterTypes) {
				signature.ParameterNames = make([]string, len(arrow.Parameters))
				for index, parameter := range arrow.Parameters {
					signature.ParameterNames[index] = parameter.Name
				}
				if hasVariadicParameter(arrow.Parameters) {
					signature.Variadic = true
					signature.ParameterTypes[len(arrow.Parameters)-1] = formatTypeRef(arrow.Parameters[len(arrow.Parameters)-1].Type)
				}
			}
		}
		return signature, true
	}
	if signature, ok := s.sourceSignature(result.Program, doc.Path, context); ok {
		return signature, true
	}
	if context.Qualifier != "" {
		if signature, ok := s.goValueMethodSignature(result, doc, context); ok {
			return signature, true
		}
		for _, imported := range result.Program.Imports {
			if !imported.Go || imported.Alias != context.Qualifier || !samePath(imported.Span.Path, doc.Path) {
				continue
			}
			goSignature, found, err := result.GoPackageFunctionSignature(imported.Path, context.Name)
			if err != nil || !found {
				return ast.CallableSignature{}, false
			}
			return ast.CallableSignature{
				ParameterNames: goSignature.ParameterNames,
				ParameterTypes: goSignature.ParameterTypes,
				Result:         goSignature.Result,
				Variadic:       goSignature.Variadic,
			}, true
		}
	}
	return ast.CallableSignature{}, false
}

func (s *Server) goValueMethodSignature(result compiler.Result, doc document, context callContext) (ast.CallableSignature, bool) {
	receiver, ok := completionReceiverAt(result.Program, doc.Path, context.OpenOffset, context.Qualifier)
	if !ok {
		return ast.CallableSignature{}, false
	}
	ref, pointer, ok := goCompletionTypeInfo(receiver.typeRef)
	if !ok {
		return ast.CallableSignature{}, false
	}
	importPath := goImportPathForQualifier(result.Program, doc.Path, ref.Qualifier)
	if importPath == "" {
		return ast.CallableSignature{}, false
	}
	goSignature, found, err := result.GoTypeMethodSignature(importPath, ref.Name, pointer, true, context.Name)
	if err != nil || !found {
		return ast.CallableSignature{}, false
	}
	return ast.CallableSignature{
		ParameterNames: goSignature.ParameterNames,
		ParameterTypes: goSignature.ParameterTypes,
		Result:         goSignature.Result,
		Variadic:       goSignature.Variadic,
	}, true
}

func (s *Server) sourceSignature(program *ast.Program, path string, context callContext) (ast.CallableSignature, bool) {
	if context.Constructor {
		if class := sourceClassDeclaration(s, program, path, context.Name); class != nil {
			return signatureFromClass(class), true
		}
		if context.Name == "Exception" {
			return ast.CallableSignature{
				ParameterNames: []string{"message"},
				ParameterTypes: []string{"string"},
				Result:         "Exception",
			}, true
		}
		return ast.CallableSignature{}, false
	}
	if context.Qualifier != "" {
		return ast.CallableSignature{}, false
	}
	if ref, arrow, ok := visibleCallable(program, path, context.OpenOffset, context.Name); ok {
		if arrow != nil {
			return signatureFromArrow(arrow), true
		}
		return signatureFromTypeRef(ref), true
	}
	for _, declaration := range program.Declarations {
		function, ok := declaration.(*ast.FunctionDecl)
		if ok && samePath(function.Span.Path, path) && s.sourceText(function.NameSpan) == context.Name {
			return signatureFromFunction(function), true
		}
	}
	for _, imported := range program.Imports {
		if imported.Go || !samePath(imported.Span.Path, path) {
			continue
		}
		for _, name := range imported.Names {
			if name != context.Name {
				continue
			}
			for _, declaration := range program.Declarations {
				function, ok := declaration.(*ast.FunctionDecl)
				if ok && samePath(function.Span.Path, imported.ResolvedPath) && s.sourceText(function.NameSpan) == name {
					return signatureFromFunction(function), true
				}
			}
		}
	}
	if signature, ok := builtinSignature(context.Name); ok {
		return signature, true
	}
	return ast.CallableSignature{}, false
}

func builtinSignature(name string) (ast.CallableSignature, bool) {
	signatures := map[string]ast.CallableSignature{
		"len":            {ParameterNames: []string{"value"}, ParameterTypes: []string{"collection"}, Result: "int"},
		"cap":            {ParameterNames: []string{"value"}, ParameterTypes: []string{"capacity collection"}, Result: "int"},
		"append":         {ParameterNames: []string{"destination", "values"}, ParameterTypes: []string{"T[]", "T"}, Result: "T[]", Variadic: true},
		"copy":           {ParameterNames: []string{"destination", "source"}, ParameterTypes: []string{"T[]", "T[]"}, Result: "int"},
		"delete":         {ParameterNames: []string{"entries", "key"}, ParameterTypes: []string{"Map<K, V>", "K"}, Result: "void"},
		"clear":          {ParameterNames: []string{"collection"}, ParameterTypes: []string{"T[] | Map<K, V>"}, Result: "void"},
		"min":            {ParameterNames: []string{"values"}, ParameterTypes: []string{"ordered"}, Result: "T", Variadic: true},
		"max":            {ParameterNames: []string{"values"}, ParameterTypes: []string{"ordered"}, Result: "T", Variadic: true},
		"makeSlice":      {ParameterNames: []string{"length", "capacity?"}, ParameterTypes: []string{"int", "int"}, Result: "T[]"},
		"makeMap":        {ParameterNames: []string{"capacity?"}, ParameterTypes: []string{"int"}, Result: "Map<K, V>"},
		"copyArray":      {ParameterNames: []string{"source"}, ParameterTypes: []string{"T[]"}, Result: "[N]T"},
		"viewArray":      {ParameterNames: []string{"source"}, ParameterTypes: []string{"T[]"}, Result: "*[N]T"},
		"goChannel":      {ParameterNames: []string{"capacity?"}, ParameterTypes: []string{"int"}, Result: "GoChannel<T>"},
		"closeGoChannel": {ParameterNames: []string{"channel"}, ParameterTypes: []string{"GoChannel<T>"}, Result: "void"},
	}
	signature, ok := signatures[name]
	return signature, ok
}

func sourceClassDeclaration(s *Server, program *ast.Program, path, name string) *ast.ClassDecl {
	for _, declaration := range program.Declarations {
		class, ok := declaration.(*ast.ClassDecl)
		if ok && samePath(class.Span.Path, path) && s.sourceText(class.NameSpan) == name {
			return class
		}
	}
	for _, imported := range program.Imports {
		if imported.Go || !samePath(imported.Span.Path, path) {
			continue
		}
		for _, importedName := range imported.Names {
			if importedName != name {
				continue
			}
			for _, declaration := range program.Declarations {
				class, ok := declaration.(*ast.ClassDecl)
				if ok && samePath(class.Span.Path, imported.ResolvedPath) && s.sourceText(class.NameSpan) == name {
					return class
				}
			}
		}
	}
	return nil
}

func signatureFromClass(class *ast.ClassDecl) ast.CallableSignature {
	return signatureFromGenericClass(class, nil)
}

func signatureFromGenericClass(class *ast.ClassDecl, arguments []ast.TypeRef) ast.CallableSignature {
	result := ast.CallableSignature{Result: class.Name}
	bindings := map[string]ast.TypeRef(nil)
	if len(class.TypeParameters) != 0 && len(class.TypeParameters) == len(arguments) {
		bindings = make(map[string]ast.TypeRef, len(arguments))
		formatted := make([]string, len(arguments))
		for index, parameter := range class.TypeParameters {
			bindings[parameter.Name] = arguments[index]
			formatted[index] = formatTypeRef(arguments[index])
		}
		result.Result += "<" + strings.Join(formatted, ", ") + ">"
	}
	if class.Constructor == nil {
		return result
	}
	result.ParameterNames = make([]string, len(class.Constructor.Parameters))
	result.ParameterTypes = make([]string, len(class.Constructor.Parameters))
	for index, parameter := range class.Constructor.Parameters {
		result.ParameterNames[index] = parameter.Name
		result.ParameterTypes[index] = formatTypeRef(substituteTypeRefParameters(parameter.Type, bindings))
	}
	result.Variadic = hasVariadicParameter(class.Constructor.Parameters)
	return result
}

func signatureFromFunction(function *ast.FunctionDecl) ast.CallableSignature {
	result := ast.CallableSignature{
		ParameterNames: make([]string, len(function.Parameters)),
		ParameterTypes: make([]string, len(function.Parameters)),
		Result:         formatTypeRef(function.ReturnType),
	}
	for index, parameter := range function.Parameters {
		result.ParameterNames[index] = parameter.Name
		result.ParameterTypes[index] = formatTypeRef(parameter.Type)
	}
	result.Variadic = hasVariadicParameter(function.Parameters)
	return result
}

func signatureFromTypeRef(ref ast.TypeRef) ast.CallableSignature {
	result := ast.CallableSignature{ParameterTypes: make([]string, len(ref.Parameters)), Result: "void", Variadic: ref.Variadic}
	for index, parameter := range ref.Parameters {
		result.ParameterTypes[index] = formatTypeRef(parameter)
	}
	if ref.Return != nil {
		result.Result = formatTypeRef(*ref.Return)
	}
	return result
}

func signatureFromArrow(arrow *ast.ArrowExpr) ast.CallableSignature {
	result := ast.CallableSignature{
		ParameterNames: make([]string, len(arrow.Parameters)),
		ParameterTypes: make([]string, len(arrow.Parameters)),
		Result:         formatTypeRef(arrow.ResolvedReturnType),
	}
	for index, parameter := range arrow.Parameters {
		result.ParameterNames[index] = parameter.Name
		result.ParameterTypes[index] = formatTypeRef(parameter.Type)
	}
	result.Variadic = hasVariadicParameter(arrow.Parameters)
	if arrow.ReturnType != nil {
		result.Result = formatTypeRef(*arrow.ReturnType)
	}
	if result.Result == "<inferred>" {
		result.Result = "<inferred>"
	}
	return result
}

func hasVariadicParameter(parameters []ast.Parameter) bool {
	return len(parameters) != 0 && parameters[len(parameters)-1].Variadic
}

func parameterNames(parameters []ast.Parameter) []string {
	names := make([]string, len(parameters))
	for index, parameter := range parameters {
		names[index] = parameter.Name
	}
	return names
}

func parametersAtDeclaration(program *ast.Program, declaration source.Span) []ast.Parameter {
	if declaration.Path == "" {
		return nil
	}
	for _, candidate := range program.Declarations {
		if function, ok := candidate.(*ast.FunctionDecl); ok && sameSourceSpan(function.NameSpan, declaration) {
			return function.Parameters
		}
		if method, ok := candidate.(*ast.MethodDecl); ok && sameSourceSpan(method.NameSpan, declaration) {
			return method.Parameters
		}
		var methods []*ast.MethodDecl
		switch candidate := candidate.(type) {
		case *ast.ClassDecl:
			methods = candidate.Methods
		case *ast.StructDecl:
			methods = candidate.Methods
		case *ast.InterfaceDecl:
			for _, method := range candidate.Methods {
				if sameSourceSpan(method.NameSpan, declaration) {
					return method.Parameters
				}
			}
		}
		for _, method := range methods {
			if sameSourceSpan(method.NameSpan, declaration) {
				return method.Parameters
			}
		}
	}
	return nil
}

func parameterNamesAtDeclaration(program *ast.Program, declaration source.Span) []string {
	return parameterNames(parametersAtDeclaration(program, declaration))
}

func callContextAt(path, text string, offset int) (callContext, bool) {
	if offset < 0 || offset > len(text) {
		return callContext{}, false
	}
	tokens, _ := lexer.Lex(path, text)
	var frames []delimiterFrame
	var consumed []token.Token
	for _, item := range tokens {
		if item.Kind == token.EOF || item.Span.Start.Offset >= offset {
			break
		}
		switch item.Kind {
		case token.LeftParen:
			var call *callContext
			if context, ok := calleeBefore(consumed, item.Span.Start.Offset); ok {
				call = &context
			}
			frames = append(frames, delimiterFrame{opening: token.LeftParen, call: call})
		case token.LeftBracket, token.LeftBrace:
			frames = append(frames, delimiterFrame{opening: item.Kind})
		case token.Comma:
			if len(frames) != 0 && frames[len(frames)-1].opening == token.LeftParen && frames[len(frames)-1].call != nil {
				frames[len(frames)-1].call.ActiveParameter++
			}
		case token.RightParen:
			frames = closeDelimiter(frames, token.LeftParen)
		case token.RightBracket:
			frames = closeDelimiter(frames, token.LeftBracket)
		case token.RightBrace:
			frames = closeDelimiter(frames, token.LeftBrace)
		}
		consumed = append(consumed, item)
	}
	for index := len(frames) - 1; index >= 0; index-- {
		if frames[index].call != nil {
			return *frames[index].call, true
		}
	}
	return callContext{}, false
}

func calleeBefore(tokens []token.Token, openOffset int) (callContext, bool) {
	index := len(tokens) - 1
	if index < 0 {
		return callContext{}, false
	}
	if tokens[index].Kind == token.Greater || tokens[index].Kind == token.ShiftRight {
		depth := 0
		for index >= 0 {
			switch tokens[index].Kind {
			case token.Greater:
				depth++
			case token.ShiftRight:
				depth += 2
			case token.Less:
				depth--
				if depth <= 0 {
					index--
					goto angleArgumentsSkipped
				}
			}
			index--
		}
		return callContext{}, false
	}

angleArgumentsSkipped:
	if index < 0 {
		return callContext{}, false
	}
	if tokens[index].Kind == token.RightBracket {
		depth := 1
		index--
		for index >= 0 && depth != 0 {
			switch tokens[index].Kind {
			case token.RightBracket:
				depth++
			case token.LeftBracket:
				depth--
			}
			index--
		}
	}
	if index < 0 {
		return callContext{}, false
	}
	item := tokens[index]
	if item.Kind == token.RightParen {
		return callContext{DisplayName: "<expression>", CalleeOffset: item.Span.Start.Offset, OpenOffset: openOffset}, true
	}
	if item.Kind != token.Identifier && item.Kind != token.This {
		return callContext{}, false
	}
	if index > 0 && tokens[index-1].Kind == token.New {
		return callContext{
			Name: item.Lexeme, DisplayName: "new " + item.Lexeme, Constructor: true,
			CalleeOffset: item.Span.Start.Offset, OpenOffset: openOffset,
		}, true
	}
	if index > 0 && (tokens[index-1].Kind == token.Function || tokens[index-1].Kind == token.Constructor) {
		return callContext{}, false
	}
	context := callContext{Name: item.Lexeme, DisplayName: item.Lexeme, CalleeOffset: item.Span.Start.Offset, OpenOffset: openOffset}
	if index >= 2 && tokens[index-1].Kind == token.Dot && (tokens[index-2].Kind == token.Identifier || tokens[index-2].Kind == token.This) {
		context.Qualifier = tokens[index-2].Lexeme
		context.DisplayName = context.Qualifier + "." + context.Name
	}
	return context, true
}

func closeDelimiter(frames []delimiterFrame, opening token.Kind) []delimiterFrame {
	for index := len(frames) - 1; index >= 0; index-- {
		if frames[index].opening == opening {
			return frames[:index]
		}
	}
	return frames
}

func insideComment(text string, offset int) bool {
	lineComment, blockComment, inString, escaped := false, false, false, false
	for index := 0; index < offset && index < len(text); index++ {
		current := text[index]
		next := byte(0)
		if index+1 < len(text) {
			next = text[index+1]
		}
		if lineComment {
			if current == '\n' || current == '\r' {
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
	return lineComment || blockComment
}

func findCallExpression(program *ast.Program, path string, context callContext) *ast.CallExpr {
	var candidates []*ast.CallExpr
	visitProgramExpressions(program, func(expression ast.Expression) {
		call, ok := expression.(*ast.CallExpr)
		if !ok || !samePath(call.Span.Path, path) || call.Span.Start.Offset > context.OpenOffset || call.Span.End.Offset < context.OpenOffset {
			return
		}
		callee := call.Callee.GetSpan()
		if callee.Start.Offset <= context.CalleeOffset && context.CalleeOffset <= callee.End.Offset {
			candidates = append(candidates, call)
		}
	})
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Span.End.Offset-candidates[i].Span.Start.Offset < candidates[j].Span.End.Offset-candidates[j].Span.Start.Offset
	})
	return candidates[0]
}

func findNewExpression(program *ast.Program, path string, context callContext) *ast.NewExpr {
	var candidates []*ast.NewExpr
	visitProgramExpressions(program, func(expression ast.Expression) {
		created, ok := expression.(*ast.NewExpr)
		if !ok || !samePath(created.Span.Path, path) || created.ClassName != context.Name || created.Span.Start.Offset > context.OpenOffset || created.Span.End.Offset < context.OpenOffset {
			return
		}
		if created.ClassNameSpan.Start.Offset <= context.CalleeOffset && context.CalleeOffset <= created.ClassNameSpan.End.Offset {
			candidates = append(candidates, created)
		}
	})
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Span.End.Offset-candidates[i].Span.Start.Offset < candidates[j].Span.End.Offset-candidates[j].Span.Start.Offset
	})
	return candidates[0]
}

func visitProgramExpressions(program *ast.Program, visit func(ast.Expression)) {
	var expression func(ast.Expression)
	var statement func(ast.Statement)
	var block func(*ast.BlockStmt)
	expression = func(value ast.Expression) {
		if value == nil {
			return
		}
		visit(value)
		switch value := value.(type) {
		case *ast.UnaryExpr:
			expression(value.Operand)
		case *ast.PropagateExpr:
			expression(value.Value)
		case *ast.TaskStartExpr:
			expression(value.Call)
		case *ast.AwaitExpr:
			expression(value.Value)
		case *ast.BinaryExpr:
			expression(value.Left)
			expression(value.Right)
		case *ast.GoTypeAssertionExpr:
			expression(value.Value)
		case *ast.CallExpr:
			expression(value.Callee)
			for _, argument := range value.Arguments {
				expression(argument)
			}
		case *ast.ArrowExpr:
			expression(value.ExpressionBody)
			block(value.BlockBody)
		case *ast.ArrayLiteralExpr:
			for _, item := range value.Elements {
				expression(item)
			}
		case *ast.ObjectLiteralExpr:
			for _, field := range value.Fields {
				expression(field.Value)
			}
		case *ast.GoCompositeLiteralExpr:
			for _, field := range value.Fields {
				expression(field.Value)
			}
		case *ast.MemberExpr:
			expression(value.Object)
		case *ast.IndexExpr:
			expression(value.Object)
			expression(value.Index)
		case *ast.SliceExpr:
			expression(value.Object)
			expression(value.Low)
			expression(value.High)
			expression(value.Max)
		case *ast.NewExpr:
			for _, argument := range value.Arguments {
				expression(argument)
			}
		case *ast.ClassUpcastExpr:
			expression(value.Value)
		}
	}
	block = func(value *ast.BlockStmt) {
		if value == nil {
			return
		}
		for _, item := range value.Statements {
			statement(item)
		}
	}
	statement = func(value ast.Statement) {
		switch value := value.(type) {
		case *ast.LabeledStmt:
			statement(value.Statement)
		case *ast.VariableDecl:
			expression(value.Value)
		case *ast.MultiVariableDecl:
			expression(value.Value)
		case *ast.BlockStmt:
			block(value)
		case *ast.ReturnStmt:
			expression(value.Value)
		case *ast.ThrowStmt:
			expression(value.Value)
		case *ast.TryStmt:
			block(value.Body)
			for _, clause := range value.Catches {
				block(clause.Body)
			}
			block(value.FinallyBody)
		case *ast.IfStmt:
			expression(value.Condition)
			block(value.Then)
			statement(value.Else)
		case *ast.ExpressionStmt:
			expression(value.Value)
		case *ast.AssignmentStmt:
			expression(value.Target)
			expression(value.Value)
		case *ast.IncDecStmt:
			expression(value.Target)
		case *ast.MultiAssignmentStmt:
			expression(value.Value)
		case *ast.WhileStmt:
			expression(value.Condition)
			block(value.Body)
		case *ast.ForStmt:
			statement(value.Initializer)
			expression(value.Condition)
			statement(value.Post)
			block(value.Body)
		case *ast.ForRangeStmt:
			expression(value.Source)
			block(value.Body)
		case *ast.SelectStmt:
			for _, clause := range value.Cases {
				expression(clause.Channel)
				expression(clause.Value)
				for _, target := range clause.Targets {
					expression(target)
				}
				block(clause.Body)
			}
		case *ast.ValueSwitchStmt:
			expression(value.Value)
			for _, clause := range value.Cases {
				for _, item := range clause.Values {
					expression(item)
				}
				block(clause.Body)
			}
		case *ast.TypeSwitchStmt:
			expression(value.Value)
			for _, clause := range value.Cases {
				block(clause.Body)
			}
		case *ast.CallControlStmt:
			expression(value.Value)
		case *ast.DetachStmt:
			expression(value.Value)
		case *ast.ChannelSendStmt:
			expression(value.Channel)
			expression(value.Value)
		}
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			block(declaration.Body)
		case *ast.MethodDecl:
			block(declaration.Body)
		case *ast.VariableDecl:
			expression(declaration.Value)
		case *ast.ClassDecl:
			if declaration.Constructor != nil {
				block(declaration.Constructor.Body)
			}
			for _, method := range declaration.Methods {
				block(method.Body)
			}
		}
	}
}

func visibleCallable(program *ast.Program, path string, offset int, name string) (ast.TypeRef, *ast.ArrowExpr, bool) {
	for _, declaration := range program.Declarations {
		if !spanContains(declaration.GetSpan(), path, offset) {
			continue
		}
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			if ref, ok := callableParameter(declaration.Parameters, name); ok {
				return ref, nil, true
			}
			return visibleCallableInBlock(declaration.Body, path, offset, name)
		case *ast.MethodDecl:
			if ref, ok := callableParameter(declaration.Parameters, name); ok {
				return ref, nil, true
			}
			return visibleCallableInBlock(declaration.Body, path, offset, name)
		case *ast.ClassDecl:
			if declaration.Constructor != nil && spanContains(declaration.Constructor.Span, path, offset) {
				if ref, ok := callableParameter(declaration.Constructor.Parameters, name); ok {
					return ref, nil, true
				}
				return visibleCallableInBlock(declaration.Constructor.Body, path, offset, name)
			}
			for _, method := range declaration.Methods {
				if spanContains(method.Span, path, offset) {
					if ref, ok := callableParameter(method.Parameters, name); ok {
						return ref, nil, true
					}
					return visibleCallableInBlock(method.Body, path, offset, name)
				}
			}
		}
	}
	for _, declaration := range program.Declarations {
		variable, ok := declaration.(*ast.VariableDecl)
		if !ok || variable.Name != name {
			continue
		}
		if variable.Type.IsFunction() {
			return variable.Type, nil, true
		}
		if arrow, ok := variable.Value.(*ast.ArrowExpr); ok {
			return ast.TypeRef{}, arrow, true
		}
	}
	return ast.TypeRef{}, nil, false
}

func callableParameter(parameters []ast.Parameter, name string) (ast.TypeRef, bool) {
	for _, parameter := range parameters {
		if parameter.Name == name && parameter.Type.IsFunction() {
			return parameter.Type, true
		}
	}
	return ast.TypeRef{}, false
}

func visibleCallableInBlock(block *ast.BlockStmt, path string, offset int, name string) (ast.TypeRef, *ast.ArrowExpr, bool) {
	if block == nil || !spanContains(block.Span, path, offset) {
		return ast.TypeRef{}, nil, false
	}
	var ref ast.TypeRef
	var arrow *ast.ArrowExpr
	found := false
	for _, item := range block.Statements {
		if item.GetSpan().Start.Offset >= offset {
			break
		}
		if variable, ok := item.(*ast.VariableDecl); ok && variable.Name == name {
			if variable.Type.IsFunction() {
				ref, arrow, found = variable.Type, nil, true
			}
			if value, ok := variable.Value.(*ast.ArrowExpr); ok {
				ref, arrow, found = ast.TypeRef{}, value, true
			}
		}
		if item.GetSpan().End.Offset < offset {
			continue
		}
		var nested *ast.BlockStmt
		switch item := item.(type) {
		case *ast.LabeledStmt:
			if ref, arrow, ok := visibleCallableInBlock(&ast.BlockStmt{Statements: []ast.Statement{item.Statement}, Span: item.Span}, path, offset, name); ok {
				return ref, arrow, true
			}
		case *ast.BlockStmt:
			nested = item
		case *ast.IfStmt:
			if spanContains(item.Then.Span, path, offset) {
				nested = item.Then
			} else if value, ok := item.Else.(*ast.BlockStmt); ok && spanContains(value.Span, path, offset) {
				nested = value
			}
		case *ast.TryStmt:
			if spanContains(item.Body.Span, path, offset) {
				nested = item.Body
			} else {
				for _, clause := range item.Catches {
					if spanContains(clause.Body.Span, path, offset) {
						nested = clause.Body
						break
					}
				}
			}
			if nested == nil && item.FinallyBody != nil && spanContains(item.FinallyBody.Span, path, offset) {
				nested = item.FinallyBody
			}
		case *ast.WhileStmt:
			nested = item.Body
		case *ast.ForStmt:
			nested = item.Body
		case *ast.ForRangeStmt:
			nested = item.Body
		}
		if nested != nil {
			if nestedRef, nestedArrow, ok := visibleCallableInBlock(nested, path, offset, name); ok {
				return nestedRef, nestedArrow, true
			}
		}
	}
	return ref, arrow, found
}
