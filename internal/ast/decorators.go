package ast

import "github.com/puffball1567/kinmokusei/internal/source"

// Decorator retains the expression and source location independently of the
// decorated declaration, so metadata diagnostics can point to the application.
type Decorator struct {
	Expression Expression
	Span       source.Span
}

func (d *Decorator) GetSpan() source.Span { return d.Span }
