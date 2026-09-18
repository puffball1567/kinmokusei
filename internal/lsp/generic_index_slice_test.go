package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericIndexSliceEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "index.km")
	uri := fileURI(path)
	input := `constraint Slice<E>=~E[];
function head<E,S extends Slice<E>>(values:S):E{return values[0];}
function tail<E,S extends Slice<E>>(values:S):S{const rest=values[1:];return rest;}
class Item{public value:int=1;}
function use(values:Item[]):int{const item=head(values);return item.value;}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "rest;", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "rest;", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "rest;", 0), `"newName":"remaining"`),
		requestAt("textDocument/hover", 5, uri, positionOf(input, "item.value", 0), ""))
	for _, message := range messages {
		if message["method"] == "textDocument/publishDiagnostics" && len(message["params"].(map[string]any)["diagnostics"].([]any)) != 0 {
			t.Fatalf("diagnostics=%v", message)
		}
	}
	for index, want := range map[float64]string{2: "rest: S", 5: "item: Item"} {
		hover, ok := messages[index]["result"].(map[string]any)
		if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), want) {
			t.Fatalf("hover=%v want=%q", messages[index], want)
		}
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(2) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "values);return", 0)))
	if label != "head(values: Item[]): Item" {
		t.Fatalf("signature=%q", label)
	}
}
