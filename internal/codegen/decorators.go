package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func decoratorRuntimeDeclarations() []goast.Decl {
	valueDeclaration := &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{&goast.TypeSpec{
		Name: goast.NewIdent("__kinmokuseiDecoratorValue"),
		Type: &goast.StructType{Fields: &goast.FieldList{List: []*goast.Field{
			{Names: []*goast.Ident{goast.NewIdent("TypeIdentity")}, Type: goast.NewIdent("string")},
			{Names: []*goast.Ident{goast.NewIdent("contract")}, Type: goast.NewIdent("string")},
			{Names: []*goast.Ident{goast.NewIdent("value")}, Type: goast.NewIdent("any")},
		}}},
	}}}
	errorDeclaration := &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{&goast.TypeSpec{
		Name: goast.NewIdent("__kinmokuseiDecoratorAdapterError"), Type: goast.NewIdent("string"),
	}}}
	errorMethod := &goast.FuncDecl{
		Recv: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("err")}, Type: goast.NewIdent("__kinmokuseiDecoratorAdapterError")}}},
		Name: goast.NewIdent("Error"),
		Type: &goast.FuncType{Params: &goast.FieldList{}, Results: &goast.FieldList{List: []*goast.Field{{Type: goast.NewIdent("string")}}}},
		Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{&goast.CallExpr{Fun: goast.NewIdent("string"), Args: []goast.Expr{goast.NewIdent("err")}}}}}},
	}
	fields := kinmokuseiAST.DecoratorContextFields()
	generated := make([]*goast.Field, 0, len(fields))
	for _, field := range fields {
		generated = append(generated, &goast.Field{
			Names: []*goast.Ident{goast.NewIdent(memberName(field.Name, kinmokuseiAST.Public))},
			Type:  goType(field.Type),
		})
	}
	contextDeclaration := &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{&goast.TypeSpec{
		Name: goast.NewIdent("__kinmokuseiDecoratorContext"),
		Type: &goast.StructType{Fields: &goast.FieldList{List: generated}},
	}}}
	return []goast.Decl{valueDeclaration, errorDeclaration, errorMethod, contextDeclaration}
}

// Decorator expressions are evaluated from top to bottom. Callbacks for the
// same target are then invoked from bottom to top, matching TypeScript's local
// decorator composition order without executing user code in the compiler.
func generateDecoratorInit(program *kinmokuseiAST.Program) (*goast.FuncDecl, error) {
	if len(program.Decorators) == 0 {
		return nil, nil
	}
	var statements []goast.Stmt
	for start := 0; start < len(program.Decorators); {
		first := program.Decorators[start]
		if first.Target == nil {
			return nil, fmt.Errorf("decorator at index %d has no checked target", start)
		}
		end := start + 1
		for end < len(program.Decorators) && program.Decorators[end].Target != nil && program.Decorators[end].Target.Identity == first.Target.Identity {
			end++
		}
		for index := start; index < end; index++ {
			factory, err := generateExpression(program.Decorators[index].Expression)
			if err != nil {
				return nil, fmt.Errorf("generate decorator factory %d: %w", index, err)
			}
			statements = append(statements, &goast.AssignStmt{
				Lhs: []goast.Expr{goast.NewIdent(decoratorLocalName(index))},
				Tok: token.DEFINE,
				Rhs: []goast.Expr{factory},
			})
		}
		for index := end - 1; index >= start; index-- {
			target := program.Decorators[index].Target
			statements = append(statements, &goast.ExprStmt{X: &goast.CallExpr{
				Fun:  goast.NewIdent(decoratorLocalName(index)),
				Args: []goast.Expr{decoratorContextLiteral(target)},
			}})
		}
		start = end
	}
	return &goast.FuncDecl{
		Name: goast.NewIdent("init"),
		Type: &goast.FuncType{Params: &goast.FieldList{}},
		Body: &goast.BlockStmt{List: statements},
	}, nil
}

func decoratorLocalName(index int) string {
	return fmt.Sprintf("__kinmokuseiDecorator%d", index)
}

func decoratorContextLiteral(target *kinmokuseiAST.DecoratorTarget) goast.Expr {
	parameterIndex := strconv.Itoa(target.ParameterIndex)
	static := "false"
	if target.Static {
		static = "true"
	}
	valueType := ""
	if target.ValueType != nil {
		valueType = decoratorTypeLabel(*target.ValueType)
	}
	values := []struct{ name, value string }{
		{"Kind", target.Kind},
		{"Identity", target.Identity},
		{"ClassIdentity", target.ClassIdentity},
		{"BaseIdentity", target.BaseIdentity},
		{"ClassName", target.ClassName},
		{"MemberName", target.MemberName},
		{"ParameterName", target.ParameterName},
	}
	elements := make([]goast.Expr, 0, len(kinmokuseiAST.DecoratorContextFields()))
	for _, value := range values {
		elements = append(elements, &goast.KeyValueExpr{Key: goast.NewIdent(value.name), Value: stringLiteral(value.value)})
	}
	elements = append(elements,
		&goast.KeyValueExpr{Key: goast.NewIdent("OverrideChain"), Value: decoratorStringSlice(target.OverrideChain)},
		&goast.KeyValueExpr{Key: goast.NewIdent("ParameterIndex"), Value: &goast.BasicLit{Kind: token.INT, Value: parameterIndex}},
		&goast.KeyValueExpr{Key: goast.NewIdent("Static"), Value: goast.NewIdent(static)},
		&goast.KeyValueExpr{Key: goast.NewIdent("Visibility"), Value: stringLiteral(decoratorVisibility(target.Visibility))},
		&goast.KeyValueExpr{Key: goast.NewIdent("ValueType"), Value: stringLiteral(valueType)},
		&goast.KeyValueExpr{Key: goast.NewIdent("ValueIdentity"), Value: stringLiteral(target.ValueIdentity)},
		&goast.KeyValueExpr{Key: goast.NewIdent("Constructible"), Value: goast.NewIdent(strconv.FormatBool(target.Constructible))},
		&goast.KeyValueExpr{Key: goast.NewIdent("ConstructUnavailableReason"), Value: stringLiteral(decoratorConstructUnavailableReason(target))},
		&goast.KeyValueExpr{Key: goast.NewIdent("Construct"), Value: decoratorConstructAdapter(target)},
		&goast.KeyValueExpr{Key: goast.NewIdent("Invocable"), Value: goast.NewIdent(strconv.FormatBool(target.Invocable))},
		&goast.KeyValueExpr{Key: goast.NewIdent("InvokeUnavailableReason"), Value: stringLiteral(decoratorInvokeUnavailableReason(target))},
		&goast.KeyValueExpr{Key: goast.NewIdent("Invoke"), Value: decoratorInvokeAdapter(target)},
		&goast.KeyValueExpr{Key: goast.NewIdent("StaticInvocable"), Value: goast.NewIdent(strconv.FormatBool(target.StaticInvocable))},
		&goast.KeyValueExpr{Key: goast.NewIdent("StaticInvokeUnavailableReason"), Value: stringLiteral(decoratorStaticInvokeUnavailableReason(target))},
		&goast.KeyValueExpr{Key: goast.NewIdent("InvokeStatic"), Value: decoratorStaticInvokeAdapter(target)},
	)
	return &goast.CompositeLit{Type: goast.NewIdent("__kinmokuseiDecoratorContext"), Elts: elements}
}

func decoratorConstructUnavailableReason(target *kinmokuseiAST.DecoratorTarget) string {
	if target.Constructible {
		return ""
	}
	if target.ConstructUnavailableReason != "" {
		return target.ConstructUnavailableReason
	}
	return "decorator target is not constructible"
}

func decoratorConstructAdapter(target *kinmokuseiAST.DecoratorTarget) goast.Expr {
	arguments := goast.NewIdent("arguments")
	valueType := goast.NewIdent("__kinmokuseiDecoratorValue")
	functionType := &goast.FuncType{
		Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{arguments}, Type: &goast.ArrayType{Elt: valueType}}}},
		Results: &goast.FieldList{List: []*goast.Field{{Type: valueType}, {Type: goast.NewIdent("error")}}},
	}
	failure := func(message string) *goast.ReturnStmt {
		return &goast.ReturnStmt{Results: []goast.Expr{
			&goast.CompositeLit{Type: valueType},
			&goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiDecoratorAdapterError"), Args: []goast.Expr{stringLiteral(message)}},
		}}
	}
	body := &goast.BlockStmt{}
	if !target.Constructible {
		body.List = append(body.List, failure(decoratorConstructUnavailableReason(target)))
		return &goast.FuncLit{Type: functionType, Body: body}
	}
	checks, constructorArguments := decoratorCheckedArguments("decorator constructor for "+target.ClassName, target.ConstructorParameters, target.ConstructorContracts, target.ConstructorVariadic, arguments, failure)
	body.List = append(body.List, checks...)
	constructed := &goast.CallExpr{Fun: goast.NewIdent("New" + target.RuntimeClassName), Args: constructorArguments}
	if target.ConstructorVariadic {
		constructed.Ellipsis = token.Pos(1)
	}
	body.List = append(body.List, &goast.ReturnStmt{Results: []goast.Expr{
		decoratorBox(constructed, kinmokuseiAST.TypeRef{Name: target.RuntimeClassName}, target.ClassIdentity, target.ClassContract),
		goast.NewIdent("nil"),
	}})
	return &goast.FuncLit{Type: functionType, Body: body}
}

func decoratorStringSlice(values []string) goast.Expr {
	elements := make([]goast.Expr, len(values))
	for index, value := range values {
		elements[index] = stringLiteral(value)
	}
	return &goast.CompositeLit{Type: &goast.ArrayType{Elt: goast.NewIdent("string")}, Elts: elements}
}

func stringLiteral(value string) goast.Expr {
	return &goast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func decoratorVisibility(visibility kinmokuseiAST.Visibility) string {
	switch visibility {
	case kinmokuseiAST.Private:
		return "private"
	case kinmokuseiAST.Protected:
		return "protected"
	case kinmokuseiAST.Public:
		return "public"
	default:
		return ""
	}
}

func decoratorTypeLabel(ref kinmokuseiAST.TypeRef) string {
	if ref.Nullable {
		ref.Nullable = false
		return decoratorTypeLabel(ref) + " | null"
	}
	if ref.IsPointer() && ref.Pointee != nil {
		return "*" + decoratorTypeLabel(*ref.Pointee)
	}
	if ref.IsArray() && ref.Element != nil {
		if ref.FixedLength != nil {
			return fmt.Sprintf("[%d]%s", *ref.FixedLength, decoratorTypeLabel(*ref.Element))
		}
		return decoratorTypeLabel(*ref.Element) + "[]"
	}
	if ref.IsFunction() && ref.Return != nil {
		parameters := make([]string, len(ref.Parameters))
		for index, parameter := range ref.Parameters {
			parameters[index] = decoratorTypeLabel(parameter)
			if ref.Variadic && index == len(ref.Parameters)-1 {
				parameters[index] = "..." + parameters[index]
			}
		}
		return "(" + strings.Join(parameters, ", ") + ") => " + decoratorTypeLabel(*ref.Return)
	}
	if len(ref.GoResults) != 0 {
		results := make([]string, len(ref.GoResults))
		for index, result := range ref.GoResults {
			results[index] = decoratorTypeLabel(result)
		}
		return "(" + strings.Join(results, ", ") + ")"
	}
	if ref.IsObject() || ref.IsGoStruct() || ref.GoInterface {
		fields := append([]kinmokuseiAST.ObjectTypeField(nil), ref.ObjectFields...)
		sort.Slice(fields, func(left, right int) bool { return fields[left].Name < fields[right].Name })
		parts := make([]string, len(fields))
		for index, field := range fields {
			parts[index] = field.Name + ": " + decoratorTypeLabel(field.Type)
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	}
	name := ref.Name
	if ref.Qualifier != "" {
		name = ref.Qualifier + "." + name
	}
	if len(ref.GenericArguments) != 0 {
		arguments := make([]string, len(ref.GenericArguments))
		for index, argument := range ref.GenericArguments {
			arguments[index] = decoratorTypeLabel(argument)
		}
		name += "<" + strings.Join(arguments, ", ") + ">"
	}
	if name == "" {
		return "unknown"
	}
	return name
}
