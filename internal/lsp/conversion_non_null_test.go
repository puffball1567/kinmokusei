package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckedConversionDiagnosticsAndDefinition(t *testing.T) {
	t.Parallel()
	bad := `alias Callback=()=>int;function f(raw:Callback):int{const checked:Callback|null=raw;return checked();}`
	good := `alias Callback=()=>int;function f(raw:Callback):int{const checked:Callback|null=Callback(raw);return checked();}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, good)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, bad), change, requestAt("textDocument/definition", 2, uri, positionOf(good, "Callback(raw)", 0), ""), `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
		t.Fatal(err)
	}
	var publications [][]any
	definitionSeen := false
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
		if message["id"] == float64(2) {
			definition, ok := message["result"].(map[string]any)
			if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(good, "Callback=")) {
				t.Fatalf("definition: %v", message)
			}
			definitionSeen = true
		}
	}
	if !definitionSeen || len(publications) != 2 || len(publications[0]) == 0 || len(publications[1]) != 0 {
		t.Fatalf("diagnostics=%v definition=%v", publications, definitionSeen)
	}
}
