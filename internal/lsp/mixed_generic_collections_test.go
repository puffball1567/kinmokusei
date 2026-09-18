package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMixedGenericCollectionsEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "mixed.km")
	uri := fileURI(path)
	input := `constraint Text=~string|~byte[];
function tail<T extends Text>(value:T):T{const rest=value[1:];return rest;}
class Item{public value:int=1;}
constraint Items=~Item[]|~[2]Item;
function head<T extends Items>(values:T):Item{const item=values[0];return item;}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "rest;", 0), ""),
		requestAt("textDocument/hover", 3, uri, positionOf(input, "item;", 0), ""),
		requestAt("textDocument/definition", 4, uri, positionOf(input, "item;", 0), ""),
		requestAt("textDocument/rename", 5, uri, positionOf(input, "rest;", 0), `"newName":"remaining"`))
	for _, message := range messages {
		if message["method"] == "textDocument/publishDiagnostics" && len(message["params"].(map[string]any)["diagnostics"].([]any)) != 0 {
			t.Fatalf("diagnostics=%v", message)
		}
	}
	for index, want := range map[float64]string{2: "rest: T", 3: "item: Item"} {
		hover, ok := messages[index]["result"].(map[string]any)
		if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), want) {
			t.Fatalf("hover=%v want=%q", messages[index], want)
		}
	}
	definition, ok := messages[4]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(4) {
		t.Fatalf("definition=%v", messages[4])
	}
	rename, ok := messages[5]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[5])
	}
}
