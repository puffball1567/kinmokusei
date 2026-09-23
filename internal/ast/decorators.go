package ast

import "github.com/puffball1567/kinmokusei/internal/source"

const (
	DecoratorContextTypeName = "DecoratorContext"
	DecoratorValueTypeName   = "DecoratorValue"
)

type DecoratorContextDefinition struct {
	Name        string
	TargetKinds []string
}

var decoratorContextDefinitions = []DecoratorContextDefinition{
	{Name: DecoratorContextTypeName},
	{Name: "ClassDecoratorContext", TargetKinds: []string{"class"}},
	{Name: "FieldDecoratorContext", TargetKinds: []string{"field"}},
	{Name: "ConstructorDecoratorContext", TargetKinds: []string{"constructor"}},
	{Name: "MethodDecoratorContext", TargetKinds: []string{"method"}},
	{Name: "GetterDecoratorContext", TargetKinds: []string{"get"}},
	{Name: "SetterDecoratorContext", TargetKinds: []string{"set"}},
	{Name: "ParameterDecoratorContext", TargetKinds: []string{"parameter"}},
}

func DecoratorContextDefinitions() []DecoratorContextDefinition {
	definitions := make([]DecoratorContextDefinition, len(decoratorContextDefinitions))
	for index, definition := range decoratorContextDefinitions {
		definitions[index] = definition
		definitions[index].TargetKinds = append([]string(nil), definition.TargetKinds...)
	}
	return definitions
}

func IsDecoratorContextTypeName(name string) bool {
	for _, definition := range decoratorContextDefinitions {
		if definition.Name == name {
			return true
		}
	}
	return false
}

func IsDecoratorBuiltinObjectTypeName(name string) bool {
	return IsDecoratorContextTypeName(name) || name == DecoratorValueTypeName
}

// DecoratorValueFields is intentionally small. The compiler owns the hidden
// payload used by callable adapters; source packages can inspect its stable
// type identity and pass the value to another checked adapter, but cannot
// perform an unchecked cast.
func DecoratorValueFields() []ObjectTypeField {
	return []ObjectTypeField{{Name: "typeIdentity", Type: TypeRef{Name: "string"}}}
}

func DecoratorContextAcceptsTarget(name, kind string) bool {
	for _, definition := range decoratorContextDefinitions {
		if definition.Name != name {
			continue
		}
		if len(definition.TargetKinds) == 0 {
			return true
		}
		for _, allowed := range definition.TargetKinds {
			if allowed == kind {
				return true
			}
		}
		return false
	}
	return false
}

// DecoratorContextFields is the single language-level contract shared by
// semantic checking, Go lowering and editor tooling.
func DecoratorContextFields() []ObjectTypeField {
	value := TypeRef{Name: DecoratorValueTypeName}
	arguments := TypeRef{Element: &value}
	result := TypeRef{Name: "Result", GenericArguments: []TypeRef{value}}
	construct := TypeRef{Parameters: []TypeRef{arguments}, Return: &result}
	return []ObjectTypeField{
		{Name: "kind", Type: TypeRef{Name: "string"}},
		{Name: "identity", Type: TypeRef{Name: "string"}},
		{Name: "classIdentity", Type: TypeRef{Name: "string"}},
		{Name: "baseIdentity", Type: TypeRef{Name: "string"}},
		{Name: "overrideChain", Type: TypeRef{Element: &TypeRef{Name: "string"}}},
		{Name: "className", Type: TypeRef{Name: "string"}},
		{Name: "memberName", Type: TypeRef{Name: "string"}},
		{Name: "parameterName", Type: TypeRef{Name: "string"}},
		{Name: "parameterIndex", Type: TypeRef{Name: "int"}},
		{Name: "static", Type: TypeRef{Name: "boolean"}},
		{Name: "visibility", Type: TypeRef{Name: "string"}},
		{Name: "valueType", Type: TypeRef{Name: "string"}},
		{Name: "valueIdentity", Type: TypeRef{Name: "string"}},
		{Name: "constructible", Type: TypeRef{Name: "boolean"}},
		{Name: "constructUnavailableReason", Type: TypeRef{Name: "string"}},
		{Name: "construct", Type: construct},
	}
}

// Decorator retains the expression and source location independently of the
// decorated declaration, so metadata diagnostics can point to the application.
type Decorator struct {
	Expression Expression
	Span       source.Span
	// Target is checked compiler metadata, not an executable runtime context.
	Target *DecoratorTarget
}

// Declaration spans identify targets independently of module-local spelling.
// ParameterIndex is zero-based for parameters and -1 for other targets.
type DecoratorTarget struct {
	Kind string
	Name string
	// Identity is an opaque, build-stable identifier. The remaining names are
	// source spellings intended for framework diagnostics and registration.
	Identity                   string
	ClassIdentity              string
	BaseIdentity               string
	OverrideChain              []string
	ClassName                  string
	MemberName                 string
	ParameterName              string
	ValueIdentity              string
	RuntimeClassName           string
	ConstructUnavailableReason string
	ConstructorParameters      []TypeRef
	Owner                      source.Span
	Declaration                source.Span
	ParameterIndex             int
	Static                     bool
	Visibility                 Visibility
	ValueType                  *TypeRef
	Constructible              bool
}

func (d *Decorator) GetSpan() source.Span { return d.Span }
