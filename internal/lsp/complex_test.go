package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestComplexCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "complex.km")
	input := "function use(a:float32,b:float32):complex64{return complex(a,b);}\n"
	items := completionLabels(completionItemsAt(t, path, input, 1, 0))
	for _, name := range []string{"complex64", "complex128", "complex", "real", "imag"} {
		if items[name] == nil {
			t.Fatalf("missing completion %s", name)
		}
	}
	position := positionOf(input, "a,b);", 0)
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, position))
	if !strings.Contains(label, "float32") || !strings.HasSuffix(label, ": complex64") {
		t.Fatalf("signature=%q", label)
	}
}
