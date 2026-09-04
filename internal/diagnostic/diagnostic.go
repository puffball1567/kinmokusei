package diagnostic

import (
	"fmt"
	"io"

	"github.com/puffball1567/kinmokusei/internal/source"
)

type Diagnostic struct {
	Message string
	Span    source.Span
}

func (d Diagnostic) Error() string {
	if d.Span.Path == "" {
		return d.Message
	}
	return fmt.Sprintf("%s:%d:%d: %s", d.Span.Path, d.Span.Start.Line, d.Span.Start.Column, d.Message)
}

func Write(w io.Writer, diagnostics []Diagnostic) {
	for _, d := range diagnostics {
		fmt.Fprintln(w, d.Error())
	}
}
