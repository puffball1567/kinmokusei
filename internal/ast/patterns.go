package ast

import "github.com/puffball1567/kinmokusei/internal/source"

const RegexTypeName = "Regex"

type RegexMethod struct {
	Name      string
	Signature CallableSignature
}

func RegexMethods() []RegexMethod {
	return []RegexMethod{
		{Name: "test", Signature: CallableSignature{ParameterNames: []string{"text"}, ParameterTypes: []string{"string"}, Result: "boolean"}},
		{Name: "captures", Signature: CallableSignature{ParameterNames: []string{"text"}, ParameterTypes: []string{"string"}, Result: "string[]"}},
		{Name: "replaceAll", Signature: CallableSignature{ParameterNames: []string{"text", "replacement"}, ParameterTypes: []string{"string", "string"}, Result: "string"}},
	}
}

type RegexExpr struct {
	Pattern string
	Index   int // assigned by the checker
	Span    source.Span
}

func (*RegexExpr) expression()            {}
func (e *RegexExpr) GetSpan() source.Span { return e.Span }

type MatchExpr struct {
	Value          Expression
	Arms           []MatchArm
	ResultType     TypeRef
	EnumExhaustive bool // all declared enum values are covered without a fallback
	Span           source.Span
}

func (*MatchExpr) expression()            {}
func (e *MatchExpr) GetSpan() source.Span { return e.Span }

type MatchArm struct {
	Pattern MatchPattern
	Guard   Expression
	Value   Expression
	Span    source.Span
}

type MatchPatternKind int

const (
	MatchWildcard MatchPatternKind = iota
	MatchBinding
	MatchNonNullBinding
	MatchLiteral
	MatchRecord
	MatchSequence
)

type MatchPattern struct {
	Kind        MatchPatternKind
	Binding     Binding
	Literal     Expression
	Fields      []MatchField
	Elements    []MatchPattern
	FixedLength bool // the checker proved the sequence length from its type
	Span        source.Span
}

type MatchField struct {
	Name                string
	NameSpan            source.Span
	Pattern             MatchPattern
	ResolvedName        string
	ResolvedDeclaration source.Span
}

func (p *MatchPattern) Walk(visit func(*MatchPattern)) {
	visit(p)
	for i := range p.Fields {
		p.Fields[i].Pattern.Walk(visit)
	}
	for i := range p.Elements {
		p.Elements[i].Walk(visit)
	}
}
