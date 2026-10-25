package codegen

import (
	"encoding/json"
	goast "go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func nativeContractName(parameter string) string  { return "__kinmokuseiContract" + parameter }
func nativeImplementationName(name string) string { return "__kinmokuseiNative" + name }
func nativeBindingName(name string) string        { return "__kinmokuseiBind" + name }
func classContractField(owner string) string      { return "__kinmokuseiContracts" + owner }
func classContractMethod(owner string) string     { return "__kinmokuseiSource" + owner }

func nativeContractLiteral(value ast.NativeContract) (string, bool) {
	if value.Parameter != "" {
		return "", false
	}
	if value.Tag == "" {
		return "", true
	}
	quoted, _ := json.Marshal(value.Tag)
	key := "[" + string(quoted)
	for _, part := range value.Parts {
		keyPart, literal := nativeContractLiteral(part)
		if !literal {
			return "", false
		}
		if keyPart == "" {
			return "", true
		}
		key += "," + keyPart
	}
	return key + "]", true
}

func nativeContractExpression(value ast.NativeContract) goast.Expr {
	if value.Parameter != "" {
		return goast.NewIdent(nativeContractName(value.Parameter))
	}
	if key, literal := nativeContractLiteral(value); literal {
		return stringLiteral(key)
	}
	args := []goast.Expr{stringLiteral(strconv.Quote(value.Tag))}
	for _, part := range value.Parts {
		args = append(args, nativeContractExpression(part))
	}
	return &goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiSourceContract"), Args: args}
}

func nativeContractExpressions(values []ast.NativeContract) []goast.Expr {
	result := make([]goast.Expr, len(values))
	for i, value := range values {
		result[i] = nativeContractExpression(value)
	}
	return result
}

func contractArrayType(count int) *goast.ArrayType {
	return &goast.ArrayType{Len: &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(count)}, Elt: goast.NewIdent("string")}
}

func contractArray(values []ast.NativeContract) goast.Expr {
	return &goast.CompositeLit{Type: contractArrayType(len(values)), Elts: nativeContractExpressions(values)}
}

func contractParameters(parameters []ast.TypeParameter) []*goast.Field {
	result := make([]*goast.Field, len(parameters))
	for i, parameter := range parameters {
		result[i] = &goast.Field{Names: []*goast.Ident{goast.NewIdent(nativeContractName(parameter.Name))}, Type: goast.NewIdent("string")}
	}
	return result
}

// Keep the public Go signature. Native source calls use an implementation with
// explicit proof arguments. Go callers do not silently acquire a source proof.
func nativeGenericDeclarations(function *goast.FuncDecl, parameters []ast.TypeParameter) []goast.Decl {
	if len(parameters) == 0 {
		return []goast.Decl{function}
	}
	name := function.Name.Name
	originalType := function.Type
	// A wrapper forwards every parameter, including source blank parameters.
	// Give only those blanks private names; leave the implementation untouched.
	forwardType := *originalType
	forwardType.Params = &goast.FieldList{}
	used := map[string]bool{}
	goast.Inspect(originalType, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			used[name.Name] = true
		}
		return true
	})
	for i, field := range originalType.Params.List {
		copyField := *field
		copyField.Names = append([]*goast.Ident(nil), field.Names...)
		for j, name := range copyField.Names {
			if name.Name == "_" {
				fresh := "__kinmokuseiArgument" + strconv.Itoa(i) + "_" + strconv.Itoa(j)
				for used[fresh] {
					fresh += "_"
				}
				used[fresh] = true
				copyField.Names[j] = goast.NewIdent(fresh)
			}
		}
		forwardType.Params.List = append(forwardType.Params.List, &copyField)
	}
	implementationType := *originalType
	implementationType.Params = &goast.FieldList{List: append(contractParameters(parameters), originalType.Params.List...)}
	function.Name = goast.NewIdent(nativeImplementationName(name))
	function.Type = &implementationType
	arguments := func(proofs bool) []goast.Expr {
		args := make([]goast.Expr, 0, len(parameters)+len(originalType.Params.List))
		for _, parameter := range parameters {
			if proofs {
				args = append(args, goast.NewIdent(nativeContractName(parameter.Name)))
			} else {
				args = append(args, stringLiteral(""))
			}
		}
		for _, field := range forwardType.Params.List {
			for _, name := range field.Names {
				args = append(args, goast.NewIdent(name.Name))
			}
		}
		return args
	}
	delegate := func(proofs bool) *goast.BlockStmt {
		var typeArguments []ast.TypeRef
		for _, field := range originalType.TypeParams.List {
			for _, name := range field.Names {
				typeArguments = append(typeArguments, ast.TypeRef{Name: name.Name, TypeParameter: true})
			}
		}
		call := &goast.CallExpr{Fun: indexedGoType(goast.NewIdent(function.Name.Name), typeArguments), Args: arguments(proofs)}
		if len(originalType.Params.List) != 0 {
			if _, variadic := originalType.Params.List[len(originalType.Params.List)-1].Type.(*goast.Ellipsis); variadic {
				call.Ellipsis = token.Pos(1)
			}
		}
		if originalType.Results == nil || len(originalType.Results.List) == 0 {
			return &goast.BlockStmt{List: []goast.Stmt{&goast.ExprStmt{X: call}, &goast.ReturnStmt{}}}
		}
		return &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{call}}}}
	}
	wrapper := &goast.FuncDecl{Name: goast.NewIdent(name), Type: &forwardType, Body: delegate(false)}
	boundType := forwardType
	boundType.TypeParams = nil
	binder := &goast.FuncDecl{Name: goast.NewIdent(nativeBindingName(name)), Type: &goast.FuncType{
		TypeParams: originalType.TypeParams, Params: &goast.FieldList{List: contractParameters(parameters)}, Results: &goast.FieldList{List: []*goast.Field{{Type: &boundType}}},
	}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{&goast.FuncLit{Type: &boundType, Body: delegate(true)}}}}}}
	return []goast.Decl{function, wrapper, binder}
}

// Struct/defined-type layouts stay unchanged. Native direct calls carry the
// receiver's source arguments to a free implementation; the Go method retains
// its signature and forwards unknown proofs, rather than inventing qualifiers.
func nativeReceiverDeclarations(method *goast.FuncDecl, owner string, goParameters, proofParameters []ast.TypeParameter) []goast.Decl {
	if len(proofParameters) == 0 {
		return []goast.Decl{method}
	}
	helperType := *method.Type
	helperType.TypeParams = &goast.FieldList{}
	for _, parameter := range goParameters {
		helperType.TypeParams.List = append(helperType.TypeParams.List, goTypeParameterField(parameter, false))
	}
	helperType.Params = &goast.FieldList{List: append(append([]*goast.Field(nil), method.Recv.List...), method.Type.Params.List...)}
	helper := &goast.FuncDecl{Name: goast.NewIdent("__kinmokuseiReceiver" + owner + method.Name.Name), Type: &helperType, Body: method.Body}
	declarations := nativeGenericDeclarations(helper, proofParameters)
	wrapper := declarations[1].(*goast.FuncDecl)
	methodType := *wrapper.Type
	methodType.TypeParams = nil
	methodType.Params = &goast.FieldList{List: wrapper.Type.Params.List[1:]}
	method.Type, method.Body = &methodType, wrapper.Body
	return append(declarations, method)
}

func generateReceiverContractValue(member *ast.MemberExpr) (goast.Expr, error) {
	bound := &goast.CallExpr{Fun: indexedGoType(goast.NewIdent(nativeBindingName(member.ReceiverContractHelper)), member.ReceiverContractTypes), Args: nativeContractExpressions(member.ReceiverContractArguments)}
	if member.MethodExpressionReceiver != nil && !member.ReceiverContractDereference {
		return bound, nil
	}
	functionType := goType(*member.ReceiverContractType).(*goast.FuncType)
	var arguments []goast.Expr
	for i, field := range functionType.Params.List {
		name := goast.NewIdent("__kinmokuseiArgument" + strconv.Itoa(i))
		field.Names = []*goast.Ident{name}
		arguments = append(arguments, name)
	}
	if member.MethodExpressionReceiver != nil {
		arguments[0] = &goast.StarExpr{X: arguments[0]}
		call := &goast.CallExpr{Fun: bound, Args: arguments}
		if len(functionType.Params.List) > 0 {
			if _, variadic := functionType.Params.List[len(functionType.Params.List)-1].Type.(*goast.Ellipsis); variadic {
				call.Ellipsis = token.Pos(1)
			}
		}
		var invoke goast.Stmt = &goast.ExprStmt{X: call}
		if functionType.Results != nil && len(functionType.Results.List) > 0 {
			invoke = &goast.ReturnStmt{Results: []goast.Expr{call}}
		}
		return &goast.FuncLit{Type: functionType, Body: &goast.BlockStmt{List: []goast.Stmt{invoke}}}, nil
	}
	receiver, err := generateExpression(member.Object)
	if err != nil {
		return nil, err
	}
	if member.GenericReceiverAddress {
		receiver = &goast.UnaryExpr{Op: token.AND, X: receiver}
	}
	if member.ReceiverContractDereference {
		receiver = &goast.StarExpr{X: receiver}
	}
	call := &goast.CallExpr{Fun: bound, Args: append([]goast.Expr{goast.NewIdent("__kinmokuseiReceiver")}, arguments...)}
	if len(functionType.Params.List) > 0 {
		if _, variadic := functionType.Params.List[len(functionType.Params.List)-1].Type.(*goast.Ellipsis); variadic {
			call.Ellipsis = token.Pos(1)
		}
	}
	var invoke goast.Stmt = &goast.ExprStmt{X: call}
	if functionType.Results != nil && len(functionType.Results.List) > 0 {
		invoke = &goast.ReturnStmt{Results: []goast.Expr{call}}
	}
	factory := &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("__kinmokuseiReceiver")}, Type: goType(*member.ReceiverContractReceiver)}}}, Results: &goast.FieldList{List: []*goast.Field{{Type: functionType}}}}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{&goast.FuncLit{Type: functionType, Body: &goast.BlockStmt{List: []goast.Stmt{invoke}}}}}}}}
	return &goast.CallExpr{Fun: factory, Args: []goast.Expr{receiver}}, nil
}

func classContractLocals(owner string, parameters []ast.TypeParameter, receiver string) []goast.Stmt {
	var result []goast.Stmt
	for i, parameter := range parameters {
		value := &goast.IndexExpr{X: &goast.SelectorExpr{X: goast.NewIdent(goName(receiver)), Sel: goast.NewIdent(classContractField(owner))}, Index: &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(i)}}
		variable := goast.NewIdent(nativeContractName(parameter.Name))
		result = append(result,
			&goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{Names: []*goast.Ident{variable}, Type: goast.NewIdent("string")}}}},
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: goast.NewIdent(goName(receiver)), Op: token.NEQ, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.AssignStmt{Lhs: []goast.Expr{variable}, Tok: token.ASSIGN, Rhs: []goast.Expr{value}}}}},
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []goast.Expr{variable}})
	}
	return result
}

func nativeContractRuntimeDeclarations() []goast.Decl {
	file, err := parser.ParseFile(token.NewFileSet(), "native-contract-runtime.go", `package runtime
func __kinmokuseiSourceContract(tag string, parts ...string) string {
 key := "["+tag
 for _, part := range parts { if part == "" { return "" }; key += ","+part }
 return key+"]"
}

func __kinmokuseiKnownContracts(values []string) bool { for _, value := range values { if value == "" { return false } }; return true }
`, 0)
	if err != nil {
		panic(err)
	}
	return file.Decls
}

func classContractGuard(owner string) goast.Stmt {
	known := &goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiKnownContracts"), Args: []goast.Expr{&goast.SliceExpr{X: goast.NewIdent("contracts")}}}
	actual := &goast.CallExpr{Fun: &goast.SelectorExpr{X: goast.NewIdent("projected"), Sel: goast.NewIdent(classContractMethod(owner))}}
	mismatch := &goast.BinaryExpr{X: actual, Op: token.NEQ, Y: &goast.StarExpr{X: goast.NewIdent("contracts")}}
	reject := &goast.IfStmt{Cond: &goast.BinaryExpr{X: &goast.UnaryExpr{Op: token.NOT, X: known}, Op: token.LOR, Y: mismatch}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{goast.NewIdent("nil"), goast.NewIdent("false")}}}}}
	return &goast.IfStmt{Cond: &goast.BinaryExpr{X: goast.NewIdent("contracts"), Op: token.NEQ, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{reject}}}
}
