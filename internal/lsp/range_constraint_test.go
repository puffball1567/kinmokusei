package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConstrainedRangeNavigationCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ranges.km")
	uri := fileURI(path)
	input := `class Leaf<V> { constructor(public value: V) {} public function read(): V { return this.value; } }
constraint Leaves = ~Leaf<int>[];
function use<T extends Leaves>(values: T): void {
  for (const item of values) { const result = item.read(); }
}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "item.read", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "item.read", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "item.read", 0), `"newName":"element"`),
	)
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "const item: Leaf<int>") {
		t.Fatalf("hover = %#v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition = %#v", messages[3])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(3) || start["character"] != float64(13) {
		t.Fatalf("definition = %#v", start)
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 {
		t.Fatalf("rename = %#v", changes)
	}
	completion := strings.Replace(input, "item.read()", "item.", 1)
	line := strings.Split(completion, "\n")[3]
	items := completionLabels(completionItemsAt(t, path, completion, 3, strings.Index(line, "item.")+len("item.")))
	if items["value"] == nil || items["read"] == nil || items["read"]["detail"] != "public function read(): int" {
		t.Fatalf("completion = %#v", items)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "); }", 0)))
	if label != "item.read(): int" {
		t.Fatalf("signature = %q", label)
	}
}
