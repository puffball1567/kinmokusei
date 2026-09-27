package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestConversionContractDiagnostics(t *testing.T) {
	t.Parallel()
	input := "alias Maybe = *int | null; type A = distinct () => Result<Maybe>; type B = distinct () => Result<*int>;\nfunction f(value: A): B { return B(value); }"
	checkSourceContractDiagnostic(t, input, "cannot convert A to B")
}

func TestAssignmentContractDiagnostics(t *testing.T) {
	t.Parallel()
	input := "alias Maybe = *int | null; type List<T> = distinct T[];\nfunction f(value: List<Maybe>): List<*int> { return value; }"
	checkSourceContractDiagnostic(t, input, "cannot use")
}

func checkSourceContractDiagnostic(t *testing.T, input, want string) {
	t.Helper()
	for _, reject := range []bool{true, false} {
		source := input
		if !reject {
			source = strings.ReplaceAll(source, "*int | null", "*int")
		}
		uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
		var output bytes.Buffer
		if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, source), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
			t.Fatal(err)
		}
		published := false
		for _, message := range decodeMessages(t, output.String()) {
			if message["method"] != "textDocument/publishDiagnostics" {
				continue
			}
			published = true
			diagnostics := message["params"].(map[string]any)["diagnostics"].([]any)
			if !reject {
				if len(diagnostics) != 0 {
					t.Fatalf("matching contracts rejected: %v", diagnostics)
				}
				continue
			}
			if len(diagnostics) != 1 {
				t.Fatalf("expected one contract diagnostic: %v", diagnostics)
			}
			diagnostic := diagnostics[0].(map[string]any)
			if !strings.Contains(diagnostic["message"].(string), want) {
				t.Fatalf("wrong diagnostic: %v", diagnostic)
			}
			start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
			if start["line"] != float64(1) || start["character"] != float64(strings.LastIndex(strings.Split(source, "\n")[1], "value")) {
				t.Fatalf("wrong source range: %v", diagnostic)
			}
		}
		if !published {
			t.Fatal("missing diagnostics publication")
		}
	}
}
