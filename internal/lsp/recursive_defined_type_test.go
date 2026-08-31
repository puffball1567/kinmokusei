package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRecursiveDefinedTypeNavigationRenameAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recursive_defined_type.otm")
	uri := fileURI(path)
	text := `type Chain = distinct Chain[];
public function size(this: Chain): int { return len(this); }
function use(value: Chain): int { return value.size(); }`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "Chain", 2), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "Chain", 1), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "Chain", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "Chain", 1), `"newName":"Branch"`),
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "type Chain = distinct Chain[]") {
		t.Fatalf("recursive defined type hover = %q", hover)
	}
	definition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(0) || definition["character"] != float64(5) {
		t.Fatalf("recursive defined type definition = %#v", definition)
	}
	if got := len(messages[4]["result"].([]any)); got != 4 {
		t.Fatalf("recursive defined type references = %d, want declaration, underlying, receiver, and parameter", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 4 {
		t.Fatalf("recursive defined type rename edits = %d, want 4", got)
	}

	completionText := strings.Replace(text, "value.size();", "value.;", 1)
	line := strings.Split(completionText, "\n")[2]
	items := completionLabels(completionItemsAt(t, path, completionText, 2, strings.Index(line, ".")+1))
	if items["size"] == nil || items["size"]["detail"] != "public function size(): int" {
		t.Fatalf("recursive defined type completion = %#v", items["size"])
	}
}
