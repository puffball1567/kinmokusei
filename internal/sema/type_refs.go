package sema

import (
	gotypes "go/types"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) markResolvedTypeRefs(program *ast.Program) {
	var activeTypeParameters map[string]source.Span
	var visitType func(*ast.TypeRef)
	visitType = func(ref *ast.TypeRef) {
		if ref == nil {
			return
		}
		if ref.Qualifier == "" {
			if declaration, ok := activeTypeParameters[ref.Name]; ok {
				ref.TypeParameter = true
				ref.ResolvedDeclaration = declaration
			} else if imported, ok := c.goNamedImports[ref.Span.Path][ref.Name]; ok && ref.Name != "" {
				ref.ResolvedDeclaration = imported.span
				lowered := *ref
				lowered.LoweredType = nil
				lowered.Qualifier = imported.pack.declaration.ResolvedAlias
				if lowered.Qualifier == "" {
					lowered.Qualifier = imported.pack.declaration.Alias
				}
				lowered.Go = true
				ref.LoweredType = &lowered
			} else if named, ok := c.nativeTypes[ref.Name]; ok {
				ref.NativeNamed = true
				ref.ResolvedDeclaration = named.declaration.NameSpan
			} else if contract, ok := c.interfaces[ref.Name]; ok {
				ref.Interface = true
				ref.ResolvedDeclaration = contract.declarationSpan
			} else if class, ok := c.classes[ref.Name]; ok {
				ref.ResolvedDeclaration = class.declarationSpan
			} else if structure, ok := c.structs[ref.Name]; ok {
				ref.Struct = true
				ref.ResolvedDeclaration = structure.declarationSpan
			}
		} else if imported := c.lookupGoPackage(ref.Span.Path, ref.Qualifier); imported != nil {
			ref.QualifierDeclaration = imported.declaration.AliasSpan
			if imported.declaration.ResolvedAlias != "" {
				ref.Qualifier = imported.declaration.ResolvedAlias
			}
		}
		for i := range ref.GenericArguments {
			visitType(&ref.GenericArguments[i])
		}
		for i := range ref.GoResults {
			visitType(&ref.GoResults[i])
		}
		visitType(ref.Element)
		visitType(ref.Pointee)
		for i := range ref.Parameters {
			visitType(&ref.Parameters[i])
		}
		visitType(ref.Return)
		for i := range ref.ObjectFields {
			visitType(&ref.ObjectFields[i].Type)
		}
		if ref.Qualifier == "" {
			if named, ok := c.nativeTypes[ref.Name]; ok && named.typeInfo.Kind != Invalid && named.declaration.Alias && len(named.typeParameters) != 0 && len(ref.GenericArguments) == len(named.typeParameters) {
				expanded := instantiateGenericAliasTypeRef(*ref, named.declaration)
				visitType(&expanded)
				ref.LoweredType = &expanded
			}
		}
	}
	visitTypeParameters := func(parameters []ast.TypeParameter) {
		for index := range parameters {
			visitType(parameters[index].Constraint)
		}
	}
	var visitExpression func(ast.Expression)
	var visitStatement func(ast.Statement)
	visitExpression = func(expression ast.Expression) {
		switch expression := expression.(type) {
		case *ast.UnaryExpr:
			visitExpression(expression.Operand)
		case *ast.PropagateExpr:
			visitExpression(expression.Value)
			visitType(&expression.ValueType)
			visitType(&expression.ResultType)
		case *ast.TaskStartExpr:
			visitExpression(expression.Call)
			visitType(&expression.ValueType)
		case *ast.AwaitExpr:
			visitExpression(expression.Value)
			visitType(&expression.ValueType)
		case *ast.BinaryExpr:
			visitExpression(expression.Left)
			visitExpression(expression.Right)
		case *ast.GoTypeAssertionExpr:
			visitExpression(expression.Value)
			visitType(&expression.Type)
		case *ast.CallExpr:
			visitExpression(expression.Callee)
			for i := range expression.ResolvedTypeArguments {
				visitType(&expression.ResolvedTypeArguments[i])
			}
			for i := range expression.TypeArguments {
				visitType(&expression.TypeArguments[i])
			}
			for _, argument := range expression.Arguments {
				visitExpression(argument)
			}
			visitType(expression.ConversionType)
			if expression.ConversionType != nil && expression.ConversionType.TypeParameter {
				if name, ok := expression.Callee.(*ast.IdentifierExpr); ok {
					name.ResolvedDeclaration = expression.ConversionType.ResolvedDeclaration
				}
			}
		case *ast.ArrowExpr:
			for i := range expression.Parameters {
				visitType(&expression.Parameters[i].Type)
			}
			visitType(expression.ReturnType)
			visitType(&expression.ResolvedReturnType)
			visitExpression(expression.ExpressionBody)
			if expression.BlockBody != nil {
				visitStatement(expression.BlockBody)
			}
		case *ast.ArrayLiteralExpr:
			visitType(&expression.ResolvedElementType)
			for _, element := range expression.Elements {
				visitExpression(element)
			}
		case *ast.ObjectLiteralExpr:
			for i := range expression.ResolvedFieldTypes {
				visitType(&expression.ResolvedFieldTypes[i])
			}
			for _, field := range expression.Fields {
				visitExpression(field.Value)
			}
		case *ast.GoCompositeLiteralExpr:
			visitType(&expression.Type)
			for _, field := range expression.Fields {
				visitExpression(field.Value)
			}
		case *ast.MemberExpr:
			visitExpression(expression.Object)
		case *ast.IndexExpr:
			visitExpression(expression.Object)
			visitExpression(expression.Index)
		case *ast.SliceExpr:
			visitExpression(expression.Object)
			visitExpression(expression.Low)
			visitExpression(expression.High)
			visitExpression(expression.Max)
		case *ast.NewExpr:
			for index := range expression.TypeArguments {
				visitType(&expression.TypeArguments[index])
			}
			for _, argument := range expression.Arguments {
				visitExpression(argument)
			}
		case *ast.ClassUpcastExpr:
			visitExpression(expression.Value)
		}
	}
	visitStatement = func(statement ast.Statement) {
		if statement == nil {
			return
		}
		switch statement := statement.(type) {
		case *ast.VariableDecl:
			visitType(&statement.Type)
			visitType(&statement.ResolvedType)
			visitExpression(statement.Value)
		case *ast.MultiVariableDecl:
			for i := range statement.Bindings {
				visitType(&statement.Bindings[i].ResolvedType)
			}
			visitExpression(statement.Value)
		case *ast.BlockStmt:
			for _, child := range statement.Statements {
				visitStatement(child)
			}
		case *ast.ReturnStmt:
			visitExpression(statement.Value)
		case *ast.ThrowStmt:
			visitExpression(statement.Value)
		case *ast.TryStmt:
			visitStatement(statement.Body)
			for _, clause := range statement.Catches {
				visitType(&clause.Type)
				visitStatement(clause.Body)
			}
			if statement.FinallyBody != nil {
				visitStatement(statement.FinallyBody)
			}
		case *ast.IfStmt:
			visitExpression(statement.Condition)
			visitStatement(statement.Then)
			if statement.Else != nil {
				visitStatement(statement.Else)
			}
		case *ast.ExpressionStmt:
			visitExpression(statement.Value)
		case *ast.AssignmentStmt:
			visitExpression(statement.Target)
			visitExpression(statement.Value)
		case *ast.IncDecStmt:
			visitExpression(statement.Target)
		case *ast.MultiAssignmentStmt:
			visitExpression(statement.Value)
		case *ast.WhileStmt:
			visitExpression(statement.Condition)
			visitStatement(statement.Body)
		case *ast.ForStmt:
			if statement.Initializer != nil {
				visitStatement(statement.Initializer)
			}
			visitExpression(statement.Condition)
			if statement.Post != nil {
				visitStatement(statement.Post)
			}
			visitStatement(statement.Body)
		case *ast.ForRangeStmt:
			for index := range statement.Bindings {
				visitType(&statement.Bindings[index].Type)
				visitType(&statement.Bindings[index].ResolvedType)
			}
			visitExpression(statement.Source)
			visitStatement(statement.Body)
		case *ast.SelectStmt:
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				for binding := range clause.Bindings {
					visitType(&clause.Bindings[binding].ResolvedType)
				}
				visitExpression(clause.Channel)
				visitExpression(clause.Value)
				for _, target := range clause.Targets {
					visitExpression(target)
				}
				visitStatement(clause.Body)
			}
		case *ast.ValueSwitchStmt:
			visitExpression(statement.Value)
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				for _, value := range clause.Values {
					visitExpression(value)
				}
				visitStatement(clause.Body)
			}
		case *ast.TypeSwitchStmt:
			visitExpression(statement.Value)
			for i := range statement.Cases {
				clause := &statement.Cases[i]
				visitType(&clause.Type)
				visitStatement(clause.Body)
			}
		case *ast.CallControlStmt:
			visitExpression(statement.Value)
		case *ast.DetachStmt:
			visitExpression(statement.Value)
			visitType(&statement.ValueType)
		case *ast.ChannelSendStmt:
			visitExpression(statement.Channel)
			visitExpression(statement.Value)
		}
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.VariableDecl:
			visitType(&declaration.Type)
			visitType(&declaration.ResolvedType)
			visitExpression(declaration.Value)
		case *ast.FunctionDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			for i := range declaration.Parameters {
				visitType(&declaration.Parameters[i].Type)
			}
			visitType(&declaration.ReturnType)
			visitStatement(declaration.Body)
			activeTypeParameters = nil
		case *ast.MethodDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			visitType(&declaration.ReceiverType)
			for i := range declaration.Parameters {
				visitType(&declaration.Parameters[i].Type)
			}
			visitType(&declaration.ReturnType)
			visitStatement(declaration.Body)
			activeTypeParameters = nil
		case *ast.ClassDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			if declaration.Base != nil {
				visitType(declaration.Base)
			}
			for i := range declaration.Implements {
				visitType(&declaration.Implements[i])
			}
			for i := range declaration.Fields {
				visitType(&declaration.Fields[i].Type)
				visitExpression(declaration.Fields[i].Initializer)
			}
			if declaration.Constructor != nil {
				for i := range declaration.Constructor.Parameters {
					visitType(&declaration.Constructor.Parameters[i].Type)
				}
				visitStatement(declaration.Constructor.Body)
			}
			for _, method := range declaration.Methods {
				classTypeParameters := activeTypeParameters
				activeTypeParameters = make(map[string]source.Span, len(classTypeParameters)+len(method.TypeParameters))
				for name, span := range classTypeParameters {
					activeTypeParameters[name] = span
				}
				for _, parameter := range method.TypeParameters {
					activeTypeParameters[parameter.Name] = parameter.NameSpan
				}
				visitTypeParameters(method.TypeParameters)
				for i := range method.Parameters {
					visitType(&method.Parameters[i].Type)
				}
				visitType(&method.ReturnType)
				visitStatement(method.Body)
				activeTypeParameters = classTypeParameters
			}
			activeTypeParameters = nil
		case *ast.InterfaceDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			for i := range declaration.Bases {
				visitType(&declaration.Bases[i])
			}
			for i := range declaration.Terms {
				visitType(&declaration.Terms[i].Type)
			}
			for i := range declaration.Methods {
				method := &declaration.Methods[i]
				for j := range method.Parameters {
					visitType(&method.Parameters[j].Type)
				}
				visitType(&method.ReturnType)
			}
			activeTypeParameters = nil
		case *ast.StructDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			for i := range declaration.Fields {
				visitType(&declaration.Fields[i].Type)
			}
			for _, method := range declaration.Methods {
				structTypeParameters := activeTypeParameters
				activeTypeParameters = make(map[string]source.Span, len(structTypeParameters)+len(method.TypeParameters))
				for name, span := range structTypeParameters {
					activeTypeParameters[name] = span
				}
				for _, parameter := range method.TypeParameters {
					activeTypeParameters[parameter.Name] = parameter.NameSpan
				}
				visitTypeParameters(method.TypeParameters)
				for i := range method.Parameters {
					visitType(&method.Parameters[i].Type)
				}
				visitType(&method.ReturnType)
				visitStatement(method.Body)
				activeTypeParameters = structTypeParameters
			}
			activeTypeParameters = nil
		case *ast.TypeDecl:
			activeTypeParameters = make(map[string]source.Span, len(declaration.TypeParameters))
			for _, parameter := range declaration.TypeParameters {
				activeTypeParameters[parameter.Name] = parameter.NameSpan
			}
			visitTypeParameters(declaration.TypeParameters)
			visitType(&declaration.Underlying)
			activeTypeParameters = nil
		case *ast.EnumDecl:
			visitType(&declaration.Underlying)
			for index := range declaration.Members {
				visitExpression(declaration.Members[index].Value)
			}
		}
	}
}

func instantiateGenericAliasTypeRef(ref ast.TypeRef, declaration *ast.TypeDecl) ast.TypeRef {
	bindings := make(map[string]ast.TypeRef, len(declaration.TypeParameters))
	for index, parameter := range declaration.TypeParameters {
		if index < len(ref.GenericArguments) {
			bindings[parameter.Name] = ref.GenericArguments[index]
		}
	}
	return substituteNativeTypeRefParameters(declaration.Underlying, bindings)
}

func substituteNativeTypeRefParameters(ref ast.TypeRef, bindings map[string]ast.TypeRef) ast.TypeRef {
	if ref.Qualifier == "" && !ref.IsArray() && !ref.IsPointer() && !ref.IsFunction() && !ref.IsObject() && !ref.IsGoStruct() && len(ref.GenericArguments) == 0 {
		if replacement, ok := bindings[ref.Name]; ok {
			return replacement
		}
	}
	result := ref
	result.LoweredType = nil
	result.GenericArguments = append([]ast.TypeRef(nil), ref.GenericArguments...)
	for index := range result.GenericArguments {
		result.GenericArguments[index] = substituteNativeTypeRefParameters(result.GenericArguments[index], bindings)
	}
	if ref.Element != nil {
		element := substituteNativeTypeRefParameters(*ref.Element, bindings)
		result.Element = &element
	}
	if ref.Pointee != nil {
		pointee := substituteNativeTypeRefParameters(*ref.Pointee, bindings)
		result.Pointee = &pointee
	}
	result.Parameters = append([]ast.TypeRef(nil), ref.Parameters...)
	for index := range result.Parameters {
		result.Parameters[index] = substituteNativeTypeRefParameters(result.Parameters[index], bindings)
	}
	if ref.Return != nil {
		returnType := substituteNativeTypeRefParameters(*ref.Return, bindings)
		result.Return = &returnType
	}
	result.ObjectFields = append([]ast.ObjectTypeField(nil), ref.ObjectFields...)
	for index := range result.ObjectFields {
		result.ObjectFields[index].Type = substituteNativeTypeRefParameters(result.ObjectFields[index].Type, bindings)
	}
	return result
}

func typeRefFromType(t Type, span source.Span) ast.TypeRef {
	if t.Kind == MultiValue {
		results := make([]ast.TypeRef, len(t.Results))
		for i, result := range t.Results {
			results[i] = typeRefFromType(result, span)
		}
		return ast.TypeRef{GoResults: results, Go: true, Span: span}
	}
	if t.Kind == GoInterface {
		methods := make([]ast.ObjectTypeField, len(t.GoMethods))
		for i, method := range t.GoMethods {
			methods[i] = ast.ObjectTypeField{Name: method.Name, Type: typeRefFromType(method.Type, span), Span: span}
		}
		return ast.TypeRef{GoInterface: true, ObjectFields: methods, Go: true, Span: span}
	}
	if t.Kind == TypeParameter {
		return ast.TypeRef{Name: t.Name, TypeParameter: true, Span: span}
	}
	if t.Kind == Nullable && t.Element != nil {
		ref := typeRefFromType(*t.Element, span)
		ref.Nullable = true
		ref.Span = span
		return ref
	}
	if t.Kind == GoPointer && t.Element != nil {
		pointee := typeRefFromType(*t.Element, span)
		return ast.TypeRef{Pointee: &pointee, Span: span}
	}
	if t.Kind == Array && t.Element != nil {
		element := typeRefFromType(*t.Element, span)
		return ast.TypeRef{Element: &element, Span: span}
	}
	if t.Kind == FixedArray && t.Element != nil {
		element := typeRefFromType(*t.Element, span)
		length := t.Length
		return ast.TypeRef{Element: &element, FixedLength: &length, Span: span}
	}
	if t.Kind == Map && t.Key != nil && t.Element != nil {
		return ast.TypeRef{Name: "Map", GenericArguments: []ast.TypeRef{typeRefFromType(*t.Key, span), typeRefFromType(*t.Element, span)}, Span: span}
	}
	if t.Kind == Result && t.Element != nil {
		return ast.TypeRef{Name: "Result", GenericArguments: []ast.TypeRef{typeRefFromType(*t.Element, span)}, Span: span}
	}
	if t.Kind == Task && t.Element != nil {
		return ast.TypeRef{Name: "Task", GenericArguments: []ast.TypeRef{typeRefFromType(*t.Element, span)}, Span: span}
	}
	if t.Kind == GoChannel && t.Element != nil {
		name := "GoChannel"
		if channel, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Chan); ok {
			if channel.Dir() == gotypes.SendOnly {
				name = "GoSendChannel"
			} else if channel.Dir() == gotypes.RecvOnly {
				name = "GoReceiveChannel"
			}
		}
		return ast.TypeRef{Name: name, GenericArguments: []ast.TypeRef{typeRefFromType(*t.Element, span)}, Go: true, Span: span}
	}
	if t.Kind == Object {
		names := make([]string, 0, len(t.Fields))
		for name := range t.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		fields := make([]ast.ObjectTypeField, 0, len(names))
		for _, name := range names {
			goName := t.FieldNames[name]
			if goName == "" {
				goName = name
			}
			fields = append(fields, ast.ObjectTypeField{Name: goName, JSONName: name, Type: typeRefFromType(t.Fields[name], span), Span: span})
		}
		return ast.TypeRef{Object: true, ObjectFields: fields, Span: span}
	}
	if t.Kind == Struct {
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for index := range t.TypeArguments {
			arguments[index] = typeRefFromType(t.TypeArguments[index], span)
		}
		return ast.TypeRef{Name: t.Name, GenericArguments: arguments, Struct: true, Span: span}
	}
	if t.Kind == Class {
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for index := range t.TypeArguments {
			arguments[index] = typeRefFromType(t.TypeArguments[index], span)
		}
		return ast.TypeRef{Name: t.Name, GenericArguments: arguments, Span: span}
	}
	if t.Kind == GoStruct {
		fields := make([]ast.ObjectTypeField, len(t.GoFields))
		for index, field := range t.GoFields {
			fields[index] = ast.ObjectTypeField{
				Name: field.Name, GoTag: field.Tag, GoEmbedded: field.Embedded,
				Type: typeRefFromType(field.Type, span), Span: span,
			}
		}
		return ast.TypeRef{GoStruct: true, ObjectFields: fields, Go: true, Span: span}
	}
	if t.Kind == Interface {
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for index := range t.TypeArguments {
			arguments[index] = typeRefFromType(t.TypeArguments[index], span)
		}
		return ast.TypeRef{Name: t.Name, GenericArguments: arguments, Interface: true, Span: span}
	}
	if object := goTypeNameObject(t.GoType); object != nil {
		name := t.Name
		name = object.Name()
		arguments := make([]ast.TypeRef, len(t.TypeArguments))
		for i := range t.TypeArguments {
			arguments[i] = typeRefFromType(t.TypeArguments[i], span)
		}
		return ast.TypeRef{Name: name, Qualifier: t.GoQualifier, GenericArguments: arguments, Go: true, Span: span}
	}
	if t.Kind == GoBasic {
		return ast.TypeRef{Name: t.Name, Qualifier: t.GoQualifier, Go: true, Span: span}
	}
	if t.Kind != Function {
		return ast.TypeRef{Name: t.Name, Span: span}
	}
	parameters := make([]ast.TypeRef, len(t.Parameters))
	for i, parameter := range t.Parameters {
		parameters[i] = typeRefFromType(parameter, span)
		if t.Variadic && i == len(t.Parameters)-1 {
			element := parameters[i]
			parameters[i] = ast.TypeRef{Element: &element, Span: span}
		}
	}
	result := typeRefFromType(*t.Result, span)
	return ast.TypeRef{Parameters: parameters, Return: &result, Variadic: t.Variadic, Span: span}
}

func goTypeNameObject(goType gotypes.Type) *gotypes.TypeName {
	switch goType := goType.(type) {
	case *gotypes.Named:
		return goType.Obj()
	case *gotypes.Alias:
		return goType.Obj()
	default:
		return nil
	}
}
