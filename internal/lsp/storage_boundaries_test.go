package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageBoundaryDiagnosticsAndSourceNavigation(t *testing.T) {
	t.Parallel()
	prefix := `import go context from "context";alias Callback=()=>void;function f(raw:`
	bad := prefix + `context.CancelFunc|null):Callback{return raw;}`
	good := prefix + `context.CancelFunc):Callback{return raw;}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, good)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, bad), change, requestAt("textDocument/hover", 2, uri, positionOf(good, "raw;", 0), ""), requestAt("textDocument/definition", 3, uri, positionOf(good, "raw;", 0), ""), `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
		t.Fatal(err)
	}
	var publications [][]any
	hoverSeen, definitionSeen := false, false
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
		if message["id"] == float64(2) {
			hover, ok := message["result"].(map[string]any)
			if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "CancelFunc") {
				t.Fatalf("hover: %v", message)
			}
			hoverSeen = true
		}
		if message["id"] == float64(3) {
			definition, ok := message["result"].(map[string]any)
			if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(good, "raw:")) {
				t.Fatalf("definition: %v", message)
			}
			definitionSeen = true
		}
	}
	if !hoverSeen || !definitionSeen || len(publications) != 2 || len(publications[0]) == 0 || len(publications[1]) != 0 {
		t.Fatalf("diagnostics=%v hover=%v definition=%v", publications, hoverSeen, definitionSeen)
	}
}
