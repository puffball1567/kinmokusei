package lsp

import (
	"strings"

	"github.com/puffball1567/onsentamago/internal/ast"
	"github.com/puffball1567/onsentamago/internal/source"
)

type completionReceiver struct {
	typeRef ast.TypeRef
	owner   string
	static  bool
}

func sourceMemberCompletions(program *ast.Program, path string, offset int, qualifier, prefix string) []completionItem {
	receiver, ok := completionReceiverAt(program, path, offset, qualifier)
	if !ok {
		return []completionItem{}
	}
	candidates := map[string]completionItem{}
	add := func(item completionItem) {
		if strings.HasPrefix(item.Label, prefix) {
			candidates[item.Label] = item
		}
	}
	collectTypeMemberCompletions(program, receiver.typeRef, receiver.owner, receiver.static, map[string]bool{}, add)
	items := make([]completionItem, 0, len(candidates))
	for _, item := range candidates {
		items = append(items, item)
	}
	sortCompletionItems(items)
	return items
}

func completionReceiverAt(program *ast.Program, path string, offset int, qualifier string) (completionReceiver, bool) {
	owner := enclosingMemberOwner(program, path, offset)
	if qualifier == "this" {
		if ref, ok := enclosingThisType(program, path, offset); ok {
			return completionReceiver{typeRef: ref, owner: owner}, true
		}
		return completionReceiver{}, false
	}
	if ref, ok := visibleValueType(program, path, offset, qualifier); ok {
		return completionReceiver{typeRef: ref, owner: owner}, true
	}
	if qualifier == "Exception" {
		return completionReceiver{typeRef: ast.TypeRef{Name: qualifier}, owner: owner, static: true}, true
	}
	if declaration := sourceVisibleNamedType(program, path, qualifier); declaration != nil {
		return completionReceiver{typeRef: typeRefForDeclaration(declaration), owner: owner, static: true}, true
	}
	return completionReceiver{}, false
}

func enclosingThisType(program *ast.Program, path string, offset int) (ast.TypeRef, bool) {
	for _, declaration := range program.Declarations {
		if !spanContains(declaration.GetSpan(), path, offset) {
			continue
		}
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			if declaration.Constructor != nil && spanContains(declaration.Constructor.Span, path, offset) {
				return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan}, true
			}
			for _, method := range declaration.Methods {
				if spanContains(method.Span, path, offset) && !method.Static {
					return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan}, true
				}
			}
		case *ast.StructDecl:
			for _, method := range declaration.Methods {
				if spanContains(method.Span, path, offset) {
					return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan}, true
				}
			}
		case *ast.MethodDecl:
			if spanContains(declaration.Span, path, offset) && declaration.ReceiverName == "this" {
				return declaration.ReceiverType, true
			}
		}
	}
	return ast.TypeRef{}, false
}

func enclosingMemberOwner(program *ast.Program, path string, offset int) string {
	for _, declaration := range program.Declarations {
		if !spanContains(declaration.GetSpan(), path, offset) {
			continue
		}
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			if declaration.Constructor != nil && spanContains(declaration.Constructor.Span, path, offset) {
				return declaration.Name
			}
			for _, method := range declaration.Methods {
				if spanContains(method.Span, path, offset) {
					return declaration.Name
				}
			}
		case *ast.StructDecl:
			for _, method := range declaration.Methods {
				if spanContains(method.Span, path, offset) {
					return declaration.Name
				}
			}
		case *ast.MethodDecl:
			if spanContains(declaration.Span, path, offset) {
				ref := declaration.ReceiverType
				if ref.IsPointer() && ref.Pointee != nil {
					ref = *ref.Pointee
				}
				return ref.Name
			}
		}
	}
	return ""
}

func visibleValueType(program *ast.Program, path string, offset int, name string) (ast.TypeRef, bool) {
	for _, declaration := range program.Declarations {
		if !spanContains(declaration.GetSpan(), path, offset) {
			continue
		}
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			if ref, ok := visibleValueTypeInBlock(program, declaration.Body, path, offset, name); ok {
				return ref, true
			}
			if ref, ok := parameterType(declaration.Parameters, name); ok {
				return ref, true
			}
			break
		case *ast.MethodDecl:
			if ref, ok := visibleValueTypeInBlock(program, declaration.Body, path, offset, name); ok {
				return ref, true
			}
			if ref, ok := parameterType(declaration.Parameters, name); ok {
				return ref, true
			}
			if declaration.ReceiverName == name {
				return declaration.ReceiverType, true
			}
			break
		case *ast.ClassDecl:
			if declaration.Constructor != nil && spanContains(declaration.Constructor.Span, path, offset) {
				if ref, ok := visibleValueTypeInBlock(program, declaration.Constructor.Body, path, offset, name); ok {
					return ref, true
				}
				if ref, ok := parameterType(declaration.Constructor.Parameters, name); ok {
					return ref, true
				}
				break
			}
			for _, method := range declaration.Methods {
				if !spanContains(method.Span, path, offset) {
					continue
				}
				if ref, ok := visibleValueTypeInBlock(program, method.Body, path, offset, name); ok {
					return ref, true
				}
				if ref, ok := parameterType(method.Parameters, name); ok {
					return ref, true
				}
				break
			}
		case *ast.StructDecl:
			for _, method := range declaration.Methods {
				if !spanContains(method.Span, path, offset) {
					continue
				}
				if ref, ok := visibleValueTypeInBlock(program, method.Body, path, offset, name); ok {
					return ref, true
				}
				if ref, ok := parameterType(method.Parameters, name); ok {
					return ref, true
				}
				break
			}
		}
	}
	for _, declaration := range program.Declarations {
		variable, ok := declaration.(*ast.VariableDecl)
		if ok && variable.Name == name && samePath(variable.Span.Path, path) {
			return variableType(program, variable)
		}
	}
	return ast.TypeRef{}, false
}

func parameterType(parameters []ast.Parameter, name string) (ast.TypeRef, bool) {
	for _, parameter := range parameters {
		if parameter.Name == name {
			return parameter.Type, true
		}
	}
	return ast.TypeRef{}, false
}

func variableType(program *ast.Program, variable *ast.VariableDecl) (ast.TypeRef, bool) {
	if variable.Type.IsSpecified() {
		return variable.Type, true
	}
	if variable.ResolvedType.IsSpecified() {
		return variable.ResolvedType, true
	}
	return expressionCompletionType(program, variable.Value)
}

func expressionCompletionType(program *ast.Program, expression ast.Expression) (ast.TypeRef, bool) {
	switch expression := expression.(type) {
	case *ast.NewExpr:
		return ast.TypeRef{Name: expression.ClassName, GenericArguments: append([]ast.TypeRef(nil), expression.TypeArguments...), ResolvedDeclaration: expression.ResolvedDeclaration}, true
	case *ast.GoCompositeLiteralExpr:
		return expression.Type, true
	case *ast.ClassUpcastExpr:
		return ast.TypeRef{Name: expression.TargetClass, Span: expression.Span}, true
	case *ast.PropagateExpr:
		return expression.ValueType, expression.ValueType.IsSpecified()
	case *ast.AwaitExpr:
		return expression.ValueType, expression.ValueType.IsSpecified()
	case *ast.CallExpr:
		if ref, ok := callableDeclarationReturn(program, expression.Callee); ok {
			return ref, true
		}
		if expression.Signature != nil {
			return simpleSignatureType(expression.Signature.Result)
		}
	}
	return ast.TypeRef{}, false
}

func callableDeclarationReturn(program *ast.Program, callee ast.Expression) (ast.TypeRef, bool) {
	var declarationSpan source.Span
	var fallbackName string
	memberCall := false
	switch callee := callee.(type) {
	case *ast.IdentifierExpr:
		declarationSpan = callee.ResolvedDeclaration
		fallbackName = callee.Name
	case *ast.MemberExpr:
		declarationSpan = callee.ResolvedDeclaration
		fallbackName = callee.Name
		memberCall = true
	default:
		return ast.TypeRef{}, false
	}
	var fallback ast.TypeRef
	fallbackCount := 0
	for _, declaration := range program.Declarations {
		if function, ok := declaration.(*ast.FunctionDecl); ok {
			if declarationSpan.Path != "" && sameSourceSpan(declarationSpan, function.NameSpan) {
				return function.ReturnType, true
			}
			if declarationSpan.Path == "" && !memberCall && function.Name == fallbackName {
				return function.ReturnType, true
			}
		}
		var methods []*ast.MethodDecl
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			methods = declaration.Methods
		case *ast.StructDecl:
			methods = declaration.Methods
		case *ast.MethodDecl:
			methods = []*ast.MethodDecl{declaration}
		case *ast.InterfaceDecl:
			for _, method := range declaration.Methods {
				if declarationSpan.Path != "" && sameSourceSpan(declarationSpan, method.NameSpan) {
					return method.ReturnType, true
				}
				if memberCall && declarationSpan.Path == "" && method.Name == fallbackName {
					fallback, fallbackCount = method.ReturnType, fallbackCount+1
				}
			}
		}
		for _, method := range methods {
			if declarationSpan.Path != "" && sameSourceSpan(declarationSpan, method.NameSpan) {
				return method.ReturnType, true
			}
			if memberCall && declarationSpan.Path == "" && method.Name == fallbackName {
				fallback, fallbackCount = method.ReturnType, fallbackCount+1
			}
		}
	}
	if fallbackCount == 1 {
		return fallback, true
	}
	return ast.TypeRef{}, false
}

func simpleSignatureType(value string) (ast.TypeRef, bool) {
	if value == "" || value == "void" || value == "<invalid>" || strings.ContainsAny(value, "(),{}") {
		return ast.TypeRef{}, false
	}
	nullable := false
	if strings.HasSuffix(value, " | null") {
		nullable = true
		value = strings.TrimSuffix(value, " | null")
	}
	if strings.HasPrefix(value, "*") {
		pointee, ok := simpleSignatureType(strings.TrimPrefix(value, "*"))
		if !ok {
			return ast.TypeRef{}, false
		}
		return ast.TypeRef{Pointee: &pointee, Nullable: nullable}, true
	}
	if strings.ContainsAny(value, "[]<>") {
		return ast.TypeRef{}, false
	}
	return ast.TypeRef{Name: value, Nullable: nullable}, true
}

func visibleValueTypeInBlock(program *ast.Program, block *ast.BlockStmt, path string, offset int, name string) (ast.TypeRef, bool) {
	if block == nil || !spanContains(block.Span, path, offset) {
		return ast.TypeRef{}, false
	}
	var visible ast.TypeRef
	found := false
	for _, statement := range block.Statements {
		if statement.GetSpan().Start.Offset >= offset {
			break
		}
		if variable, ok := statement.(*ast.VariableDecl); ok && variable.Name == name {
			if ref, valid := variableType(program, variable); valid {
				visible, found = ref, true
			} else {
				visible, found = ast.TypeRef{}, true
			}
		}
		if multiple, ok := statement.(*ast.MultiVariableDecl); ok {
			for _, binding := range multiple.Bindings {
				if binding.Name == name {
					visible, found = binding.ResolvedType, true
				}
			}
		}
		if statement.GetSpan().End.Offset < offset {
			continue
		}
		if ref, ok := nestedVisibleValueType(program, statement, path, offset, name); ok {
			return ref, true
		}
	}
	return visible, found
}

func nestedVisibleValueType(program *ast.Program, statement ast.Statement, path string, offset int, name string) (ast.TypeRef, bool) {
	switch statement := statement.(type) {
	case *ast.LabeledStmt:
		return nestedVisibleValueType(program, statement.Statement, path, offset, name)
	case *ast.BlockStmt:
		return visibleValueTypeInBlock(program, statement, path, offset, name)
	case *ast.IfStmt:
		if spanContains(statement.Then.Span, path, offset) {
			return visibleValueTypeInBlock(program, statement.Then, path, offset, name)
		}
		if branch, ok := statement.Else.(*ast.BlockStmt); ok && spanContains(branch.Span, path, offset) {
			return visibleValueTypeInBlock(program, branch, path, offset, name)
		}
	case *ast.TryStmt:
		if spanContains(statement.Body.Span, path, offset) {
			return visibleValueTypeInBlock(program, statement.Body, path, offset, name)
		}
		for _, clause := range statement.Catches {
			if !spanContains(clause.Body.Span, path, offset) {
				continue
			}
			if clause.Name == name {
				return clause.Type, true
			}
			return visibleValueTypeInBlock(program, clause.Body, path, offset, name)
		}
		if statement.FinallyBody != nil && spanContains(statement.FinallyBody.Span, path, offset) {
			return visibleValueTypeInBlock(program, statement.FinallyBody, path, offset, name)
		}
	case *ast.WhileStmt:
		return visibleValueTypeInBlock(program, statement.Body, path, offset, name)
	case *ast.ForStmt:
		if variable, ok := statement.Initializer.(*ast.VariableDecl); ok && variable.Name == name {
			if ref, valid := variableType(program, variable); valid {
				return ref, true
			}
		}
		return visibleValueTypeInBlock(program, statement.Body, path, offset, name)
	case *ast.ForRangeStmt:
		if spanContains(statement.Body.Span, path, offset) {
			for _, binding := range statement.Bindings {
				if binding.Name == name {
					if binding.Type.IsSpecified() {
						return binding.Type, true
					}
					return binding.ResolvedType, true
				}
			}
			return visibleValueTypeInBlock(program, statement.Body, path, offset, name)
		}
	case *ast.SelectStmt:
		for index := range statement.Cases {
			clause := &statement.Cases[index]
			if spanContains(clause.Body.Span, path, offset) {
				if clause.Declare {
					for _, binding := range clause.Bindings {
						if binding.Name == name {
							return binding.ResolvedType, true
						}
					}
				}
				return visibleValueTypeInBlock(program, clause.Body, path, offset, name)
			}
		}
	case *ast.ValueSwitchStmt:
		for index := range statement.Cases {
			if spanContains(statement.Cases[index].Body.Span, path, offset) {
				return visibleValueTypeInBlock(program, statement.Cases[index].Body, path, offset, name)
			}
		}
	case *ast.TypeSwitchStmt:
		for index := range statement.Cases {
			clause := &statement.Cases[index]
			if !spanContains(clause.Body.Span, path, offset) {
				continue
			}
			if clause.Name == name && clause.Type.IsSpecified() {
				return clause.Type, true
			}
			return visibleValueTypeInBlock(program, clause.Body, path, offset, name)
		}
	}
	return ast.TypeRef{}, false
}

func collectTypeMemberCompletions(program *ast.Program, ref ast.TypeRef, owner string, static bool, seen map[string]bool, add func(completionItem)) {
	if ref.Nullable {
		ref.Nullable = false
	}
	if ref.IsPointer() && ref.Pointee != nil {
		ref = *ref.Pointee
	}
	if ref.IsObject() {
		if static {
			return
		}
		for _, field := range ref.ObjectFields {
			add(completionItem{Label: field.Name, Kind: 5, Detail: field.Name + ": " + formatTypeRef(field.Type), SortText: "0_" + field.Name})
		}
		return
	}
	if ref.Name == "Exception" {
		if !static {
			add(completionItem{Label: "message", Kind: 5, Detail: "public message: string", SortText: "0_message"})
			add(completionItem{Label: "error", Kind: 2, Detail: "public function error(): string", SortText: "0_error"})
		}
		return
	}
	if ref.Name == "error" {
		if !static {
			add(completionItem{Label: "Error", Kind: 2, Detail: "function Error(): string", SortText: "0_Error"})
		}
		return
	}
	if ref.Name == "" || seen[ref.Name] {
		return
	}
	seen[ref.Name] = true
	declaration := sourceTypeDeclaration(program, ref)
	switch declaration := declaration.(type) {
	case *ast.ClassDecl:
		bindings := genericClassTypeRefBindings(declaration, ref)
		if declaration.Base != nil {
			collectTypeMemberCompletions(program, *declaration.Base, owner, static, seen, add)
		}
		for _, field := range declaration.Fields {
			if !static && memberVisible(program, owner, declaration.Name, field.Visibility) {
				fieldType := substituteTypeRefParameters(field.Type, bindings)
				add(completionItem{Label: field.Name, Kind: 5, Detail: visibilityName(field.Visibility) + " " + field.Name + ": " + formatTypeRef(fieldType), SortText: "0_" + field.Name})
			}
		}
		if declaration.Constructor != nil {
			for _, parameter := range declaration.Constructor.Parameters {
				if parameter.IsField && !static && memberVisible(program, owner, declaration.Name, parameter.Visibility) {
					parameterType := substituteTypeRefParameters(parameter.Type, bindings)
					add(completionItem{Label: parameter.Name, Kind: 5, Detail: visibilityName(parameter.Visibility) + " " + parameter.Name + ": " + formatTypeRef(parameterType), SortText: "0_" + parameter.Name})
				}
			}
		}
		for _, method := range declaration.Methods {
			if method.Static != static || !memberVisible(program, owner, declaration.Name, method.Visibility) {
				continue
			}
			parameters := append([]ast.Parameter(nil), method.Parameters...)
			for index := range parameters {
				parameters[index].Type = substituteTypeRefParameters(parameters[index].Type, bindings)
			}
			result := substituteTypeRefParameters(method.ReturnType, bindings)
			add(completionItem{Label: method.Name, Kind: 2, Detail: visibilityName(method.Visibility) + " " + functionDetail(method.Name, parameters, result), SortText: "0_" + method.Name})
		}
	case *ast.StructDecl:
		if static {
			return
		}
		bindings := genericStructTypeRefBindings(declaration, ref)
		for _, field := range declaration.Fields {
			if memberVisible(program, owner, declaration.Name, field.Visibility) {
				fieldType := substituteTypeRefParameters(field.Type, bindings)
				add(completionItem{Label: field.Name, Kind: 5, Detail: visibilityName(field.Visibility) + " " + field.Name + ": " + formatTypeRef(fieldType), SortText: "0_" + field.Name})
			}
		}
		for _, method := range declaration.Methods {
			if memberVisible(program, owner, declaration.Name, method.Visibility) {
				parameters := append([]ast.Parameter(nil), method.Parameters...)
				for index := range parameters {
					parameters[index].Type = substituteTypeRefParameters(parameters[index].Type, bindings)
				}
				result := substituteTypeRefParameters(method.ReturnType, bindings)
				add(completionItem{Label: method.Name, Kind: 2, Detail: visibilityName(method.Visibility) + " " + functionDetail(method.Name, parameters, result), SortText: "0_" + method.Name})
			}
		}
		for _, candidate := range program.Declarations {
			method, ok := candidate.(*ast.MethodDecl)
			if !ok || !method.External {
				continue
			}
			receiver := method.ReceiverType
			if receiver.IsPointer() && receiver.Pointee != nil {
				receiver = *receiver.Pointee
			}
			if receiver.Name == declaration.Name && memberVisible(program, owner, declaration.Name, method.Visibility) {
				methodBindings := externalReceiverTypeRefBindings(receiver, ref)
				parameters := append([]ast.Parameter(nil), method.Parameters...)
				for index := range parameters {
					parameters[index].Type = substituteTypeRefParameters(parameters[index].Type, methodBindings)
				}
				result := substituteTypeRefParameters(method.ReturnType, methodBindings)
				add(completionItem{Label: method.Name, Kind: 2, Detail: visibilityName(method.Visibility) + " " + functionDetail(method.Name, parameters, result), SortText: "0_" + method.Name})
			}
		}
	case *ast.TypeDecl:
		if static || declaration.Alias {
			return
		}
		for _, candidate := range program.Declarations {
			method, ok := candidate.(*ast.MethodDecl)
			if !ok || !method.External {
				continue
			}
			receiver := method.ReceiverType
			if receiver.IsPointer() && receiver.Pointee != nil {
				receiver = *receiver.Pointee
			}
			if receiver.Name == declaration.Name && memberVisible(program, owner, declaration.Name, method.Visibility) {
				methodBindings := externalReceiverTypeRefBindings(receiver, ref)
				parameters := append([]ast.Parameter(nil), method.Parameters...)
				for index := range parameters {
					parameters[index].Type = substituteTypeRefParameters(parameters[index].Type, methodBindings)
				}
				result := substituteTypeRefParameters(method.ReturnType, methodBindings)
				add(completionItem{Label: method.Name, Kind: 2, Detail: visibilityName(method.Visibility) + " " + functionDetail(method.Name, parameters, result), SortText: "0_" + method.Name})
			}
		}
	case *ast.EnumDecl:
		if static {
			for _, member := range declaration.Members {
				add(completionItem{Label: member.Name, Kind: 20, Detail: declaration.Name + "." + member.Name, SortText: "0_" + member.Name})
			}
			return
		}
		for _, candidate := range program.Declarations {
			method, ok := candidate.(*ast.MethodDecl)
			if !ok || !method.External {
				continue
			}
			receiver := method.ReceiverType
			if receiver.IsPointer() && receiver.Pointee != nil {
				receiver = *receiver.Pointee
			}
			if receiver.Name == declaration.Name && memberVisible(program, owner, declaration.Name, method.Visibility) {
				parameters := append([]ast.Parameter(nil), method.Parameters...)
				add(completionItem{Label: method.Name, Kind: 2, Detail: visibilityName(method.Visibility) + " " + functionDetail(method.Name, parameters, method.ReturnType), SortText: "0_" + method.Name})
			}
		}
	case *ast.InterfaceDecl:
		if static {
			return
		}
		bindings := genericInterfaceTypeRefBindings(declaration, ref)
		for _, method := range declaration.Methods {
			parameters := append([]ast.Parameter(nil), method.Parameters...)
			for index := range parameters {
				parameters[index].Type = substituteTypeRefParameters(parameters[index].Type, bindings)
			}
			result := substituteTypeRefParameters(method.ReturnType, bindings)
			add(completionItem{Label: method.Name, Kind: 2, Detail: functionDetail(method.Name, parameters, result), SortText: "0_" + method.Name})
		}
	}
}

func externalReceiverTypeRefBindings(receiver ast.TypeRef, selected ast.TypeRef) map[string]ast.TypeRef {
	bindings := map[string]ast.TypeRef{}
	for index, argument := range receiver.GenericArguments {
		if index >= len(selected.GenericArguments) || argument.Name == "" || argument.Qualifier != "" {
			continue
		}
		bindings[argument.Name] = selected.GenericArguments[index]
	}
	return bindings
}

func genericStructTypeRefBindings(declaration *ast.StructDecl, instantiated ast.TypeRef) map[string]ast.TypeRef {
	if len(declaration.TypeParameters) == 0 || len(declaration.TypeParameters) != len(instantiated.GenericArguments) {
		return nil
	}
	bindings := make(map[string]ast.TypeRef, len(declaration.TypeParameters))
	for index, parameter := range declaration.TypeParameters {
		bindings[parameter.Name] = instantiated.GenericArguments[index]
	}
	return bindings
}

func genericClassTypeRefBindings(declaration *ast.ClassDecl, instantiated ast.TypeRef) map[string]ast.TypeRef {
	if len(declaration.TypeParameters) == 0 || len(declaration.TypeParameters) != len(instantiated.GenericArguments) {
		return nil
	}
	bindings := make(map[string]ast.TypeRef, len(declaration.TypeParameters))
	for index, parameter := range declaration.TypeParameters {
		bindings[parameter.Name] = instantiated.GenericArguments[index]
	}
	return bindings
}

func genericInterfaceTypeRefBindings(declaration *ast.InterfaceDecl, instantiated ast.TypeRef) map[string]ast.TypeRef {
	if len(declaration.TypeParameters) == 0 || len(declaration.TypeParameters) != len(instantiated.GenericArguments) {
		return nil
	}
	bindings := make(map[string]ast.TypeRef, len(declaration.TypeParameters))
	for index, parameter := range declaration.TypeParameters {
		bindings[parameter.Name] = instantiated.GenericArguments[index]
	}
	return bindings
}

func substituteTypeRefParameters(ref ast.TypeRef, bindings map[string]ast.TypeRef) ast.TypeRef {
	if ref.Qualifier == "" && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() && len(ref.GenericArguments) == 0 {
		if replacement, ok := bindings[ref.Name]; ok {
			return replacement
		}
	}
	result := ref
	result.GenericArguments = append([]ast.TypeRef(nil), ref.GenericArguments...)
	for index := range result.GenericArguments {
		result.GenericArguments[index] = substituteTypeRefParameters(result.GenericArguments[index], bindings)
	}
	if ref.Element != nil {
		element := substituteTypeRefParameters(*ref.Element, bindings)
		result.Element = &element
	}
	if ref.Pointee != nil {
		pointee := substituteTypeRefParameters(*ref.Pointee, bindings)
		result.Pointee = &pointee
	}
	result.Parameters = append([]ast.TypeRef(nil), ref.Parameters...)
	for index := range result.Parameters {
		result.Parameters[index] = substituteTypeRefParameters(result.Parameters[index], bindings)
	}
	if ref.Return != nil {
		returnType := substituteTypeRefParameters(*ref.Return, bindings)
		result.Return = &returnType
	}
	result.ObjectFields = append([]ast.ObjectTypeField(nil), ref.ObjectFields...)
	for index := range result.ObjectFields {
		result.ObjectFields[index].Type = substituteTypeRefParameters(result.ObjectFields[index].Type, bindings)
	}
	return result
}

func sourceNamedType(program *ast.Program, name string) ast.Declaration {
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.ClassDecl:
			if declaration.Name == name {
				return declaration
			}
		case *ast.StructDecl:
			if declaration.Name == name {
				return declaration
			}
		case *ast.InterfaceDecl:
			if declaration.Name == name {
				return declaration
			}
		case *ast.TypeDecl:
			if declaration.Name == name {
				return declaration
			}
		case *ast.EnumDecl:
			if declaration.Name == name {
				return declaration
			}
		}
	}
	return nil
}

func sourceTypeDeclaration(program *ast.Program, ref ast.TypeRef) ast.Declaration {
	if ref.ResolvedDeclaration.Path != "" {
		for _, declaration := range program.Declarations {
			matched := false
			switch declaration := declaration.(type) {
			case *ast.ClassDecl:
				matched = sameSourceSpan(ref.ResolvedDeclaration, declaration.NameSpan)
			case *ast.StructDecl:
				matched = sameSourceSpan(ref.ResolvedDeclaration, declaration.NameSpan)
			case *ast.InterfaceDecl:
				matched = sameSourceSpan(ref.ResolvedDeclaration, declaration.NameSpan)
			case *ast.TypeDecl:
				matched = sameSourceSpan(ref.ResolvedDeclaration, declaration.NameSpan)
			case *ast.EnumDecl:
				matched = sameSourceSpan(ref.ResolvedDeclaration, declaration.NameSpan)
			default:
				continue
			}
			if matched {
				return declaration
			}
		}
	}
	if ref.Span.Path != "" {
		return sourceVisibleNamedType(program, ref.Span.Path, ref.Name)
	}
	return sourceNamedType(program, ref.Name)
}

func typeRefForDeclaration(declaration ast.Declaration) ast.TypeRef {
	switch declaration := declaration.(type) {
	case *ast.ClassDecl:
		return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan, Span: declaration.NameSpan}
	case *ast.StructDecl:
		return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan, Span: declaration.NameSpan}
	case *ast.InterfaceDecl:
		return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan, Span: declaration.NameSpan}
	case *ast.TypeDecl:
		return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan, Span: declaration.NameSpan, NativeNamed: !declaration.Alias}
	case *ast.EnumDecl:
		return ast.TypeRef{Name: declaration.Name, ResolvedDeclaration: declaration.NameSpan, Span: declaration.NameSpan, NativeNamed: true}
	default:
		return ast.TypeRef{}
	}
}

func sourceVisibleNamedType(program *ast.Program, path, name string) ast.Declaration {
	for _, declaration := range program.Declarations {
		if samePath(declaration.GetSpan().Path, path) {
			switch declaration := declaration.(type) {
			case *ast.ClassDecl:
				if declaration.Name == name {
					return declaration
				}
			case *ast.StructDecl:
				if declaration.Name == name {
					return declaration
				}
			case *ast.InterfaceDecl:
				if declaration.Name == name {
					return declaration
				}
			case *ast.TypeDecl:
				if declaration.Name == name {
					return declaration
				}
			case *ast.EnumDecl:
				if declaration.Name == name {
					return declaration
				}
			}
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
				if samePath(declaration.GetSpan().Path, imported.ResolvedPath) {
					switch declaration := declaration.(type) {
					case *ast.ClassDecl:
						if declaration.Name == name {
							return declaration
						}
					case *ast.StructDecl:
						if declaration.Name == name {
							return declaration
						}
					case *ast.InterfaceDecl:
						if declaration.Name == name {
							return declaration
						}
					case *ast.TypeDecl:
						if declaration.Name == name {
							return declaration
						}
					case *ast.EnumDecl:
						if declaration.Name == name {
							return declaration
						}
					}
				}
			}
		}
	}
	return nil
}

func memberVisible(program *ast.Program, current, declaring string, visibility ast.Visibility) bool {
	switch visibility {
	case ast.Public:
		return true
	case ast.Private:
		return current == declaring
	case ast.Protected:
		return current == declaring || sourceClassExtends(program, current, declaring, map[string]bool{})
	default:
		return false
	}
}

func sourceClassExtends(program *ast.Program, className, baseName string, seen map[string]bool) bool {
	if className == "" || className == baseName || seen[className] {
		return className == baseName
	}
	seen[className] = true
	declaration, ok := sourceNamedType(program, className).(*ast.ClassDecl)
	if !ok || declaration.Base == nil {
		return false
	}
	return declaration.Base.Name == baseName || sourceClassExtends(program, declaration.Base.Name, baseName, seen)
}

func visibilityName(visibility ast.Visibility) string {
	switch visibility {
	case ast.Public:
		return "public"
	case ast.Protected:
		return "protected"
	default:
		return "private"
	}
}
