package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnusedResultErrorDiagnostic(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "result.km"))
	input := `function load():Result<int>{return ok(1);}function use():int{const [value,err]=load();return value;}`
	var output bytes.Buffer
	err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output)
	if err != nil {
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
			if strings.Contains(diagnostic["message"].(string), `Result error binding "err" is never used`) {
				at := diagnostic["range"].(map[string]any)["start"].(map[string]any)["character"]
				if at != float64(strings.Index(input, "err]")) {
					t.Fatalf("diagnostic position: %v", diagnostic)
				}
				return
			}
		}
	}
	t.Fatalf("missing Result diagnostic: %s", output.String())
}

func TestDiscardHasNoLocalSymbol(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "discard.km"))
	input := `function load():Result<int>{return ok(1);}function use():void{const _=load();_=load();}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/completion", 2, uri, positionOf(input, "_=load();}", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "_=load();_=", 0), `"newName":"discarded"`),
		requestAt("textDocument/definition", 4, uri, positionOf(input, "load();_=", 0), ""))
	for _, raw := range messages[2]["result"].([]any) {
		if raw.(map[string]any)["label"] == "_" {
			t.Fatalf("blank completion: %v", raw)
		}
	}
	if messages[3]["result"] != nil {
		t.Fatalf("blank rename: %v", messages[3])
	}
	if messages[4]["result"] == nil {
		t.Fatalf("missing discarded callee definition: %v", messages[4])
	}
}
