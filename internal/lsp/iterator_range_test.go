package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIteratorRangeBindingNavigationAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iterator.km")
	uri := fileURI(path)
	input := `class Leaf { constructor(public value: int) {} }
class Sequence<T> {
  constructor(private value: T) {}
  public function iterate(yield: (value: T) => boolean): void { yield(this.value); }
}
function use(sequence: Sequence<Leaf>): void {
  for (const item of sequence.iterate) { const n = item.value; }
}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "item.value", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "item.value", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "item.value", 0), `"newName":"element"`),
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "const item: Leaf") {
		t.Fatalf("hover = %s", hover)
	}
	start := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(6) || start["character"] != float64(13) {
		t.Fatalf("definition = %#v", start)
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 {
		t.Fatalf("rename = %#v", changes)
	}
	completion := strings.Replace(input, "item.value", "item.", 1)
	line := strings.Split(completion, "\n")[6]
	items := completionLabels(completionItemsAt(t, path, completion, 6, strings.Index(line, "item.")+len("item.")))
	if items["value"] == nil {
		t.Fatalf("completion = %#v", items)
	}
}
