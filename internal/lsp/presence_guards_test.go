package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresenceSwitchDiagnosticsAndDefinition(t *testing.T) {
	t.Parallel()
	prefix := `class C{public n:int=7;}const YES=true;function f(m:Map<int,C>):int{const [value,ok]=m[0];`
	bad := prefix + `switch(ok){case false{return value.n;}default{return 0;}}}`
	good := prefix + `switch(ok){case YES{return value.n;}default{return 0;}}}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, good)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, bad), change, requestAt("textDocument/definition", 2, uri, positionOf(good, "n;}", 0), ""), `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
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
			if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(good, "n:int")) {
				t.Fatalf("definition: %v", message)
			}
			definitionSeen = true
		}
	}
	if !definitionSeen || len(publications) != 2 || len(publications[0]) == 0 || len(publications[1]) != 0 {
		t.Fatalf("diagnostics=%v definition=%v", publications, definitionSeen)
	}
}
