package compiler

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type sourceLinker struct {
	unimported map[source.Span]bool
	// A discovery pass uses the same lexical traversal with identity mappings.
	// Reserve bindings before choosing cross-module implementation names.
	lexicalNames map[string]bool
	diagnostics  *[]diagnostic.Diagnostic
}

const typeParameterLinkBinding = "\x00type-parameter"

func (linker *sourceLinker) capture(name, linked string, span source.Span) {
	if linker.diagnostics != nil {
		*linker.diagnostics = append(*linker.diagnostics, diagnostic.Diagnostic{
			Message: fmt.Sprintf("imported reference %q would be captured by local binding %q; rename that binding", name, linked), Span: span,
		})
	}
}

func (linker *sourceLinker) name(name string, span source.Span, visible moduleNames) string {
	if linked, exists := visible[name]; exists {
		if linked == typeParameterLinkBinding {
			return name
		}
		if linked == "" {
			linker.unimported[span] = true
			return name
		}
		if linked != name && visible[linked] == typeParameterLinkBinding {
			linker.capture(name, linked, span)
		}
		return linked
	}
	return name
}

func (linker *sourceLinker) linkProgram(program *ast.Program, declarations, visible moduleNames) {
	for _, exported := range program.Exports {
		for i := range exported.Names {
			if linked, ok := declarations[exported.Names[i].Name]; ok && exported.Names[i].ResolvedName == "" {
				exported.Names[i].ResolvedName = linked
			}
		}
	}
	for _, declaration := range program.Declarations {
		linker.linkDeclaration(declaration, declarations, visible)
	}
}

func (linker *sourceLinker) linkDeclaration(declaration ast.Declaration, declarations, visible moduleNames) {
	switch declaration := declaration.(type) {
	case *ast.CABIExportDecl:
		for index, name := range declaration.Names {
			if linked, ok := declarations[name]; ok {
				declaration.Names[index] = linked
			} else if linked, ok := visible[name]; ok {
				declaration.Names[index] = linked
			}
		}
	case *ast.VariableDecl:
		original := declaration.Name
		linker.linkType(&declaration.Type, visible)
		linker.linkExpression(declaration.Value, visible, nil)
		declaration.Name = declarations[original]
	case *ast.FunctionDecl:
		original := declaration.Name
		functionVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			functionVisible[parameter.Name] = typeParameterLinkBinding
		}
		linker.linkTypeParameters(declaration.TypeParameters, functionVisible)
		locals := linker.parameterNames(declaration.Parameters)
		for i := range declaration.Parameters {
			linker.linkType(&declaration.Parameters[i].Type, functionVisible)
		}
		linker.linkType(&declaration.ReturnType, functionVisible)
		linker.linkBlock(declaration.Body, functionVisible, locals)
		declaration.Name = declarations[original]
	case *ast.MethodDecl:
		methodVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			methodVisible[parameter.Name] = typeParameterLinkBinding
		}
		linker.linkTypeParameters(declaration.TypeParameters, methodVisible)
		locals := linker.parameterNames(declaration.Parameters)
		linker.bindLocal(locals, declaration.ReceiverName)
		linker.linkType(&declaration.ReceiverType, methodVisible)
		for i := range declaration.Parameters {
			linker.linkType(&declaration.Parameters[i].Type, methodVisible)
		}
		linker.linkType(&declaration.ReturnType, methodVisible)
		linker.linkBlock(declaration.Body, methodVisible, locals)
	case *ast.ClassDecl:
		original := declaration.Name
		if declaration.SourceName == "" && linker.lexicalNames == nil {
			declaration.SourceName = original
		}
		linker.linkClassDecorators(declaration, visible)
		classVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			classVisible[parameter.Name] = typeParameterLinkBinding
		}
		linker.linkTypeParameters(declaration.TypeParameters, classVisible)
		if declaration.Base != nil {
			linker.linkType(declaration.Base, classVisible)
		}
		for i := range declaration.Implements {
			linker.linkType(&declaration.Implements[i], classVisible)
		}
		for i := range declaration.Fields {
			fieldVisible := classVisible
			if declaration.Fields[i].Static {
				fieldVisible = visible
			}
			linker.linkType(&declaration.Fields[i].Type, fieldVisible)
			linker.linkExpression(declaration.Fields[i].Initializer, fieldVisible, map[string]bool{})
		}
		if declaration.Constructor != nil {
			locals := linker.parameterNames(declaration.Constructor.Parameters)
			locals["this"] = true
			for i := range declaration.Constructor.Parameters {
				linker.linkType(&declaration.Constructor.Parameters[i].Type, classVisible)
			}
			linker.linkBlock(declaration.Constructor.Body, classVisible, locals)
		}
		for _, method := range declaration.Methods {
			methodVisible := cloneModuleNames(classVisible)
			if method.Static && method.Accessor != "" {
				methodVisible = cloneModuleNames(visible)
			}
			for _, parameter := range method.TypeParameters {
				methodVisible[parameter.Name] = typeParameterLinkBinding
			}
			linker.linkTypeParameters(method.TypeParameters, methodVisible)
			locals := linker.parameterNames(method.Parameters)
			if !method.Static {
				locals["this"] = true
			}
			for i := range method.Parameters {
				linker.linkType(&method.Parameters[i].Type, methodVisible)
			}
			linker.linkType(&method.ReturnType, methodVisible)
			linker.linkBlock(method.Body, methodVisible, locals)
		}
		declaration.Name = declarations[original]
	case *ast.StructDecl:
		original := declaration.Name
		structVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			structVisible[parameter.Name] = typeParameterLinkBinding
		}
		linker.linkTypeParameters(declaration.TypeParameters, structVisible)
		for i := range declaration.Fields {
			linker.linkType(&declaration.Fields[i].Type, structVisible)
		}
		for _, method := range declaration.Methods {
			methodVisible := cloneModuleNames(structVisible)
			for _, parameter := range method.TypeParameters {
				methodVisible[parameter.Name] = typeParameterLinkBinding
			}
			linker.linkTypeParameters(method.TypeParameters, methodVisible)
			locals := linker.parameterNames(method.Parameters)
			locals["this"] = true
			for i := range method.Parameters {
				linker.linkType(&method.Parameters[i].Type, methodVisible)
			}
			linker.linkType(&method.ReturnType, methodVisible)
			linker.linkBlock(method.Body, methodVisible, locals)
		}
		declaration.Name = declarations[original]
	case *ast.TypeDecl:
		original := declaration.Name
		typeVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			typeVisible[parameter.Name] = typeParameterLinkBinding
		}
		linker.linkTypeParameters(declaration.TypeParameters, typeVisible)
		linker.linkType(&declaration.Underlying, typeVisible)
		declaration.Name = declarations[original]
	case *ast.EnumDecl:
		original := declaration.Name
		linker.linkType(&declaration.Underlying, visible)
		for index := range declaration.Members {
			linker.linkExpression(declaration.Members[index].Value, visible, nil)
		}
		declaration.Name = declarations[original]
	case *ast.InterfaceDecl:
		original := declaration.Name
		interfaceVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			interfaceVisible[parameter.Name] = typeParameterLinkBinding
		}
		linker.linkTypeParameters(declaration.TypeParameters, interfaceVisible)
		for index := range declaration.Bases {
			linker.linkType(&declaration.Bases[index], interfaceVisible)
		}
		for index := range declaration.Terms {
			linker.linkType(&declaration.Terms[index].Type, interfaceVisible)
		}
		for i := range declaration.Methods {
			for j := range declaration.Methods[i].Parameters {
				linker.linkType(&declaration.Methods[i].Parameters[j].Type, interfaceVisible)
			}
			linker.linkType(&declaration.Methods[i].ReturnType, interfaceVisible)
		}
		declaration.Name = declarations[original]
	}
}

func (linker *sourceLinker) linkTypeParameters(parameters []ast.TypeParameter, visible moduleNames) {
	for index := range parameters {
		if linker.lexicalNames != nil {
			linker.lexicalNames[parameters[index].Name] = true
		}
		if parameters[index].Constraint != nil {
			linker.linkType(parameters[index].Constraint, visible)
		}
	}
}

func (linker *sourceLinker) bindLocal(locals map[string]bool, name string) {
	locals[name] = true
	if linker.lexicalNames != nil && name != "_" {
		linker.lexicalNames[name] = true
	}
}

func (linker *sourceLinker) parameterNames(parameters []ast.Parameter) map[string]bool {
	result := map[string]bool{}
	for _, parameter := range parameters {
		linker.bindLocal(result, parameter.Name)
	}
	return result
}

func cloneNames(names map[string]bool) map[string]bool {
	result := make(map[string]bool, len(names))
	for name := range names {
		result[name] = true
	}
	return result
}

func (linker *sourceLinker) linkBlock(block *ast.BlockStmt, visible moduleNames, inherited map[string]bool) {
	if block == nil {
		return
	}
	locals := cloneNames(inherited)
	groupEnd := 0
	for index, statement := range block.Statements {
		if index >= groupEnd {
			group := ast.LocalArrowGroup(block.Statements[index:])
			groupEnd = index + len(group)
			for _, declaration := range group {
				linker.bindLocal(locals, declaration.Name)
			}
		}
		linker.linkStatement(statement, visible, locals)
		if variable, ok := statement.(*ast.VariableDecl); ok {
			linker.bindLocal(locals, variable.Name)
		}
		if declaration, ok := statement.(*ast.MultiVariableDecl); ok {
			for _, binding := range declaration.Bindings {
				if binding.Name != "_" {
					linker.bindLocal(locals, binding.Name)
				}
			}
		}
	}
}

func (linker *sourceLinker) linkStatement(statement ast.Statement, visible moduleNames, locals map[string]bool) {
	switch statement := statement.(type) {
	case *ast.VariableDecl:
		linker.linkType(&statement.Type, visible)
		if _, arrow := statement.Value.(*ast.ArrowExpr); arrow {
			locals = cloneNames(locals)
			linker.bindLocal(locals, statement.Name)
		}
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.MultiVariableDecl:
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.BlockStmt:
		linker.linkBlock(statement, visible, locals)
	case *ast.ReturnStmt:
		linker.linkExpression(statement.Value, visible, locals)
		for _, value := range statement.AdditionalValues {
			linker.linkExpression(value, visible, locals)
		}
	case *ast.ThrowStmt:
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.TryStmt:
		linker.linkBlock(statement.Body, visible, locals)
		for _, clause := range statement.Catches {
			linker.linkType(&clause.Type, visible)
			catchLocals := cloneNames(locals)
			if clause.Name != "_" {
				linker.bindLocal(catchLocals, clause.Name)
			}
			linker.linkBlock(clause.Body, visible, catchLocals)
		}
		linker.linkBlock(statement.FinallyBody, visible, locals)
	case *ast.IfStmt:
		linker.linkExpression(statement.Condition, visible, locals)
		linker.linkBlock(statement.Then, visible, locals)
		if statement.Else != nil {
			linker.linkStatement(statement.Else, visible, cloneNames(locals))
		}
	case *ast.ExpressionStmt:
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.AssignmentStmt:
		linker.linkExpression(statement.Target, visible, locals)
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.IncDecStmt:
		linker.linkExpression(statement.Target, visible, locals)
	case *ast.MultiAssignmentStmt:
		for i := range statement.Bindings {
			binding := &statement.Bindings[i]
			if binding.Name != "_" && !locals[binding.Name] {
				binding.Name = linker.name(binding.Name, binding.Span, visible)
			}
		}
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.WhileStmt:
		linker.linkExpression(statement.Condition, visible, locals)
		linker.linkBlock(statement.Body, visible, locals)
	case *ast.ForStmt:
		loopLocals := cloneNames(locals)
		if statement.Initializer != nil {
			linker.linkStatement(statement.Initializer, visible, loopLocals)
			if variable, ok := statement.Initializer.(*ast.VariableDecl); ok {
				linker.bindLocal(loopLocals, variable.Name)
			}
			if declaration, ok := statement.Initializer.(*ast.MultiVariableDecl); ok {
				for _, binding := range declaration.Bindings {
					if binding.Name != "_" {
						linker.bindLocal(loopLocals, binding.Name)
					}
				}
			}
		}
		linker.linkExpression(statement.Condition, visible, loopLocals)
		linker.linkBlock(statement.Body, visible, loopLocals)
		if statement.Post != nil {
			linker.linkStatement(statement.Post, visible, loopLocals)
		}
	case *ast.ForRangeStmt:
		for index := range statement.Bindings {
			linker.linkType(&statement.Bindings[index].Type, visible)
		}
		linker.linkExpression(statement.Source, visible, locals)
		loopLocals := cloneNames(locals)
		for _, binding := range statement.Bindings {
			if binding.Name != "_" {
				linker.bindLocal(loopLocals, binding.Name)
			}
		}
		linker.linkBlock(statement.Body, visible, loopLocals)
	case *ast.SelectStmt:
		for i := range statement.Cases {
			clause := &statement.Cases[i]
			linker.linkExpression(clause.Channel, visible, locals)
			linker.linkExpression(clause.Value, visible, locals)
			for _, target := range clause.Targets {
				linker.linkExpression(target, visible, locals)
			}
			caseLocals := cloneNames(locals)
			if clause.Declare {
				for _, binding := range clause.Bindings {
					if binding.Name != "_" {
						linker.bindLocal(caseLocals, binding.Name)
					}
				}
			}
			linker.linkBlock(clause.Body, visible, caseLocals)
		}
	case *ast.ValueSwitchStmt:
		linker.linkExpression(statement.Value, visible, locals)
		for i := range statement.Cases {
			clause := &statement.Cases[i]
			for _, value := range clause.Values {
				linker.linkExpression(value, visible, locals)
			}
			linker.linkBlock(clause.Body, visible, cloneNames(locals))
		}
	case *ast.TypeSwitchStmt:
		linker.linkExpression(statement.Value, visible, locals)
		for i := range statement.Cases {
			clause := &statement.Cases[i]
			linker.linkType(&clause.Type, visible)
			caseLocals := cloneNames(locals)
			if !clause.Nil && !clause.Default && clause.Name != "_" {
				linker.bindLocal(caseLocals, clause.Name)
			}
			linker.linkBlock(clause.Body, visible, caseLocals)
		}
	case *ast.CallControlStmt:
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.DetachStmt:
		linker.linkExpression(statement.Value, visible, locals)
	case *ast.ChannelSendStmt:
		linker.linkExpression(statement.Channel, visible, locals)
		linker.linkExpression(statement.Value, visible, locals)
	}
}

func (linker *sourceLinker) linkExpression(expression ast.Expression, visible moduleNames, locals map[string]bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		if !locals[expression.Name] {
			linked := linker.name(expression.Name, expression.Span, visible)
			if linked != expression.Name && locals[linked] {
				linker.capture(expression.Name, linked, expression.Span)
			}
			expression.Name = linked
		}
	case *ast.UnaryExpr:
		linker.linkExpression(expression.Operand, visible, locals)
	case *ast.PropagateExpr:
		linker.linkExpression(expression.Value, visible, locals)
	case *ast.TaskStartExpr:
		linker.linkExpression(expression.Call, visible, locals)
	case *ast.AwaitExpr:
		linker.linkExpression(expression.Value, visible, locals)
	case *ast.BinaryExpr:
		linker.linkExpression(expression.Left, visible, locals)
		linker.linkExpression(expression.Right, visible, locals)
	case *ast.CallExpr:
		linker.linkExpression(expression.Callee, visible, locals)
		for i := range expression.TypeArguments {
			linker.linkType(&expression.TypeArguments[i], visible)
		}
		for _, argument := range expression.Arguments {
			linker.linkExpression(argument, visible, locals)
		}
	case *ast.ArrowExpr:
		arrowLocals := cloneNames(locals)
		for i := range expression.Parameters {
			linker.linkType(&expression.Parameters[i].Type, visible)
			linker.bindLocal(arrowLocals, expression.Parameters[i].Name)
		}
		linker.linkType(expression.ReturnType, visible)
		linker.linkExpression(expression.ExpressionBody, visible, arrowLocals)
		linker.linkBlock(expression.BlockBody, visible, arrowLocals)
	case *ast.ArrayLiteralExpr:
		for _, element := range expression.Elements {
			linker.linkExpression(element, visible, locals)
		}
	case *ast.ObjectLiteralExpr:
		for _, field := range expression.Fields {
			linker.linkExpression(field.Value, visible, locals)
		}
	case *ast.GoCompositeLiteralExpr:
		linker.linkType(&expression.Type, visible)
		for _, field := range expression.Fields {
			linker.linkExpression(field.Value, visible, locals)
		}
	case *ast.MemberExpr:
		linker.linkExpression(expression.Object, visible, locals)
	case *ast.IndexExpr:
		linker.linkExpression(expression.Object, visible, locals)
		linker.linkExpression(expression.Index, visible, locals)
	case *ast.SliceExpr:
		linker.linkExpression(expression.Object, visible, locals)
		linker.linkExpression(expression.Low, visible, locals)
		linker.linkExpression(expression.High, visible, locals)
		linker.linkExpression(expression.Max, visible, locals)
	case *ast.NewExpr:
		expression.ClassName = linker.name(expression.ClassName, expression.Span, visible)
		for index := range expression.TypeArguments {
			linker.linkType(&expression.TypeArguments[index], visible)
		}
		for _, argument := range expression.Arguments {
			linker.linkExpression(argument, visible, locals)
		}
	case *ast.ClassUpcastExpr:
		linker.linkExpression(expression.Value, visible, locals)
	}
}

func cloneModuleNames(values moduleNames) moduleNames {
	cloned := make(moduleNames, len(values))
	for name, linked := range values {
		cloned[name] = linked
	}
	return cloned
}

func (linker *sourceLinker) linkType(ref *ast.TypeRef, visible moduleNames) {
	if ref == nil {
		return
	}
	// A qualified name belongs to the Go package, not the source module's
	// alias namespace. Only its qualifier participates in module linking.
	if ref.Qualifier == "" {
		ref.Name = linker.name(ref.Name, ref.Span, visible)
	}
	ref.Qualifier = linker.name(ref.Qualifier, ref.Span, visible)
	for i := range ref.GenericArguments {
		linker.linkType(&ref.GenericArguments[i], visible)
	}
	linker.linkType(ref.Element, visible)
	linker.linkType(ref.Pointee, visible)
	for i := range ref.Parameters {
		linker.linkType(&ref.Parameters[i], visible)
	}
	linker.linkType(ref.Return, visible)
	for i := range ref.GoResults {
		linker.linkType(&ref.GoResults[i], visible)
	}
	for i := range ref.ObjectFields {
		linker.linkType(&ref.ObjectFields[i].Type, visible)
	}
}
