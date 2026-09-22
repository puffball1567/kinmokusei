package codegen

import (
	goast "go/ast"
	"go/token"
	"sort"
	"strconv"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func goTypeParameterField(parameter kinmokuseiAST.TypeParameter, inferredComparable bool) *goast.Field {
	var constraint goast.Expr = goast.NewIdent("any")
	if parameter.Constraint != nil {
		constraint = goConstraintType(*parameter.Constraint)
	}
	if inferredComparable {
		if parameter.Constraint == nil {
			constraint = goast.NewIdent("comparable")
		} else if parameter.Constraint.Qualifier != "" || parameter.Constraint.Name != "comparable" {
			constraint = &goast.InterfaceType{Methods: &goast.FieldList{List: []*goast.Field{
				{Type: constraint},
				{Type: goast.NewIdent("comparable")},
			}}}
		}
	}
	return &goast.Field{
		Names: []*goast.Ident{goast.NewIdent(goName(parameter.Name))},
		Type:  constraint,
	}
}

func goConstraintType(ref kinmokuseiAST.TypeRef) goast.Expr {
	if ref.Qualifier == "" && ref.Name == "comparable" {
		return goast.NewIdent("comparable")
	}
	return goType(ref)
}

func comparableTypeParameters(ref kinmokuseiAST.TypeRef) map[string]bool {
	result := map[string]bool{}
	collectComparableTypeParameters(ref, result)
	return result
}

func collectComparableTypeParameters(ref kinmokuseiAST.TypeRef, result map[string]bool) {
	if ref.Qualifier == "" && ref.Name == "Map" && len(ref.GenericArguments) == 2 {
		collectTypeParametersRequiringComparability(ref.GenericArguments[0], result)
	}
	if ref.Element != nil {
		collectComparableTypeParameters(*ref.Element, result)
	}
	if ref.Pointee != nil {
		collectComparableTypeParameters(*ref.Pointee, result)
	}
	for _, argument := range ref.GenericArguments {
		collectComparableTypeParameters(argument, result)
	}
	for _, parameter := range ref.Parameters {
		collectComparableTypeParameters(parameter, result)
	}
	if ref.Return != nil {
		collectComparableTypeParameters(*ref.Return, result)
	}
	for _, field := range ref.ObjectFields {
		collectComparableTypeParameters(field.Type, result)
	}
}

func collectTypeParametersRequiringComparability(ref kinmokuseiAST.TypeRef, result map[string]bool) {
	if ref.IsPointer() || ref.IsSlice() || ref.Name == "Map" || ref.IsFunction() || ref.Object || ref.GoStruct {
		return
	}
	if len(ref.GenericArguments) == 0 && ref.Element == nil {
		result[ref.Name] = true
		return
	}
	if ref.Element != nil {
		collectTypeParametersRequiringComparability(*ref.Element, result)
	}
	for _, argument := range ref.GenericArguments {
		collectTypeParametersRequiringComparability(argument, result)
	}
}

func goParameterField(parameter kinmokuseiAST.Parameter) *goast.Field {
	parameterType := goType(parameter.Type)
	if parameter.Variadic && parameter.Type.Element != nil {
		parameterType = &goast.Ellipsis{Elt: goType(*parameter.Type.Element)}
	}
	return &goast.Field{
		Names: []*goast.Ident{goast.NewIdent(goName(parameter.Name))},
		Type:  parameterType,
	}
}

func goStoredField(name, jsonName string, visibility kinmokuseiAST.Visibility, fieldType kinmokuseiAST.TypeRef) *goast.Field {
	field := &goast.Field{
		Names: []*goast.Ident{goast.NewIdent(goName(name))},
		Type:  goType(fieldType),
	}
	if visibility == kinmokuseiAST.Public {
		field.Tag = &goast.BasicLit{Kind: token.STRING, Value: "`json:\"" + jsonName + "\"`"}
	}
	return field
}

func goType(ref kinmokuseiAST.TypeRef) goast.Expr {
	if ref.Nullable {
		ref.Nullable = false
		return goType(ref)
	}
	if ref.LoweredType != nil {
		return goType(*ref.LoweredType)
	}
	if ref.Qualifier == "" && ref.Name == kinmokuseiAST.DecoratorContextTypeName {
		return goast.NewIdent("__kinmokuseiDecoratorContext")
	}
	if ref.Name == "Task" && len(ref.GenericArguments) == 1 {
		return taskGoType(ref.GenericArguments[0])
	}
	if ref.TypeParameter {
		return goast.NewIdent(goName(ref.Name))
	}
	if ref.NativeNamed {
		return indexedGoType(goast.NewIdent(ref.Name), ref.GenericArguments)
	}
	if ref.IsPointer() {
		return &goast.StarExpr{X: goType(*ref.Pointee)}
	}
	if ref.Interface {
		return indexedGoType(goast.NewIdent(ref.Name), ref.GenericArguments)
	}
	if ref.Struct {
		return indexedGoType(goast.NewIdent(ref.Name), ref.GenericArguments)
	}
	if ref.IsArray() {
		arrayType := &goast.ArrayType{Elt: goType(*ref.Element)}
		if ref.IsFixedArray() {
			arrayType.Len = &goast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(*ref.FixedLength, 10)}
		}
		return arrayType
	}
	if ref.Qualifier == "" && ref.Name == "Map" && len(ref.GenericArguments) == 2 {
		return &goast.MapType{Key: goType(ref.GenericArguments[0]), Value: goType(ref.GenericArguments[1])}
	}
	if len(ref.GenericArguments) == 1 {
		direction := goast.SEND | goast.RECV
		switch ref.Name {
		case "GoSendChannel":
			direction = goast.SEND
		case "GoReceiveChannel":
			direction = goast.RECV
		case "GoChannel":
		default:
			direction = 0
		}
		if direction != 0 {
			return &goast.ChanType{Dir: direction, Value: goType(ref.GenericArguments[0])}
		}
	}
	if ref.IsObject() {
		objectFields := append([]kinmokuseiAST.ObjectTypeField(nil), ref.ObjectFields...)
		sort.Slice(objectFields, func(left, right int) bool { return objectFields[left].Name < objectFields[right].Name })
		fields := make([]*goast.Field, 0, len(objectFields))
		for _, field := range objectFields {
			fieldName := memberName(field.Name, kinmokuseiAST.Public)
			generated := &goast.Field{Names: []*goast.Ident{goast.NewIdent(goName(fieldName))}, Type: goType(field.Type)}
			if field.JSONName != "" {
				generated.Tag = &goast.BasicLit{Kind: token.STRING, Value: "`json:\"" + field.JSONName + "\"`"}
			}
			fields = append(fields, generated)
		}
		return &goast.StructType{Fields: &goast.FieldList{List: fields}}
	}
	if ref.IsGoStruct() {
		fields := make([]*goast.Field, 0, len(ref.ObjectFields))
		for _, field := range ref.ObjectFields {
			generated := &goast.Field{Type: goType(field.Type)}
			if !field.GoEmbedded {
				generated.Names = []*goast.Ident{goast.NewIdent(field.Name)}
			}
			if field.GoTag != "" {
				generated.Tag = &goast.BasicLit{Kind: token.STRING, Value: strconv.Quote(field.GoTag)}
			}
			fields = append(fields, generated)
		}
		return &goast.StructType{Fields: &goast.FieldList{List: fields}}
	}
	if ref.GoInterface {
		methods := make([]*goast.Field, len(ref.ObjectFields))
		for i, method := range ref.ObjectFields {
			name := method.Name
			if !ref.Go {
				name = memberName(name, kinmokuseiAST.Public)
			}
			methods[i] = &goast.Field{Names: []*goast.Ident{goast.NewIdent(name)}, Type: goType(method.Type)}
		}
		return &goast.InterfaceType{Methods: &goast.FieldList{List: methods}}
	}
	if ref.IsFunction() {
		parameters := make([]*goast.Field, len(ref.Parameters))
		for i, parameter := range ref.Parameters {
			parameterType := goType(parameter)
			if ref.Variadic && i == len(ref.Parameters)-1 && parameter.Element != nil {
				parameterType = &goast.Ellipsis{Elt: goType(*parameter.Element)}
			}
			parameters[i] = &goast.Field{Type: parameterType}
		}
		fn := &goast.FuncType{Params: &goast.FieldList{List: parameters}}
		fn.Results = functionResults(*ref.Return)
		return fn
	}
	if ref.Qualifier != "" {
		var qualified goast.Expr = &goast.SelectorExpr{X: goast.NewIdent(goName(ref.Qualifier)), Sel: goast.NewIdent(ref.Name)}
		if len(ref.GenericArguments) == 1 {
			return &goast.IndexExpr{X: qualified, Index: goType(ref.GenericArguments[0])}
		}
		if len(ref.GenericArguments) > 1 {
			arguments := make([]goast.Expr, len(ref.GenericArguments))
			for i := range ref.GenericArguments {
				arguments[i] = goType(ref.GenericArguments[i])
			}
			return &goast.IndexListExpr{X: qualified, Indices: arguments}
		}
		return qualified
	}
	if ref.Go {
		return indexedGoType(goast.NewIdent(ref.Name), ref.GenericArguments)
	}
	name := goTypeName(ref.Name)
	if isGoBuiltinType(name) {
		return goast.NewIdent(name)
	}
	return &goast.StarExpr{X: indexedGoType(goast.NewIdent(name), ref.GenericArguments)}
}

func indexedGoType(base goast.Expr, arguments []kinmokuseiAST.TypeRef) goast.Expr {
	if len(arguments) == 1 {
		return &goast.IndexExpr{X: base, Index: goType(arguments[0])}
	}
	if len(arguments) > 1 {
		indices := make([]goast.Expr, len(arguments))
		for index := range arguments {
			indices[index] = goType(arguments[index])
		}
		return &goast.IndexListExpr{X: base, Indices: indices}
	}
	return base
}

func taskGoType(result kinmokuseiAST.TypeRef) goast.Expr {
	resultTask := result.Name == "Result" && len(result.GenericArguments) == 1
	value := result
	if resultTask {
		value = result.GenericArguments[0]
	}
	if value.Name == "void" {
		name := "__kinmokuseiVoidTask"
		if resultTask {
			name = "__kinmokuseiVoidResultTask"
		}
		return &goast.StarExpr{X: goast.NewIdent(name)}
	}
	name := "__kinmokuseiTask"
	if resultTask {
		name = "__kinmokuseiResultTask"
	}
	return &goast.StarExpr{X: &goast.IndexExpr{X: goast.NewIdent(name), Index: goType(value)}}
}

func isGoBuiltinType(name string) bool {
	switch name {
	case "bool", "string", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "complex64", "complex128", "byte", "error":
		return true
	default:
		return false
	}
}
