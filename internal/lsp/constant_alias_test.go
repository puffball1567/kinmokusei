package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNumericConstantAliasEditor(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "constants.km"))
	input := `const original=2.0;const offset=original;function use(xs:int[]):int{return xs[offset];}`
	at := positionOf(input, "offset];", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"index"`))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "const offset") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "offset=")) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
}
