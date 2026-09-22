package ast

import "github.com/puffball1567/kinmokusei/internal/source"

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
	Kind           string
	Name           string
	Owner          source.Span
	Declaration    source.Span
	ParameterIndex int
	Static         bool
	Visibility     Visibility
	ValueType      *TypeRef
}

func (d *Decorator) GetSpan() source.Span { return d.Span }
