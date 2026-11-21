package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStructEmbeddingEditor(t *testing.T) {
	t.Parallel()
	input := `struct Cell<T> {
  public value: T;
  public function read(): T { return this.value; }
}
struct Layer { Cell<int>; }
function use(x: Layer): int { return x.value + x.read() + x.Cell.value; }
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "value +", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "read()", 1), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "Cell.value", 0), `"newName":"Box"`),
	)
	for id, want := range map[float64]float64{2: 1, 3: 2} {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != want {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename=%v", messages[4])
	}
	line := strings.Split(input, "\n")[5]
	items := completionLabels(completionItemsAt(t, path, input, 5, strings.Index(line, "value +")))
	if items["value"] == nil || items["value"]["detail"] != "value: int" {
		t.Fatalf("field completion=%v", items)
	}
	if items["read"] == nil || !strings.Contains(items["read"]["detail"].(string), "int") {
		t.Fatalf("method completion=%v", items)
	}
}
