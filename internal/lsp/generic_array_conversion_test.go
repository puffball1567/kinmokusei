package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericArrayConversionsEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "arrays.km")
	uri := fileURI(path)
	input := `constraint Slice<E>=~E[];
function pair<E,S extends Slice<E>>(values:S):[2]E{return copyArray[[2]E](values);}
class Item{public value:int=1;}
function use(values:Item[]):int{const result=pair(values);return result[0].value;}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "result[0]", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "result[0]", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "result[0]", 0), `"newName":"copied"`))
	for _, message := range messages {
		if message["method"] == "textDocument/publishDiagnostics" && len(message["params"].(map[string]any)["diagnostics"].([]any)) != 0 {
			t.Fatalf("diagnostics=%v", message)
		}
	}
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "result: [2]Item") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(3) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "values);return", 0)))
	if label != "pair(values: Item[]): [2]Item" {
		t.Fatalf("signature=%q", label)
	}
}

func TestGenericArrayViewEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "views.km")
	uri := fileURI(path)
	input := `constraint Slice<E>=~E[];
function pair<E,S extends Slice<E>>(values:S):*[2]E{return viewArray[[2]E](values);}
class Item{public value:int=1;}
function use(values:Item[]):int{const result=pair(values);return result[0].value;}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "result[0]", 0), ""))
	for _, message := range messages {
		if message["method"] == "textDocument/publishDiagnostics" && len(message["params"].(map[string]any)["diagnostics"].([]any)) != 0 {
			t.Fatalf("diagnostics=%v", message)
		}
	}
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "result: *[2]Item") {
		t.Fatalf("hover=%v", messages[2])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "values);return", 0)))
	if label != "pair(values: Item[]): *[2]Item" {
		t.Fatalf("signature=%q", label)
	}
}
