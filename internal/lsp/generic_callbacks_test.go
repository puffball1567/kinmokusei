package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericCallbackInferredParameterNavigation(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function apply<T>(v:T,f:(x:T)=>T):T{return f(v);} const run=()=>apply(2,(value)=>value*2);`,
		`import go { IndexFunc } from "slices"; const run=()=>IndexFunc([1,2],(value)=>value==2);`,
	} {
		path := filepath.Join(t.TempDir(), "callbacks.km")
		uri := fileURI(path)
		at := positionOf(input, "=>value", 0)
		at.Character += 2
		messages := serveMessages(t, openDocument(uri, input),
			requestAt("textDocument/hover", 2, uri, at, ""),
			requestAt("textDocument/definition", 3, uri, at, ""),
			requestAt("textDocument/rename", 4, uri, at, `"newName":"item"`),
			requestAt("textDocument/completion", 5, uri, at, ""),
		)
		hover, ok := messages[2]["result"].(map[string]any)
		if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "value: int") {
			t.Fatalf("hover %v", messages[2])
		}
		definition, ok := messages[3]["result"].(map[string]any)
		if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "value")) {
			t.Fatalf("definition %v", messages[3])
		}
		rename, ok := messages[4]["result"].(map[string]any)
		if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
			t.Fatalf("rename %v", messages[4])
		}
		found := false
		for _, raw := range messages[5]["result"].([]any) {
			item := raw.(map[string]any)
			if item["label"] == "value" && item["detail"] == "value: int" {
				found = true
			}
		}
		if !found {
			t.Fatal("missing inferred parameter completion")
		}
	}
}
