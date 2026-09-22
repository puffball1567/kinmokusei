package ast

import "github.com/puffball1567/kinmokusei/internal/source"

const DecoratorContextTypeName = "DecoratorContext"

// DecoratorContextFields is the single language-level contract shared by
// semantic checking, Go lowering and editor tooling.
func DecoratorContextFields() []ObjectTypeField {
	return []ObjectTypeField{
		{Name: "kind", Type: TypeRef{Name: "string"}},
		{Name: "identity", Type: TypeRef{Name: "string"}},
		{Name: "classIdentity", Type: TypeRef{Name: "string"}},
		{Name: "baseIdentity", Type: TypeRef{Name: "string"}},
		{Name: "className", Type: TypeRef{Name: "string"}},
		{Name: "memberName", Type: TypeRef{Name: "string"}},
		{Name: "parameterName", Type: TypeRef{Name: "string"}},
		{Name: "parameterIndex", Type: TypeRef{Name: "int"}},
		{Name: "static", Type: TypeRef{Name: "boolean"}},
		{Name: "visibility", Type: TypeRef{Name: "string"}},
		{Name: "valueType", Type: TypeRef{Name: "string"}},
		{Name: "valueIdentity", Type: TypeRef{Name: "string"}},
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
	Identity       string
	ClassIdentity  string
	BaseIdentity   string
	ClassName      string
	MemberName     string
	ParameterName  string
	ValueIdentity  string
	Owner          source.Span
	Declaration    source.Span
	ParameterIndex int
	Static         bool
	Visibility     Visibility
	ValueType      *TypeRef
}

func (d *Decorator) GetSpan() source.Span { return d.Span }
