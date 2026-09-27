package sema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) checkDecoratorValueBuiltin(expr *ast.CallExpr, unwrap bool) Type {
	c.usesDecoratorContext = true
	name := "decoratorValue"
	if unwrap {
		name = "decoratorValueAs"
	}
	if expr.Expanded {
		c.report(expr.Span, name+" does not accept spread arguments")
	}
	if len(expr.Arguments) != 1 {
		c.report(expr.Span, fmt.Sprintf("%s expects 1 argument, got %d", name, len(expr.Arguments)))
	}
	if unwrap {
		return c.checkDecoratorValueUnwrap(expr)
	}
	if len(expr.TypeArguments) > 1 {
		c.report(expr.Span, "decoratorValue accepts at most one explicit type argument")
	}
	value := Type{Kind: Invalid, Name: "<invalid>"}
	if len(expr.TypeArguments) == 0 && len(expr.Arguments) != 0 {
		value = defaultLiteralType(c.singleValue(c.checkExpression(expr.Arguments[0]), expr.Arguments[0].GetSpan()))
	}
	if len(expr.TypeArguments) == 1 {
		expected := c.resolveType(expr.TypeArguments[0])
		if len(expr.Arguments) != 0 {
			actual := c.checkExpressionExpectedSlot(&expr.Arguments[0], expected)
			c.requireAssignable(expected, actual, expr.Arguments[0].GetSpan())
		}
		value = expected
	}
	if !decoratorValuePayloadType(value) {
		if decoratorValueHasTypeParameter(value) {
			c.report(expr.Span, "DecoratorValue requires a concrete payload type; open type parameters cannot preserve source contracts")
		} else if value.Kind != Invalid {
			c.report(expr.Span, fmt.Sprintf("type %s cannot be stored in DecoratorValue", value.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	c.prepareGoTypeForEmission(&value, expr.Span)
	ref := typeRefFromType(value, expr.Span)
	expr.ResolvedTypeArguments = []ast.TypeRef{ref}
	expr.DecoratorValueIdentity = decoratorValueRuntimeIdentity(value)
	expr.DecoratorValueContract = decoratorValueContract(value)
	expr.Builtin = ast.DecoratorValueCall
	expr.Signature = &ast.CallableSignature{ParameterNames: []string{"value"}, ParameterTypes: []string{value.String()}, Result: ast.DecoratorValueTypeName}
	return decoratorValueType()
}

func (c *Checker) checkDecoratorValueUnwrap(expr *ast.CallExpr) Type {
	if len(expr.TypeArguments) != 1 {
		c.report(expr.Span, "decoratorValueAs expects exactly one explicit type argument")
		for _, argument := range expr.Arguments {
			c.checkExpression(argument)
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	target := c.resolveType(expr.TypeArguments[0])
	if !decoratorValuePayloadType(target) {
		if decoratorValueHasTypeParameter(target) {
			c.report(expr.TypeArguments[0].Span, "DecoratorValue requires a concrete payload type; open type parameters cannot preserve source contracts")
		} else if target.Kind != Invalid {
			c.report(expr.TypeArguments[0].Span, fmt.Sprintf("type %s cannot be extracted from DecoratorValue", target.String()))
		}
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	if len(expr.Arguments) != 0 {
		actual := c.checkExpressionExpectedSlot(&expr.Arguments[0], decoratorValueType())
		c.requireAssignable(decoratorValueType(), actual, expr.Arguments[0].GetSpan())
	}
	c.prepareGoTypeForEmission(&target, expr.Span)
	ref := typeRefFromType(target, expr.Span)
	expr.ResolvedTypeArguments = []ast.TypeRef{ref}
	expr.DecoratorValueIdentity = decoratorValueRuntimeIdentity(target)
	expr.DecoratorValueContract = decoratorValueContract(target)
	expr.Builtin = ast.DecoratorValueAsCall
	result := Type{Kind: Result, Name: "Result", Element: &target}
	expr.Signature = &ast.CallableSignature{ParameterNames: []string{"value"}, ParameterTypes: []string{ast.DecoratorValueTypeName}, Result: result.String()}
	return result
}

func decoratorValuePayloadType(value Type) bool {
	if decoratorValueHasTypeParameter(value) {
		return false
	}
	switch value.Kind {
	case Invalid, Void, Result, Task, MultiValue, GoPackage, GoTypeName, Nil, Null, UntypedInt:
		return false
	default:
		return true
	}
}

// An open type parameter cannot preserve source nullability after Go erasure.
// Require a concrete storage contract instead of pretending a Go assertion is
// sufficient. Concrete generic instantiations remain supported.
func decoratorValueHasTypeParameter(value Type) bool {
	if value.Kind == TypeParameter {
		return true
	}
	for _, child := range []*Type{value.Element, value.Key, value.Result} {
		if child != nil && decoratorValueHasTypeParameter(*child) {
			return true
		}
	}
	for _, children := range [][]Type{value.Parameters, value.Results, value.TypeArguments} {
		for _, child := range children {
			if decoratorValueHasTypeParameter(child) {
				return true
			}
		}
	}
	for _, child := range value.Fields {
		if decoratorValueHasTypeParameter(child) {
			return true
		}
	}
	for _, method := range value.GoMethods {
		if decoratorValueHasTypeParameter(method.Type) {
			return true
		}
	}
	for _, field := range value.GoFields {
		if decoratorValueHasTypeParameter(field.Type) {
			return true
		}
	}
	return false
}

// Diagnostic type labels are not serialization: e.g. a nullable callback and a
// callback returning nullable values may print alike. Use a structural private
// contract, independently of the public typeIdentity and the erased Go type.
func decoratorValueContract(value Type) string {
	type contract struct {
		Kind     TypeKind
		Name     string            `json:",omitempty"`
		Parts    []string          `json:",omitempty"`
		Fields   map[string]string `json:",omitempty"`
		Variadic bool              `json:",omitempty"`
		Length   int64             `json:",omitempty"`
	}
	c := contract{Kind: value.Kind, Variadic: value.Variadic, Length: value.Length}
	switch value.Kind {
	case Class, Struct, Interface, TypeParameter:
		c.Name = value.Name
	case GoNamed, GoBasic:
		if value.GoType != nil {
			c.Name = gotypes.TypeString(gotypes.Unalias(value.GoType), func(p *gotypes.Package) string { return p.Path() })
		} else {
			c.Name = value.Name
		}
	case GoChannel:
		c.Name = value.Name
	case Object:
		if ast.IsDecoratorBuiltinObjectTypeName(value.Name) {
			c.Name = value.Name
			break
		}
		c.Fields = map[string]string{}
		for name, field := range value.Fields {
			c.Fields[name] = decoratorValueContract(field)
		}
	case GoStruct:
		c.Fields = map[string]string{}
		for _, field := range value.GoFields {
			c.Fields[field.Name] = decoratorValueContract(field.Type)
		}
	case GoInterface:
		c.Fields = map[string]string{}
		for _, method := range value.GoMethods {
			c.Fields[method.Name] = decoratorValueContract(method.Type)
		}
	}
	for _, child := range []*Type{value.Element, value.Key, value.Result} {
		if child == nil {
			c.Parts = append(c.Parts, "")
		} else {
			c.Parts = append(c.Parts, decoratorValueContract(*child))
		}
	}
	for _, children := range [][]Type{value.Parameters, value.Results, value.TypeArguments} {
		parts := make([]string, len(children))
		for index, child := range children {
			parts[index] = decoratorValueContract(child)
		}
		encoded, _ := json.Marshal(parts)
		c.Parts = append(c.Parts, string(encoded))
	}
	encoded, _ := json.Marshal(c)
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func decoratorValueRuntimeIdentity(value Type) string {
	switch value.Kind {
	case Class:
		return "type|" + value.String()
	case Interface:
		return "interface|" + value.String()
	case Struct:
		return "struct|" + value.String()
	default:
		return value.String()
	}
}
