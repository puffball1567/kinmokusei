package lsp

import (
	"path/filepath"
	"testing"
)

func TestTypeParameterConversionNavigationAndRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conversion.km")
	uri := fileURI(path)
	input := `constraint Number = ~int | ~int8;
class Counter<T extends Number> {
  public function convert(value: int): T { return T(value); }
}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "T(value)", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "T(value)", 0), `"newName":"NumberType"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition = %#v", messages[2])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(1) || start["character"] != float64(14) {
		t.Fatalf("definition = %#v", definition)
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 3 {
		t.Fatalf("rename edits = %d, want 3", got)
	}
}
