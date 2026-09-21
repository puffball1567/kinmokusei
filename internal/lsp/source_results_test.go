package lsp

import (
	"path/filepath"
	"testing"
)

func TestAdditionalReturnExpressionTooling(t *testing.T) {
	path := filepath.Join(t.TempDir(), "return-list.km")
	uri := fileURI(path)
	input := `function helper(value:int):int{return value;} function pair():(int,int){return 1,helper(2);}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/references", 2, uri, positionOf(input, "helper", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "helper", 1), `"newName":"calculate"`),
	)
	if refs, ok := messages[2]["result"].([]any); !ok || len(refs) != 2 {
		t.Fatalf("references = %#v", messages[2])
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 {
		t.Fatalf("rename = %#v", changes)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "2);", 0)))
	if label != "helper(value: int): int" {
		t.Fatalf("signature = %q", label)
	}
}

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
