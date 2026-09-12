package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestForwardArrowInferredNavigation(t *testing.T) {
	t.Parallel()
	input := `const first=()=>later(21);const later=(value:int)=>value*2;`
	uri := fileURI(filepath.Join(t.TempDir(), "forward.km"))
	at := positionOf(input, "later(21)", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"twice"`),
	)
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "=> int") {
		t.Fatalf("hover %v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.LastIndex(input, "later")) {
		t.Fatalf("definition %v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename %v", messages[4])
	}
}
