package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func generateClass(class *kinmokuseiAST.ClassDecl) ([]goast.Decl, error) {
	var declarations []goast.Decl
	typeParameterFields := make([]*goast.Field, 0, len(class.TypeParameters))
	for _, parameter := range class.TypeParameters {
		typeParameterFields = append(typeParameterFields, goTypeParameterField(parameter, false))
	}
	classType := indexedGoType(goast.NewIdent(class.Name), typeRefsForTypeParameters(class.TypeParameters))
	classPointer := &goast.StarExpr{X: classType}
	for _, method := range class.Methods {
		if !method.Virtual || method.Override || method.Static {
			continue
		}
		parameters := make([]*goast.Field, 0, len(method.Parameters))
		for _, parameter := range method.Parameters {
			parameters = append(parameters, goParameterField(parameter))
		}
		methodType := &goast.FuncType{Params: &goast.FieldList{List: parameters}, Results: functionResults(method.ReturnType)}
		name := method.GoName
		if name == "" {
			name = memberName(method.Name, method.Visibility)
		}
		name = virtualSlotName(class.Name, name)
		interfaceName := virtualInterfaceName(class.Name)
		if len(declarations) != 0 {
			if declaration, ok := declarations[len(declarations)-1].(*goast.GenDecl); ok {
				if specification, ok := declaration.Specs[0].(*goast.TypeSpec); ok && specification.Name.Name == interfaceName {
					specification.Type.(*goast.InterfaceType).Methods.List = append(specification.Type.(*goast.InterfaceType).Methods.List, &goast.Field{Names: []*goast.Ident{goast.NewIdent(name)}, Type: methodType})
					continue
				}
			}
		}
		interfaceSpec := &goast.TypeSpec{
			Name: goast.NewIdent(interfaceName), Type: &goast.InterfaceType{Methods: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent(name)}, Type: methodType}}}},
		}
		if len(typeParameterFields) != 0 {
			interfaceSpec.TypeParams = &goast.FieldList{List: typeParameterFields}
		}
		declarations = append(declarations, &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{interfaceSpec}})
	}
	if len(class.Ancestors) != 0 {
		projectionSpec := &goast.TypeSpec{
			Name: goast.NewIdent(classProjectionInterfaceName(class.Name)),
			Type: &goast.InterfaceType{Methods: &goast.FieldList{List: []*goast.Field{{
				Names: []*goast.Ident{goast.NewIdent(classProjectionMethodName(class.Name))},
				Type: &goast.FuncType{
					Params:  &goast.FieldList{},
					Results: &goast.FieldList{List: []*goast.Field{{Type: classPointer}}},
				},
			}}}},
		}
		if len(typeParameterFields) != 0 {
			projectionSpec.TypeParams = &goast.FieldList{List: typeParameterFields}
		}
		declarations = append(declarations, &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{projectionSpec}})
	}

	fields := make([]*goast.Field, 0, len(class.Fields)+2)
	if class.Base != nil {
		fields = append(fields, &goast.Field{Type: goClassType(*class.Base)})
	}
	if class.HierarchyRoot == class.Name {
		fields = append(fields, &goast.Field{Names: []*goast.Ident{goast.NewIdent(classRootName())}, Type: goast.NewIdent("any")})
	}
	for _, owner := range class.VirtualOwners {
		if owner == class.Name {
			fields = append(fields, &goast.Field{
				Names: []*goast.Ident{goast.NewIdent(virtualSelfName(owner))},
				Type:  indexedGoType(goast.NewIdent(virtualInterfaceName(owner)), typeRefsForTypeParameters(class.TypeParameters)),
			})
		}
	}
	for _, field := range class.Fields {
		name := field.GoName
		if name == "" {
			name = memberName(field.Name, field.Visibility)
		}
		if field.Static {
			value, err := generateExpression(field.Initializer)
			if err != nil {
				return nil, err
			}
			declarations = append(declarations, &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
				Names: []*goast.Ident{goast.NewIdent(name)}, Type: goType(field.Type), Values: []goast.Expr{value},
			}}})
			continue
		}
		fields = append(fields, goStoredField(name, field.Name, field.Visibility, field.Type))
	}
	if class.Constructor != nil {
		for _, parameter := range class.Constructor.Parameters {
			if parameter.IsField {
				fields = append(fields, goStoredField(memberName(parameter.Name, parameter.Visibility), parameter.Name, parameter.Visibility, parameter.Type))
			}
		}
	}
	classSpec := &goast.TypeSpec{Name: goast.NewIdent(class.Name), Type: &goast.StructType{Fields: &goast.FieldList{List: fields}}}
	if len(typeParameterFields) != 0 {
		classSpec.TypeParams = &goast.FieldList{List: typeParameterFields}
	}
	declarations = append(declarations, &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{classSpec}})
	if len(class.Ancestors) != 0 {
		declarations = append(declarations, &goast.FuncDecl{
			Recv: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("this")}, Type: classPointer}}},
			Name: goast.NewIdent(classProjectionMethodName(class.Name)),
			Type: &goast.FuncType{Params: &goast.FieldList{}, Results: &goast.FieldList{List: []*goast.Field{{Type: classPointer}}}},
			Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("this")}}}},
		})
	}
	for ancestorIndex, ancestor := range class.Ancestors {
		ancestorRef := kinmokuseiAST.TypeRef{Name: ancestor}
		if ancestorIndex < len(class.AncestorTypes) {
			ancestorRef = class.AncestorTypes[ancestorIndex]
		}
		ancestorType := goClassType(ancestorRef)
		selected := goast.Expr(goast.NewIdent("value"))
		for index := 0; index <= ancestorIndex; index++ {
			selected = &goast.SelectorExpr{X: selected, Sel: goast.NewIdent(class.Ancestors[index])}
		}
		upcastType := &goast.FuncType{
			Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("value")}, Type: classPointer}}},
			Results: &goast.FieldList{List: []*goast.Field{{Type: &goast.StarExpr{X: ancestorType}}}},
		}
		if len(typeParameterFields) != 0 {
			upcastType.TypeParams = &goast.FieldList{List: typeParameterFields}
		}
		declarations = append(declarations, &goast.FuncDecl{
			Name: goast.NewIdent(upcastName(class.Name, ancestor)),
			Type: upcastType,
			Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.IfStmt{Cond: &goast.BinaryExpr{X: goast.NewIdent("value"), Op: token.EQL, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("nil")}}}}},
				&goast.ReturnStmt{Results: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: selected}}},
			}},
		})
		checkedType := &goast.FuncType{
			Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("value")}, Type: &goast.StarExpr{X: ancestorType}}}},
			Results: &goast.FieldList{List: []*goast.Field{{Type: classPointer}, {Type: goast.NewIdent("bool")}}},
		}
		if len(typeParameterFields) != 0 {
			checkedType.TypeParams = &goast.FieldList{List: typeParameterFields}
		}
		checkedBody := &goast.BlockStmt{List: []goast.Stmt{
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: goast.NewIdent("value"), Op: token.EQL, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("nil"), goast.NewIdent("false")}}}}},
		}}
		projectionType := indexedGoType(goast.NewIdent(classProjectionInterfaceName(class.Name)), typeRefsForTypeParameters(class.TypeParameters))
		projectionAssertion := &goast.TypeAssertExpr{
			X:    &goast.SelectorExpr{X: goast.NewIdent("value"), Sel: goast.NewIdent(classRootName())},
			Type: projectionType,
		}
		checkedBody.List = append(checkedBody.List, &goast.IfStmt{
			Init: &goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("projected"), goast.NewIdent("ok")}, Tok: token.DEFINE, Rhs: []goast.Expr{projectionAssertion}},
			Cond: goast.NewIdent("ok"), Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{
				&goast.CallExpr{Fun: &goast.SelectorExpr{X: goast.NewIdent("projected"), Sel: goast.NewIdent(classProjectionMethodName(class.Name))}},
				goast.NewIdent("true"),
			}}}},
		})
		checkedBody.List = append(checkedBody.List, &goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("nil"), goast.NewIdent("false")}})
		declarations = append(declarations, &goast.FuncDecl{
			Name: goast.NewIdent(downcastName(ancestor, class.Name)), Type: checkedType,
			Body: checkedBody,
		})
		mustType := &goast.FuncType{
			Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("value")}, Type: &goast.StarExpr{X: ancestorType}}}},
			Results: &goast.FieldList{List: []*goast.Field{{Type: classPointer}}},
		}
		if len(typeParameterFields) != 0 {
			mustType.TypeParams = &goast.FieldList{List: typeParameterFields}
		}
		downcastCallee := indexedGoType(goast.NewIdent(downcastName(ancestor, class.Name)), typeRefsForTypeParameters(class.TypeParameters))
		declarations = append(declarations, &goast.FuncDecl{
			Name: goast.NewIdent(mustDowncastName(ancestor, class.Name)),
			Type: mustType,
			Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("result"), goast.NewIdent("ok")}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.CallExpr{Fun: downcastCallee, Args: []goast.Expr{goast.NewIdent("value")}}}},
				&goast.IfStmt{Cond: &goast.UnaryExpr{Op: token.NOT, X: goast.NewIdent("ok")}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{&goast.BasicLit{Kind: token.STRING, Value: strconv.Quote("cannot downcast " + ancestor + " to " + class.Name)}}}}}}},
				&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("result")}},
			}},
		})
		declarations = append(declarations,
			generateConversionWrapper(publicUpcastName(class.Name, ancestor), upcastName(class.Name, ancestor), typeRefForClass(class), ancestorRef, class.TypeParameters, false),
			generateConversionWrapper(publicDowncastName(ancestor, class.Name), downcastName(ancestor, class.Name), ancestorRef, typeRefForClass(class), class.TypeParameters, true),
			generateConversionWrapper(publicMustDowncastName(ancestor, class.Name), mustDowncastName(ancestor, class.Name), ancestorRef, typeRefForClass(class), class.TypeParameters, false),
		)
	}
	for _, implemented := range class.Implements {
		if len(class.TypeParameters) != 0 {
			continue
		}
		declarations = append(declarations, &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
			Names: []*goast.Ident{goast.NewIdent("_")}, Type: goType(implemented),
			Values: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: &goast.CompositeLit{Type: goast.NewIdent(class.Name)}}},
		}}})
	}

	constructorType := &goast.FuncType{Params: &goast.FieldList{}}
	initializerType := &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("this")}, Type: classPointer}}}}
	if len(typeParameterFields) != 0 {
		initializerType.TypeParams = &goast.FieldList{List: typeParameterFields}
	}
	var constructorParameters []kinmokuseiAST.Parameter
	if class.Constructor != nil {
		constructorParameters = class.Constructor.Parameters
	}
	factoryParameterNames := constructorFactoryParameterNames(class, constructorParameters)
	for i, parameter := range constructorParameters {
		field := goParameterField(parameter)
		field.Names = []*goast.Ident{goast.NewIdent(factoryParameterNames[i])}
		constructorType.Params.List = append(constructorType.Params.List, field)
		initializerType.Params.List = append(initializerType.Params.List, goParameterField(parameter))
	}
	initializerBody := &goast.BlockStmt{}
	constructorStatements := []kinmokuseiAST.Statement(nil)
	if class.Constructor != nil {
		constructorStatements = class.Constructor.Body.Statements
	}
	if class.Base != nil {
		var baseCall goast.Stmt
		if len(constructorStatements) != 0 {
			if expression, ok := constructorStatements[0].(*kinmokuseiAST.ExpressionStmt); ok {
				if call, ok := expression.Value.(*kinmokuseiAST.CallExpr); ok && call.SuperConstructor {
					generated, err := generateExpression(call)
					if err != nil {
						return nil, err
					}
					baseCall = &goast.ExprStmt{X: generated}
					constructorStatements = constructorStatements[1:]
				}
			}
		}
		if baseCall == nil {
			baseCall = &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent(initializerName(class.Base.Name)), Args: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(class.Base.Name)}}}}}
		}
		initializerBody.List = append(initializerBody.List, baseCall)
	}
	fieldBody := &goast.BlockStmt{}
	for _, field := range class.Fields {
		if field.Static || field.Initializer == nil {
			continue
		}
		value, err := generateExpression(field.Initializer)
		if err != nil {
			return nil, err
		}
		fieldBody.List = append(fieldBody.List, &goast.AssignStmt{
			Lhs: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(goName(memberName(field.Name, field.Visibility)))}},
			Tok: token.ASSIGN, Rhs: []goast.Expr{value},
		})
	}
	if len(fieldBody.List) != 0 {
		// A separate lexical scope prevents constructor parameters from
		// shadowing module bindings used by field initializers.
		fieldType := &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("this")}, Type: classPointer}}}}
		if len(typeParameterFields) != 0 {
			fieldType.TypeParams = &goast.FieldList{List: typeParameterFields}
		}
		name := "__kinmokuseiFields" + class.Name
		declarations = append(declarations, &goast.FuncDecl{Name: goast.NewIdent(name), Type: fieldType, Body: fieldBody})
		initializerBody.List = append(initializerBody.List, &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent(name), Args: []goast.Expr{goast.NewIdent("this")}}})
	}
	for _, parameter := range constructorParameters {
		if parameter.IsField {
			initializerBody.List = append(initializerBody.List, &goast.AssignStmt{
				Lhs: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(goName(memberName(parameter.Name, parameter.Visibility)))}}, Tok: token.ASSIGN,
				Rhs: []goast.Expr{goast.NewIdent(goName(parameter.Name))},
			})
		}
	}
	for _, owner := range class.VirtualOwners {
		initializerBody.List = append(initializerBody.List, &goast.AssignStmt{
			Lhs: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(virtualSelfName(owner))}}, Tok: token.ASSIGN,
			Rhs: []goast.Expr{goast.NewIdent("this")},
		})
	}
	if class.HierarchyRoot != "" {
		initializerBody.List = append(initializerBody.List, &goast.AssignStmt{
			Lhs: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(classRootName())}}, Tok: token.ASSIGN,
			Rhs: []goast.Expr{goast.NewIdent("this")},
		})
	}
	if len(constructorStatements) != 0 {
		body, err := generateBlock(&kinmokuseiAST.BlockStmt{Statements: constructorStatements})
		if err != nil {
			return nil, err
		}
		initializerBody.List = append(initializerBody.List, body.List...)
	}
	declarations = append(declarations, &goast.FuncDecl{Name: goast.NewIdent(initializerName(class.Name)), Type: initializerType, Body: initializerBody})

	if len(typeParameterFields) != 0 {
		constructorType.TypeParams = &goast.FieldList{List: typeParameterFields}
	}
	constructorType.Results = &goast.FieldList{List: []*goast.Field{{Type: classPointer}}}
	constructorBody := &goast.BlockStmt{}
	constructorBody.List = append(constructorBody.List, &goast.AssignStmt{
		Lhs: []goast.Expr{goast.NewIdent("this")}, Tok: token.DEFINE,
		Rhs: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: &goast.CompositeLit{Type: classType}}},
	})
	initializerArguments := []goast.Expr{goast.NewIdent("this")}
	for _, name := range factoryParameterNames {
		initializerArguments = append(initializerArguments, goast.NewIdent(name))
	}
	initializerCall := &goast.CallExpr{Fun: goast.NewIdent(initializerName(class.Name)), Args: initializerArguments}
	if len(constructorParameters) != 0 && constructorParameters[len(constructorParameters)-1].Variadic {
		initializerCall.Ellipsis = token.Pos(1)
	}
	constructorBody.List = append(constructorBody.List, &goast.ExprStmt{X: initializerCall})
	constructorBody.List = append(constructorBody.List, &goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("this")}})
	if !class.Abstract {
		declarations = append(declarations, &goast.FuncDecl{
			Name: goast.NewIdent("New" + class.Name), Type: constructorType, Body: constructorBody,
		})
	}

	for _, method := range class.Methods {
		parameters := make([]*goast.Field, 0, len(method.Parameters))
		for _, parameter := range method.Parameters {
			parameters = append(parameters, goParameterField(parameter))
		}
		methodType := &goast.FuncType{Params: &goast.FieldList{List: parameters}}
		methodType.Results = functionResults(method.ReturnType)
		body, err := generateBlock(method.Body)
		if err != nil {
			return nil, err
		}
		if method.Abstract {
			body = abstractMethodBody(class.Name, method.Name)
		}
		name := method.GoName
		if name == "" {
			name = memberName(method.Name, method.Visibility)
		}
		if len(method.TypeParameters) != 0 && !method.Static {
			generated, err := generateGenericMethodHelper(class.Name, class.TypeParameters, method, "this", classPointer)
			if err != nil {
				return nil, err
			}
			declarations = append(declarations, generated)
			continue
		}
		generated := &goast.FuncDecl{Name: goast.NewIdent(goName(name)), Type: methodType, Body: body}
		if method.Static {
			generated.Name = goast.NewIdent(staticMethodName(class.Name, name, method.Visibility))
			var staticTypeParameters []*goast.Field
			for _, parameter := range method.TypeParameters {
				staticTypeParameters = append(staticTypeParameters, goTypeParameterField(parameter, false))
			}
			if method.Accessor == "" {
				staticTypeParameters = append(staticTypeParameters, typeParameterFields...)
			}
			if len(staticTypeParameters) != 0 {
				generated.Type.TypeParams = &goast.FieldList{List: staticTypeParameters}
			}
		} else {
			generated.Recv = &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("this")}, Type: classPointer}}}
			if method.Virtual || method.Override {
				generated.Name = goast.NewIdent(virtualSlotName(method.VirtualOwner, name))
				if method.Virtual && !method.Override {
					declarations = append(declarations, generateVirtualWrapper(class, method, name))
				}
			}
		}
		declarations = append(declarations, generated)
	}
	return declarations, nil
}

func typeRefsForTypeParameters(parameters []kinmokuseiAST.TypeParameter) []kinmokuseiAST.TypeRef {
	result := make([]kinmokuseiAST.TypeRef, len(parameters))
	for index, parameter := range parameters {
		result[index] = kinmokuseiAST.TypeRef{Name: parameter.Name, TypeParameter: true, Span: parameter.Span}
	}
	return result
}

func typeRefForClass(class *kinmokuseiAST.ClassDecl) kinmokuseiAST.TypeRef {
	return kinmokuseiAST.TypeRef{Name: class.Name, GenericArguments: typeRefsForTypeParameters(class.TypeParameters), Span: class.NameSpan}
}

func goClassType(ref kinmokuseiAST.TypeRef) goast.Expr {
	return indexedGoType(goast.NewIdent(ref.Name), ref.GenericArguments)
}

func generateVirtualWrapper(class *kinmokuseiAST.ClassDecl, method *kinmokuseiAST.MethodDecl, name string) *goast.FuncDecl {
	className := class.Name
	parameters := make([]*goast.Field, 0, len(method.Parameters))
	arguments := make([]goast.Expr, 0, len(method.Parameters))
	for _, parameter := range method.Parameters {
		parameterName := goName(parameter.Name)
		parameters = append(parameters, goParameterField(parameter))
		arguments = append(arguments, goast.NewIdent(parameterName))
	}
	methodType := &goast.FuncType{Params: &goast.FieldList{List: parameters}, Results: functionResults(method.ReturnType)}
	directCall := &goast.CallExpr{
		Fun:  &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(virtualSlotName(className, name))},
		Args: arguments,
	}
	dispatchCall := &goast.CallExpr{
		Fun: &goast.SelectorExpr{
			X:   &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(virtualSelfName(className))},
			Sel: goast.NewIdent(virtualSlotName(className, name)),
		},
		Args: arguments,
	}
	if len(method.Parameters) != 0 && method.Parameters[len(method.Parameters)-1].Variadic {
		directCall.Ellipsis = token.Pos(1)
		dispatchCall.Ellipsis = token.Pos(1)
	}
	condition := &goast.BinaryExpr{
		X:  &goast.BinaryExpr{X: goast.NewIdent("this"), Op: token.EQL, Y: goast.NewIdent("nil")},
		Op: token.LOR,
		Y: &goast.BinaryExpr{
			X:  &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(virtualSelfName(className))},
			Op: token.EQL, Y: goast.NewIdent("nil"),
		},
	}
	body := &goast.BlockStmt{}
	if method.ReturnType.Name == "void" {
		body.List = append(body.List,
			&goast.IfStmt{Cond: condition, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ExprStmt{X: directCall}, &goast.ReturnStmt{},
			}}},
			&goast.ExprStmt{X: dispatchCall},
		)
	} else {
		body.List = append(body.List,
			&goast.IfStmt{Cond: condition, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ReturnStmt{Results: []goast.Expr{directCall}},
			}}},
			&goast.ReturnStmt{Results: []goast.Expr{dispatchCall}},
		)
	}
	return &goast.FuncDecl{
		Recv: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("this")}, Type: &goast.StarExpr{X: goClassType(typeRefForClass(class))}}}},
		Name: goast.NewIdent(goName(name)), Type: methodType, Body: body,
	}
}

func virtualInterfaceName(className string) string { return "__kinmokusei" + className + "Virtual" }
func virtualSelfName(className string) string      { return "__kinmokusei" + className + "Self" }
func virtualSlotName(owner, method string) string  { return "__kinmokusei" + owner + method }
func initializerName(className string) string      { return "__kinmokuseiInit" + className }
func upcastName(source, target string) string      { return "__kinmokuseiUpcast" + source + "To" + target }
func downcastName(source, target string) string {
	return "__kinmokuseiDowncast" + source + "To" + target
}
func mustDowncastName(source, target string) string {
	return "__kinmokuseiMustDowncast" + source + "To" + target
}
func publicUpcastName(source, target string) string   { return "Upcast" + source + "To" + target }
func publicDowncastName(source, target string) string { return "Downcast" + source + "To" + target }
func publicMustDowncastName(source, target string) string {
	return "MustDowncast" + source + "To" + target
}
func classRootName() string { return "__kinmokuseiRoot" }
func classProjectionInterfaceName(className string) string {
	return "__kinmokusei" + className + "Projection"
}
func classProjectionMethodName(className string) string { return "__kinmokuseiAs" + className }

func generateConversionWrapper(name, implementation string, source, target kinmokuseiAST.TypeRef, parameters []kinmokuseiAST.TypeParameter, checked bool) *goast.FuncDecl {
	results := []*goast.Field{{Type: goType(target)}}
	if checked {
		results = append(results, &goast.Field{Type: goast.NewIdent("bool")})
	}
	functionType := &goast.FuncType{
		Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("value")}, Type: goType(source)}}},
		Results: &goast.FieldList{List: results},
	}
	if len(parameters) != 0 {
		typeParameters := make([]*goast.Field, 0, len(parameters))
		for _, parameter := range parameters {
			typeParameters = append(typeParameters, goTypeParameterField(parameter, false))
		}
		functionType.TypeParams = &goast.FieldList{List: typeParameters}
	}
	callee := indexedGoType(goast.NewIdent(implementation), typeRefsForTypeParameters(parameters))
	return &goast.FuncDecl{
		Name: goast.NewIdent(name),
		Type: functionType,
		Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{&goast.CallExpr{
			Fun: callee, Args: []goast.Expr{goast.NewIdent("value")},
		}}}}},
	}
}
