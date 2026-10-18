package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeHelperBindingDiagnostic(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "runtime.km"))
	input := "function f():int{\nconst __kinmokuseiReturn=5;\ntry{return __kinmokuseiReturn;}finally{}\n}"
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
		t.Fatal(err)
	}
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] != "textDocument/publishDiagnostics" {
			continue
		}
		params := message["params"].(map[string]any)
		if params["uri"] != uri {
			continue
		}
		for _, raw := range params["diagnostics"].([]any) {
			diagnostic := raw.(map[string]any)
			if !strings.Contains(diagnostic["message"].(string), `local name "__kinmokuseiReturn" conflicts with a compiler built-in`) {
				continue
			}
			start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
			if start["line"] != float64(1) {
				t.Fatalf("unexpected diagnostic location: %v", diagnostic)
			}
			return
		}
	}
	t.Fatal("missing reserved runtime binding diagnostic")
}
