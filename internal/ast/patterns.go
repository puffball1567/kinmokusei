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
	SubjectKind    MatchSubjectKind
	EnumExhaustive bool // all declared enum values are covered without a fallback
	Span           source.Span
}

func (*MatchExpr) expression()            {}
func (e *MatchExpr) GetSpan() source.Span { return e.Span }

type MatchSubjectKind int

const (
	MatchValueSubject MatchSubjectKind = iota
	MatchResultSubject
	MatchVoidResultSubject
)

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
	MatchTypeBinding
	MatchLiteral
	MatchRecord
	MatchSequence
	MatchSequenceRest
	MatchResultOK
	MatchResultFail
)

type MatchPattern struct {
	Kind              MatchPatternKind
	Binding           Binding
	Assertion         *GoTypeAssertionExpr
	Literal           Expression
	Fields            []MatchField
	Elements          []MatchPattern
	Payload           *MatchPattern
	FixedLength       bool // the checker proved the sequence length from its type
	CoversSliceLength bool // the sequence matches every slice of the admitted lengths
	Span              source.Span
}

type MatchField struct {
	Name                string
	NameSpan            source.Span
	Pattern             MatchPattern
	ResolvedName        string
	ResolvedDeclaration source.Span
}

func (p *MatchPattern) SequenceRestIndex() int {
	for i := range p.Elements {
		if p.Elements[i].Kind == MatchSequenceRest {
			return i
		}
	}
	return -1
}

func (p *MatchPattern) Walk(visit func(*MatchPattern)) {
	visit(p)
	if p.Payload != nil {
		p.Payload.Walk(visit)
	}
	for i := range p.Fields {
		p.Fields[i].Pattern.Walk(visit)
	}
	for i := range p.Elements {
		p.Elements[i].Walk(visit)
	}
}
