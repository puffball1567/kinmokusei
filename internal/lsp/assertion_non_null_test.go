package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestFunctionAssertionDiagnosticsAndRecovery(t *testing.T) {
	t.Parallel()
	bad := `alias Callback=()=>int;function f(value:interface{}):int{const [callback,ok]=value as? Callback;return callback();}`
	good := `alias Callback=()=>int;function f(value:interface{}):int{const [callback,ok]=value as? Callback;if(!ok){return 0;}return callback();}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, good)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, bad), change, `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
		t.Fatal(err)
	}
	var publications [][]any
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
	}
	if len(publications) != 2 || len(publications[0]) != 1 || len(publications[1]) != 0 || !strings.Contains(publications[0][0].(map[string]any)["message"].(string), "may be absent") {
		t.Fatalf("diagnostics=%v", publications)
	}
}
