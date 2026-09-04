package diagnostic

import (
	"bytes"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestDiagnosticFormattingMatrix(t *testing.T) {
	withLocation := Diagnostic{Message: "broken", Span: source.Span{Path: "main.km", Start: source.Position{Line: 3, Column: 5}}}
	withoutLocation := Diagnostic{Message: "plain"}
	if got := withLocation.Error(); got != "main.km:3:5: broken" {
		t.Fatalf("located Error() = %q", got)
	}
	if got := withoutLocation.Error(); got != "plain" {
		t.Fatalf("plain Error() = %q", got)
	}
	var output bytes.Buffer
	Write(&output, []Diagnostic{withLocation, withoutLocation})
	if got, want := output.String(), "main.km:3:5: broken\nplain\n"; got != want {
		t.Fatalf("Write() = %q, want %q", got, want)
	}
	output.Reset()
	Write(&output, nil)
	if output.Len() != 0 {
		t.Fatalf("Write(nil) = %q", output.String())
	}
}
