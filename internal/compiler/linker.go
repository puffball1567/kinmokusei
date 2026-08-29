package compiler

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"ontama.local/ontama/internal/ast"
	"ontama.local/ontama/internal/diagnostic"
	"ontama.local/ontama/internal/source"
)

type moduleNames map[string]string

func (l *moduleLoader) linkModules(rootPaths []string) map[string]map[string]bool {
	roots := map[string]bool{}
	for _, path := range rootPaths {
		absolute, err := filepath.Abs(path)
		if err == nil {
			roots[filepath.Clean(absolute)] = true
		}
	}

	paths := make([]string, 0, len(l.programs))
	nameCounts := map[string]int{}
	for path, program := range l.programs {
		paths = append(paths, path)
		for _, declaration := range program.Declarations {
			if name := topLevelName(declaration); name != "" {
				nameCounts[name]++
			}
		}
	}
	sort.Strings(paths)

	bindings := map[string]moduleNames{}
	for _, path := range paths {
		bindings[path] = moduleNames{}
		for _, declaration := range l.programs[path].Declarations {
			name := topLevelName(declaration)
			if name == "" {
				continue
			}
			linked := name
			if nameCounts[name] > 1 && !roots[path] {
				linked = l.linkedModuleName(path, name)
			}
			bindings[path][name] = linked
		}
	}
	goAliasBindings, canonicalGoAliases := l.linkGoAliases(paths, bindings)

	allowed := map[string]map[string]bool{}
	for _, path := range paths {
		program := l.programs[path]
		moduleBindings := moduleNames{}
		moduleAllowed := map[string]bool{}
		for sourceName, linked := range bindings[path] {
			moduleBindings[sourceName] = linked
			moduleAllowed[linked] = true
		}
		for sourceAlias, linked := range goAliasBindings[path] {
			moduleBindings[sourceAlias] = linked
		}
		seenImports := map[string]bool{}
		for sourceAlias := range goAliasBindings[path] {
			if _, local := bindings[path][sourceAlias]; local {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
					Message: fmt.Sprintf("Go package alias %q conflicts with a declaration in the same module", sourceAlias), Span: importAliasSpan(program, sourceAlias),
				})
			}
			seenImports[sourceAlias] = true
		}
		for _, imported := range program.Imports {
			if imported.Go {
				continue
			}
			if imported.ResolvedPath == "" {
				continue
			}
			targetBindings := bindings[imported.ResolvedPath]
			for _, name := range imported.Names {
				if _, local := bindings[path][name]; local {
					l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{
						Message: fmt.Sprintf("imported name %q conflicts with a declaration in the same module", name), Span: imported.Span,
					})
					continue
				}
				if seenImports[name] {
					l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate import binding %q", name), Span: imported.Span})
					continue
				}
				seenImports[name] = true
				if linked, exists := targetBindings[name]; exists {
					moduleBindings[name] = linked
					moduleAllowed[linked] = true
				}
			}
		}
		linkProgram(program, bindings[path], moduleBindings)
		allowed[l.paths[path]] = moduleAllowed
	}
	for i := range l.merged.Imports {
		if imported := &l.merged.Imports[i]; imported.Go {
			imported.ResolvedAlias = canonicalGoAliases[imported.Path]
		}
	}
	return allowed
}

func (l *moduleLoader) linkGoAliases(paths []string, declarationBindings map[string]moduleNames) (map[string]moduleNames, map[string]string) {
	bindings := map[string]moduleNames{}
	canonical := map[string]string{}
	usedAliases := map[string]string{
		"bool": "<Go built-in>", "string": "<Go built-in>", "int": "<Go built-in>", "int32": "<Go built-in>",
		"int64": "<Go built-in>", "float32": "<Go built-in>", "float64": "<Go built-in>", "byte": "<Go built-in>",
	}
	for _, moduleBindings := range declarationBindings {
		for _, linked := range moduleBindings {
			usedAliases[linked] = "<language declaration>"
		}
	}
	for _, path := range paths {
		bindings[path] = moduleNames{}
		seenPaths := map[string]bool{}
		seenAliases := map[string]bool{}
		for i := range l.programs[path].Imports {
			imported := &l.programs[path].Imports[i]
			if !imported.Go {
				continue
			}
			if isReservedLanguageTypeName(imported.Alias) {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("Go package alias %q conflicts with a built-in type", imported.Alias), Span: imported.Span})
				continue
			}
			if seenPaths[imported.Path] {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate Go package import %q", imported.Path), Span: imported.Span})
				continue
			}
			seenPaths[imported.Path] = true
			if seenAliases[imported.Alias] {
				l.diagnostics = append(l.diagnostics, diagnostic.Diagnostic{Message: fmt.Sprintf("duplicate import binding %q", imported.Alias), Span: imported.Span})
				continue
			}
			seenAliases[imported.Alias] = true
			linked, exists := canonical[imported.Path]
			if !exists {
				linked = imported.Alias
				if previousPath, used := usedAliases[linked]; used && previousPath != imported.Path {
					linked = linkedGoAlias(imported.Path, imported.Alias)
				}
				canonical[imported.Path] = linked
				usedAliases[linked] = imported.Path
			}
			bindings[path][imported.Alias] = linked
			imported.ResolvedAlias = linked
		}
	}
	return bindings, canonical
}

func importAliasSpan(program *ast.Program, alias string) source.Span {
	for _, imported := range program.Imports {
		if imported.Go && imported.Alias == alias {
			return imported.Span
		}
	}
	return source.Span{}
}

func isReservedLanguageTypeName(name string) bool {
	switch name {
	case "void", "boolean", "string", "int", "int32", "int64", "uint16", "uint32", "uint64", "float32", "float", "number", "float64", "byte", "error", "Map", "Result":
		return true
	default:
		return false
	}
}

func linkedGoAlias(importPath, alias string) string {
	digest := sha256.Sum256([]byte(importPath))
	return fmt.Sprintf("%s_%x", alias, digest[:4])
}

func topLevelName(declaration ast.Declaration) string {
	switch declaration := declaration.(type) {
	case *ast.FunctionDecl:
		return declaration.Name
	case *ast.VariableDecl:
		return declaration.Name
	case *ast.ClassDecl:
		return declaration.Name
	case *ast.StructDecl:
		return declaration.Name
	case *ast.TypeDecl:
		return declaration.Name
	case *ast.InterfaceDecl:
		return declaration.Name
	default:
		return ""
	}
}

func (l *moduleLoader) linkedModuleName(path, name string) string {
	stablePath := path
	if l.linkBase != "" {
		if relative, err := filepath.Rel(l.linkBase, path); err == nil {
			stablePath = relative
		}
	}
	base := strings.TrimSuffix(filepath.Base(stablePath), filepath.Ext(stablePath))
	var cleaned strings.Builder
	for _, character := range base {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' {
			cleaned.WriteRune(character)
		} else {
			cleaned.WriteByte('_')
		}
	}
	digest := sha256.Sum256([]byte(filepath.ToSlash(stablePath)))
	return fmt.Sprintf("_ontama_%s_%x_%s", cleaned.String(), digest[:4], name)
}

func linkProgram(program *ast.Program, declarations, visible moduleNames) {
	for _, declaration := range program.Declarations {
		linkDeclaration(declaration, declarations, visible)
	}
}

func linkDeclaration(declaration ast.Declaration, declarations, visible moduleNames) {
	switch declaration := declaration.(type) {
	case *ast.VariableDecl:
		original := declaration.Name
		linkType(&declaration.Type, visible)
		linkExpression(declaration.Value, visible, nil)
		declaration.Name = declarations[original]
	case *ast.FunctionDecl:
		original := declaration.Name
		functionVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			delete(functionVisible, parameter.Name)
		}
		locals := parameterNames(declaration.Parameters)
		for i := range declaration.Parameters {
			linkType(&declaration.Parameters[i].Type, functionVisible)
		}
		linkType(&declaration.ReturnType, functionVisible)
		linkBlock(declaration.Body, functionVisible, locals)
		declaration.Name = declarations[original]
	case *ast.MethodDecl:
		methodVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			delete(methodVisible, parameter.Name)
		}
		locals := parameterNames(declaration.Parameters)
		locals[declaration.ReceiverName] = true
		linkType(&declaration.ReceiverType, methodVisible)
		for i := range declaration.Parameters {
			linkType(&declaration.Parameters[i].Type, methodVisible)
		}
		linkType(&declaration.ReturnType, methodVisible)
		linkBlock(declaration.Body, methodVisible, locals)
	case *ast.ClassDecl:
		original := declaration.Name
		if declaration.Base != nil {
			linkType(declaration.Base, visible)
		}
		for i := range declaration.Implements {
			linkType(&declaration.Implements[i], visible)
		}
		for i := range declaration.Fields {
			linkType(&declaration.Fields[i].Type, visible)
		}
		if declaration.Constructor != nil {
			locals := parameterNames(declaration.Constructor.Parameters)
			locals["this"] = true
			for i := range declaration.Constructor.Parameters {
				linkType(&declaration.Constructor.Parameters[i].Type, visible)
			}
			linkBlock(declaration.Constructor.Body, visible, locals)
		}
		for _, method := range declaration.Methods {
			methodVisible := cloneModuleNames(visible)
			for _, parameter := range method.TypeParameters {
				delete(methodVisible, parameter.Name)
			}
			locals := parameterNames(method.Parameters)
			if !method.Static {
				locals["this"] = true
			}
			for i := range method.Parameters {
				linkType(&method.Parameters[i].Type, methodVisible)
			}
			linkType(&method.ReturnType, methodVisible)
			linkBlock(method.Body, methodVisible, locals)
		}
		declaration.Name = declarations[original]
	case *ast.StructDecl:
		original := declaration.Name
		structVisible := cloneModuleNames(visible)
		for _, parameter := range declaration.TypeParameters {
			delete(structVisible, parameter.Name)
		}
		for i := range declaration.Fields {
			linkType(&declaration.Fields[i].Type, structVisible)
		}
		for _, method := range declaration.Methods {
			methodVisible := cloneModuleNames(structVisible)
			for _, parameter := range method.TypeParameters {
				delete(methodVisible, parameter.Name)
			}
			locals := parameterNames(method.Parameters)
			locals["this"] = true
			for i := range method.Parameters {
				linkType(&method.Parameters[i].Type, methodVisible)
			}
			linkType(&method.ReturnType, methodVisible)
			linkBlock(method.Body, methodVisible, locals)
		}
		declaration.Name = declarations[original]
	case *ast.TypeDecl:
		original := declaration.Name
		linkType(&declaration.Underlying, visible)
		declaration.Name = declarations[original]
	case *ast.InterfaceDecl:
		original := declaration.Name
		for i := range declaration.Methods {
			for j := range declaration.Methods[i].Parameters {
				linkType(&declaration.Methods[i].Parameters[j].Type, visible)
			}
			linkType(&declaration.Methods[i].ReturnType, visible)
		}
		declaration.Name = declarations[original]
	}
}

func parameterNames(parameters []ast.Parameter) map[string]bool {
	result := map[string]bool{}
	for _, parameter := range parameters {
		result[parameter.Name] = true
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

func linkBlock(block *ast.BlockStmt, visible moduleNames, inherited map[string]bool) {
	if block == nil {
		return
	}
	locals := cloneNames(inherited)
	for _, statement := range block.Statements {
		linkStatement(statement, visible, locals)
		if variable, ok := statement.(*ast.VariableDecl); ok {
			locals[variable.Name] = true
		}
		if declaration, ok := statement.(*ast.MultiVariableDecl); ok {
			for _, binding := range declaration.Bindings {
				if binding.Name != "_" {
					locals[binding.Name] = true
				}
			}
		}
	}
}

func linkStatement(statement ast.Statement, visible moduleNames, locals map[string]bool) {
	switch statement := statement.(type) {
	case *ast.VariableDecl:
		linkType(&statement.Type, visible)
		linkExpression(statement.Value, visible, locals)
	case *ast.MultiVariableDecl:
		linkExpression(statement.Value, visible, locals)
	case *ast.BlockStmt:
		linkBlock(statement, visible, locals)
	case *ast.ReturnStmt:
		linkExpression(statement.Value, visible, locals)
	case *ast.ThrowStmt:
		linkExpression(statement.Value, visible, locals)
	case *ast.TryStmt:
		linkBlock(statement.Body, visible, locals)
		for _, clause := range statement.Catches {
			linkType(&clause.Type, visible)
			catchLocals := cloneNames(locals)
			if clause.Name != "_" {
				catchLocals[clause.Name] = true
			}
			linkBlock(clause.Body, visible, catchLocals)
		}
		linkBlock(statement.FinallyBody, visible, locals)
	case *ast.IfStmt:
		linkExpression(statement.Condition, visible, locals)
		linkBlock(statement.Then, visible, locals)
		if statement.Else != nil {
			linkStatement(statement.Else, visible, cloneNames(locals))
		}
	case *ast.ExpressionStmt:
		linkExpression(statement.Value, visible, locals)
	case *ast.AssignmentStmt:
		linkExpression(statement.Target, visible, locals)
		linkExpression(statement.Value, visible, locals)
	case *ast.IncDecStmt:
		linkExpression(statement.Target, visible, locals)
	case *ast.MultiAssignmentStmt:
		for i := range statement.Bindings {
			binding := &statement.Bindings[i]
			if binding.Name != "_" && !locals[binding.Name] {
				if linked, exists := visible[binding.Name]; exists {
					binding.Name = linked
				}
			}
		}
		linkExpression(statement.Value, visible, locals)
	case *ast.WhileStmt:
		linkExpression(statement.Condition, visible, locals)
		linkBlock(statement.Body, visible, locals)
	case *ast.ForStmt:
		loopLocals := cloneNames(locals)
		if statement.Initializer != nil {
			linkStatement(statement.Initializer, visible, loopLocals)
			if variable, ok := statement.Initializer.(*ast.VariableDecl); ok {
				loopLocals[variable.Name] = true
			}
			if declaration, ok := statement.Initializer.(*ast.MultiVariableDecl); ok {
				for _, binding := range declaration.Bindings {
					if binding.Name != "_" {
						loopLocals[binding.Name] = true
					}
				}
			}
		}
		linkExpression(statement.Condition, visible, loopLocals)
		linkBlock(statement.Body, visible, loopLocals)
		if statement.Post != nil {
			linkStatement(statement.Post, visible, loopLocals)
		}
	case *ast.ForRangeStmt:
		for index := range statement.Bindings {
			linkType(&statement.Bindings[index].Type, visible)
		}
		linkExpression(statement.Source, visible, locals)
		loopLocals := cloneNames(locals)
		for _, binding := range statement.Bindings {
			if binding.Name != "_" {
				loopLocals[binding.Name] = true
			}
		}
		linkBlock(statement.Body, visible, loopLocals)
	case *ast.SelectStmt:
		for i := range statement.Cases {
			clause := &statement.Cases[i]
			linkExpression(clause.Channel, visible, locals)
			linkExpression(clause.Value, visible, locals)
			for _, target := range clause.Targets {
				linkExpression(target, visible, locals)
			}
			caseLocals := cloneNames(locals)
			if clause.Declare {
				for _, binding := range clause.Bindings {
					if binding.Name != "_" {
						caseLocals[binding.Name] = true
					}
				}
			}
			linkBlock(clause.Body, visible, caseLocals)
		}
	case *ast.ValueSwitchStmt:
		linkExpression(statement.Value, visible, locals)
		for i := range statement.Cases {
			clause := &statement.Cases[i]
			for _, value := range clause.Values {
				linkExpression(value, visible, locals)
			}
			linkBlock(clause.Body, visible, cloneNames(locals))
		}
	case *ast.TypeSwitchStmt:
		linkExpression(statement.Value, visible, locals)
		for i := range statement.Cases {
			clause := &statement.Cases[i]
			linkType(&clause.Type, visible)
			caseLocals := cloneNames(locals)
			if !clause.Nil && !clause.Default && clause.Name != "_" {
				caseLocals[clause.Name] = true
			}
			linkBlock(clause.Body, visible, caseLocals)
		}
	case *ast.CallControlStmt:
		linkExpression(statement.Value, visible, locals)
	case *ast.DetachStmt:
		linkExpression(statement.Value, visible, locals)
	case *ast.ChannelSendStmt:
		linkExpression(statement.Channel, visible, locals)
		linkExpression(statement.Value, visible, locals)
	}
}

func linkExpression(expression ast.Expression, visible moduleNames, locals map[string]bool) {
	switch expression := expression.(type) {
	case *ast.IdentifierExpr:
		if !locals[expression.Name] {
			if linked, exists := visible[expression.Name]; exists {
				expression.Name = linked
			}
		}
	case *ast.UnaryExpr:
		linkExpression(expression.Operand, visible, locals)
	case *ast.PropagateExpr:
		linkExpression(expression.Value, visible, locals)
	case *ast.TaskStartExpr:
		linkExpression(expression.Call, visible, locals)
	case *ast.AwaitExpr:
		linkExpression(expression.Value, visible, locals)
	case *ast.BinaryExpr:
		linkExpression(expression.Left, visible, locals)
		linkExpression(expression.Right, visible, locals)
	case *ast.CallExpr:
		linkExpression(expression.Callee, visible, locals)
		for i := range expression.TypeArguments {
			linkType(&expression.TypeArguments[i], visible)
		}
		for _, argument := range expression.Arguments {
			linkExpression(argument, visible, locals)
		}
	case *ast.ArrowExpr:
		arrowLocals := cloneNames(locals)
		for i := range expression.Parameters {
			linkType(&expression.Parameters[i].Type, visible)
			arrowLocals[expression.Parameters[i].Name] = true
		}
		linkType(expression.ReturnType, visible)
		linkExpression(expression.ExpressionBody, visible, arrowLocals)
		linkBlock(expression.BlockBody, visible, arrowLocals)
	case *ast.ArrayLiteralExpr:
		for _, element := range expression.Elements {
			linkExpression(element, visible, locals)
		}
	case *ast.ObjectLiteralExpr:
		for _, field := range expression.Fields {
			linkExpression(field.Value, visible, locals)
		}
	case *ast.GoCompositeLiteralExpr:
		linkType(&expression.Type, visible)
		for _, field := range expression.Fields {
			linkExpression(field.Value, visible, locals)
		}
	case *ast.MemberExpr:
		linkExpression(expression.Object, visible, locals)
	case *ast.IndexExpr:
		linkExpression(expression.Object, visible, locals)
		linkExpression(expression.Index, visible, locals)
	case *ast.SliceExpr:
		linkExpression(expression.Object, visible, locals)
		linkExpression(expression.Low, visible, locals)
		linkExpression(expression.High, visible, locals)
		linkExpression(expression.Max, visible, locals)
	case *ast.NewExpr:
		if linked, exists := visible[expression.ClassName]; exists {
			expression.ClassName = linked
		}
		for _, argument := range expression.Arguments {
			linkExpression(argument, visible, locals)
		}
	case *ast.ClassUpcastExpr:
		linkExpression(expression.Value, visible, locals)
	}
}

func cloneModuleNames(values moduleNames) moduleNames {
	cloned := make(moduleNames, len(values))
	for name, linked := range values {
		cloned[name] = linked
	}
	return cloned
}

func linkType(ref *ast.TypeRef, visible moduleNames) {
	if ref == nil {
		return
	}
	if linked, exists := visible[ref.Name]; exists {
		ref.Name = linked
	}
	if linked, exists := visible[ref.Qualifier]; exists {
		ref.Qualifier = linked
	}
	for i := range ref.GenericArguments {
		linkType(&ref.GenericArguments[i], visible)
	}
	linkType(ref.Element, visible)
	linkType(ref.Pointee, visible)
	for i := range ref.Parameters {
		linkType(&ref.Parameters[i], visible)
	}
	linkType(ref.Return, visible)
	for i := range ref.ObjectFields {
		linkType(&ref.ObjectFields[i].Type, visible)
	}
}
