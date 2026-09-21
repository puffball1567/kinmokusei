package lsp

import (
	"path/filepath"
	"testing"
)

func TestMultipleResultTypeReferences(t *testing.T) {
	uri := fileURI(filepath.Join(t.TempDir(), "results.km"))
	input := `class Box {} function pair():(Box,Box){return pair();}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/references", 2, uri, positionOf(input, "Box", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "Box", 1), `"newName":"Item"`),
	)
	if refs, ok := messages[2]["result"].([]any); !ok || len(refs) != 3 {
		t.Fatalf("references = %#v", messages[2])
	}
	result, ok := messages[3]["result"].(map[string]any)
	if !ok {
		t.Fatalf("rename = %#v", messages[3])
	}
	changes := result["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 3 {
		t.Fatalf("rename edits = %d", got)
	}
}
