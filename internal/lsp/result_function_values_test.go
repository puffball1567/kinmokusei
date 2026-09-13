package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResultFunctionValueNavigation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "result.km")
	uri := fileURI(path)
	input := `function run():Result<int>{const load:(n:int)=>Result<int>=(value)=>{return ok(value);};return load(42);}`
	at := positionOf(input, "load(42)", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"fetch"`),
		requestAt("textDocument/completion", 5, uri, at, ""),
		requestAt("textDocument/hover", 6, uri, positionOf(input, "value);", 0), ""),
	)
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Result<int>") {
		t.Fatalf("hover %v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "load:")) {
		t.Fatalf("definition %v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename %v", messages[4])
	}
	found := false
	for _, raw := range messages[5]["result"].([]any) {
		item := raw.(map[string]any)
		if item["label"] == "load" {
			found = strings.Contains(item["detail"].(string), "Result<int>")
		}
	}
	if !found {
		t.Fatalf("completion %v", messages[5])
	}
	parameter, ok := messages[6]["result"].(map[string]any)
	if !ok || !strings.Contains(parameter["contents"].(map[string]any)["value"].(string), "value: int") {
		t.Fatalf("contextual parameter %v", messages[6])
	}
	at.Character += len("load(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, at))
	if !strings.Contains(label, "Result<int>") {
		t.Fatalf("signature %s", label)
	}
}

func TestNamedResultFunctionValueSignature(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "named.km")
	input := `type Load<T>=distinct (n:T)=>Result<T>;function run():Result<int>{const load:Load<int>=(value)=>{return ok(value);};return load(42);}`
	at := positionOf(input, "load(42)", 0)
	at.Character += len("load(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, at))
	if !strings.Contains(label, "Result<int>") {
		t.Fatalf("signature %s", label)
	}
}
