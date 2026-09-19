package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func generateDeclaration(decl kinmokuseiAST.Declaration) ([]goast.Decl, error) {
	switch decl := decl.(type) {
	case *kinmokuseiAST.FunctionDecl:
		params := make([]*goast.Field, 0, len(decl.Parameters))
		for _, param := range decl.Parameters {
			params = append(params, goParameterField(param))
		}
		fnType := &goast.FuncType{Params: &goast.FieldList{List: params}}
		if len(decl.TypeParameters) != 0 {
			typeParameters := make([]*goast.Field, 0, len(decl.TypeParameters))
			for _, parameter := range decl.TypeParameters {
				typeParameters = append(typeParameters, goTypeParameterField(parameter, false))
			}
			fnType.TypeParams = &goast.FieldList{List: typeParameters}
		}
		fnType.Results = functionResults(decl.ReturnType)
		body, err := generateBlock(decl.Body)
		if err != nil {
			return nil, err
		}
		return []goast.Decl{&goast.FuncDecl{Name: goast.NewIdent(goName(decl.Name)), Type: fnType, Body: body}}, nil
	case *kinmokuseiAST.MethodDecl:
		generated, err := generateNativeStructMethod(decl, decl.ReceiverName, goType(decl.ReceiverType))
		if err != nil {
			return nil, err
		}
		return []goast.Decl{generated}, nil
	case *kinmokuseiAST.VariableDecl:
		value, err := generateExpression(decl.Value)
		if err != nil {
			return nil, err
		}
		if decl.FunctionBinding {
			function, ok := value.(*goast.FuncLit)
			if !ok {
				return nil, fmt.Errorf("function binding %q does not contain an arrow", decl.Name)
			}
			return []goast.Decl{&goast.FuncDecl{Name: goast.NewIdent(goName(decl.Name)), Type: function.Type, Body: function.Body}}, nil
		}
		tok := token.VAR
		if decl.Constant && isGoConstant(decl.Value) {
			tok = token.CONST
		}
		spec := &goast.ValueSpec{Names: []*goast.Ident{goast.NewIdent(goName(decl.Name))}, Values: []goast.Expr{value}}
		if decl.Type.Name != "" || decl.Type.IsFunction() || decl.Type.IsPointer() || decl.Type.IsArray() {
			spec.Type = goType(decl.Type)
		}
		return []goast.Decl{&goast.GenDecl{Tok: tok, Specs: []goast.Spec{spec}}}, nil
	case *kinmokuseiAST.CABIExportDecl:
		return nil, nil
	case *kinmokuseiAST.ClassDecl:
		return generateClass(decl)
	case *kinmokuseiAST.StructDecl:
		fields := make([]*goast.Field, 0, len(decl.Fields))
		for _, field := range decl.Fields {
			name := field.GoName
			if name == "" {
				name = memberName(field.Name, field.Visibility)
			}
			fields = append(fields, goStoredField(name, field.Name, field.Visibility, field.Type))
		}
		typeSpec := &goast.TypeSpec{Name: goast.NewIdent(decl.Name), Type: &goast.StructType{Fields: &goast.FieldList{List: fields}}}
		if len(decl.TypeParameters) != 0 {
			typeFields := make([]*goast.Field, 0, len(decl.TypeParameters))
			for _, parameter := range decl.TypeParameters {
				typeFields = append(typeFields, goTypeParameterField(parameter, false))
			}
			typeSpec.TypeParams = &goast.FieldList{List: typeFields}
		}
		declarations := []goast.Decl{&goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{typeSpec}}}
		receiverArguments := make([]goast.Expr, len(decl.TypeParameters))
		for index, parameter := range decl.TypeParameters {
			receiverArguments[index] = goast.NewIdent(parameter.Name)
		}
		for _, method := range decl.Methods {
			receiver := goast.Expr(goast.NewIdent(decl.Name))
			if len(receiverArguments) == 1 {
				receiver = &goast.IndexExpr{X: receiver, Index: receiverArguments[0]}
			} else if len(receiverArguments) > 1 {
				receiver = &goast.IndexListExpr{X: receiver, Indices: receiverArguments}
			}
			if method.PointerReceiver {
				receiver = &goast.StarExpr{X: receiver}
			}
			if len(method.TypeParameters) != 0 {
				generated, err := generateGenericMethodHelper(decl.Name, decl.TypeParameters, method, "this", receiver)
				if err != nil {
					return nil, err
				}
				declarations = append(declarations, generated)
				continue
			}
			generated, err := generateNativeStructMethod(method, "this", receiver)
			if err != nil {
				return nil, err
			}
			declarations = append(declarations, generated)
		}
		return declarations, nil
	case *kinmokuseiAST.TypeDecl:
		if decl.Alias && len(decl.TypeParameters) != 0 {
			return nil, nil
		}
		spec := &goast.TypeSpec{Name: goast.NewIdent(decl.Name), Type: goType(decl.Underlying)}
		if decl.Alias {
			spec.Assign = token.Pos(1)
		}
		if len(decl.TypeParameters) != 0 {
			typeFields := make([]*goast.Field, 0, len(decl.TypeParameters))
			comparable := comparableTypeParameters(decl.Underlying)
			for _, parameter := range decl.TypeParameters {
				typeFields = append(typeFields, goTypeParameterField(parameter, comparable[parameter.Name]))
			}
			spec.TypeParams = &goast.FieldList{List: typeFields}
		}
		return []goast.Decl{&goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{spec}}}, nil
	case *kinmokuseiAST.EnumDecl:
		typeDeclaration := &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{&goast.TypeSpec{
			Name: goast.NewIdent(decl.Name), Type: goType(decl.Underlying),
		}}}
		constants := make([]goast.Spec, 0, len(decl.Members))
		for _, member := range decl.Members {
			constants = append(constants, &goast.ValueSpec{
				Names:  []*goast.Ident{goast.NewIdent(enumMemberGoName(decl.Name, member.Name))},
				Type:   goast.NewIdent(decl.Name),
				Values: []goast.Expr{goIntegerConstant(member.ResolvedValue)},
			})
		}
		return []goast.Decl{typeDeclaration, &goast.GenDecl{Tok: token.CONST, Specs: constants}}, nil
	case *kinmokuseiAST.InterfaceDecl:
		if decl.Constraint {
			var union goast.Expr
			methods := []*goast.Field{}
			for _, term := range decl.Terms {
				termType := goConstraintType(term.Type)
				if term.Underlying {
					termType = &goast.UnaryExpr{Op: token.TILDE, X: termType}
				}
				if decl.Intersection {
					methods = append(methods, &goast.Field{Type: termType})
					continue
				}
				if union == nil {
					union = termType
				} else {
					union = &goast.BinaryExpr{X: union, Op: token.OR, Y: termType}
				}
			}
			if union != nil {
				methods = append(methods, &goast.Field{Type: union})
			}
			typeSpec := &goast.TypeSpec{Name: goast.NewIdent(decl.Name), Type: &goast.InterfaceType{Methods: &goast.FieldList{List: methods}}}
			if len(decl.TypeParameters) != 0 {
				typeSpec.TypeParams = &goast.FieldList{}
				for _, parameter := range decl.TypeParameters {
					typeSpec.TypeParams.List = append(typeSpec.TypeParams.List, goTypeParameterField(parameter, false))
				}
			}
			return []goast.Decl{&goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{typeSpec}}}, nil
		}
		methods := make([]*goast.Field, 0, len(decl.Methods))
		for _, base := range decl.Bases {
			methods = append(methods, &goast.Field{Type: goType(base)})
		}
		for _, method := range decl.Methods {
			parameters := make([]*goast.Field, 0, len(method.Parameters))
			for _, parameter := range method.Parameters {
				parameters = append(parameters, goParameterField(parameter))
			}
			methodType := &goast.FuncType{Params: &goast.FieldList{List: parameters}}
			methodType.Results = functionResults(method.ReturnType)
			name := method.GoName
			if name == "" {
				name = memberName(method.Name, kinmokuseiAST.Public)
			}
			methods = append(methods, &goast.Field{Names: []*goast.Ident{goast.NewIdent(name)}, Type: methodType})
		}
		typeSpec := &goast.TypeSpec{Name: goast.NewIdent(decl.Name), Type: &goast.InterfaceType{Methods: &goast.FieldList{List: methods}}}
		if len(decl.TypeParameters) != 0 {
			typeFields := make([]*goast.Field, 0, len(decl.TypeParameters))
			for _, parameter := range decl.TypeParameters {
				typeFields = append(typeFields, goTypeParameterField(parameter, false))
			}
			typeSpec.TypeParams = &goast.FieldList{List: typeFields}
		}
		return []goast.Decl{&goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{typeSpec}}}, nil
	default:
		return nil, fmt.Errorf("unsupported declaration %T", decl)
	}
}

func generateNativeStructMethod(method *kinmokuseiAST.MethodDecl, receiverName string, receiver goast.Expr) (*goast.FuncDecl, error) {
	parameters := make([]*goast.Field, 0, len(method.Parameters))
	for _, parameter := range method.Parameters {
		parameters = append(parameters, goParameterField(parameter))
	}
	methodType := &goast.FuncType{Params: &goast.FieldList{List: parameters}, Results: functionResults(method.ReturnType)}
	body, err := generateBlock(method.Body)
	if err != nil {
		return nil, err
	}
	name := method.GoName
	if name == "" {
		name = memberName(method.Name, method.Visibility)
	}
	return &goast.FuncDecl{
		Recv: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent(goName(receiverName))}, Type: receiver}}},
		Name: goast.NewIdent(goName(name)), Type: methodType, Body: body,
	}, nil
}

func generateGenericMethodHelper(owner string, ownerTypeParameters []kinmokuseiAST.TypeParameter, method *kinmokuseiAST.MethodDecl, receiverName string, receiver goast.Expr) (*goast.FuncDecl, error) {
	parameters := []*goast.Field{{Names: []*goast.Ident{goast.NewIdent(goName(receiverName))}, Type: receiver}}
	for _, parameter := range method.Parameters {
		parameters = append(parameters, goParameterField(parameter))
	}
	functionType := &goast.FuncType{Params: &goast.FieldList{List: parameters}, Results: functionResults(method.ReturnType), TypeParams: &goast.FieldList{}}
	for _, parameter := range method.TypeParameters {
		functionType.TypeParams.List = append(functionType.TypeParams.List, goTypeParameterField(parameter, false))
	}
	for _, parameter := range ownerTypeParameters {
		functionType.TypeParams.List = append(functionType.TypeParams.List, goTypeParameterField(parameter, false))
	}
	body, err := generateBlock(method.Body)
	if err != nil {
		return nil, err
	}
	name := method.GoName
	if name == "" {
		name = memberName(method.Name, method.Visibility)
	}
	return &goast.FuncDecl{
		Name: goast.NewIdent(staticMethodName(owner, name, method.Visibility)),
		Type: functionType,
		Body: body,
	}, nil
}
