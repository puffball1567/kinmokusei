package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestZeroSliceDiagnosticAndRecovery(t *testing.T) {
	uri := fileURI(filepath.Join(t.TempDir(), "slices.km"))
	input := "class User{}\nfunction main():void{\n const xs=makeSlice<User>(1);\n}"
	valid := "class User{}\nalias Maybe=User|null;\nfunction main():void{const xs=makeSlice<Maybe>(1);}"
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, valid)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input), change, `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
		t.Fatal(err)
	}
	var publications [][]any
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
	}
	if len(publications) != 2 || len(publications[0]) != 1 || len(publications[1]) != 0 {
		t.Fatalf("diagnostics=%v", publications)
	}
	diagnostic := publications[0][0].(map[string]any)
	start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
	if !strings.Contains(diagnostic["message"].(string), "zero value") || start["line"] != float64(2) {
		t.Fatalf("lost original source location: %v", diagnostic)
	}
}
