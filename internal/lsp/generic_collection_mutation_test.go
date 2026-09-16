package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericCollectionMutationEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "mutation.km")
	uri := fileURI(path)
	input := `constraint Lookup=~Map<string,int>|~Map<string,string>;
function remove<T extends Lookup>(value:T,key:string):void{delete(value,key);clear(value);}
function use(value:Map<string,int>):void{remove(value,"x");}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "value,key", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "value,key", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "value,key", 0), `"newName":"items"`))
	for _, message := range messages {
		if message["method"] == "textDocument/publishDiagnostics" && len(message["params"].(map[string]any)["diagnostics"].([]any)) != 0 {
			t.Fatalf("diagnostics=%v", message)
		}
	}
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "value: T") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(1) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename=%v", messages[4])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, `value,"x"`, 0)))
	if label != "remove(value: Map<string, int>, key: string): void" {
		t.Fatalf("signature=%q", label)
	}
}
