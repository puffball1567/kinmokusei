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

func decoratorContextDeclaration() goast.Decl {
	fields := []struct {
		name           string
		typeExpression goast.Expr
	}{
		{"Kind", goast.NewIdent("string")},
		{"Identity", goast.NewIdent("string")},
		{"ClassName", goast.NewIdent("string")},
		{"MemberName", goast.NewIdent("string")},
		{"ParameterName", goast.NewIdent("string")},
		{"ParameterIndex", goast.NewIdent("int")},
		{"Static", goast.NewIdent("bool")},
		{"Visibility", goast.NewIdent("string")},
		{"ValueType", goast.NewIdent("string")},
	}
	generated := make([]*goast.Field, 0, len(fields))
	for _, field := range fields {
		generated = append(generated, &goast.Field{Names: []*goast.Ident{goast.NewIdent(field.name)}, Type: field.typeExpression})
	}
	return &goast.GenDecl{Tok: token.TYPE, Specs: []goast.Spec{&goast.TypeSpec{
		Name: goast.NewIdent("__kinmokuseiDecoratorContext"),
		Type: &goast.StructType{Fields: &goast.FieldList{List: generated}},
	}}}
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
		{"ClassName", target.ClassName},
		{"MemberName", target.MemberName},
		{"ParameterName", target.ParameterName},
	}
	elements := make([]goast.Expr, 0, 9)
	for _, value := range values {
		elements = append(elements, &goast.KeyValueExpr{Key: goast.NewIdent(value.name), Value: stringLiteral(value.value)})
	}
	elements = append(elements,
		&goast.KeyValueExpr{Key: goast.NewIdent("ParameterIndex"), Value: &goast.BasicLit{Kind: token.INT, Value: parameterIndex}},
		&goast.KeyValueExpr{Key: goast.NewIdent("Static"), Value: goast.NewIdent(static)},
		&goast.KeyValueExpr{Key: goast.NewIdent("Visibility"), Value: stringLiteral(decoratorVisibility(target.Visibility))},
		&goast.KeyValueExpr{Key: goast.NewIdent("ValueType"), Value: stringLiteral(valueType)},
	)
	return &goast.CompositeLit{Type: goast.NewIdent("__kinmokuseiDecoratorContext"), Elts: elements}
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
