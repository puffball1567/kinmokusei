package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorSurvivesDeepSyntaxAndRecoversAfterEdit(t *testing.T) {
	uri := fileURI(filepath.Join(t.TempDir(), "deep.km"))
	input := "const value=" + strings.Repeat("!", 10000) + "true;"
	valid := "const answer: int=42;"
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, valid)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":7}}}`, uri)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		openDocument(uri, input), change, hover,
		`{"jsonrpc":"2.0","id":3,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)), &output); err != nil {
		t.Fatal(err)
	}
	publications := [][]any{}
	foundHover := false
	var hoverText string
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
		if message["id"] == float64(2) {
			result, ok := message["result"].(map[string]any)
			if !ok {
				t.Fatalf("hover=%v", message)
			}
			hoverText = result["contents"].(map[string]any)["value"].(string)
			foundHover = strings.Contains(hoverText, "answer: int")
		}
	}
	if len(publications) != 2 || len(publications[0]) != 1 || len(publications[1]) != 0 {
		t.Fatalf("diagnostics=%v", publications)
	}
	if !strings.Contains(publications[0][0].(map[string]any)["message"].(string), "syntax nesting is too deep") {
		t.Fatalf("diagnostics=%v", publications[0])
	}
	if !foundHover {
		t.Fatalf("hover did not recover after editing the deep file: %q", hoverText)
	}
}
