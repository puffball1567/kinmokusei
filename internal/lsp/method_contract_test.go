package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestMethodContractDiagnostics(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "contract.km"))
	input := `class Leaf{}
interface Reader{function read():Leaf;}
class Bad implements Reader{public function read():Leaf|null{return null;}}`
	var output bytes.Buffer
	err := Serve(strings.NewReader(framed(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		openDocument(uri, input),
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)), &output)
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
		for _, diagnostic := range params["diagnostics"].([]any) {
			if strings.Contains(diagnostic.(map[string]any)["message"].(string), "incompatible signature") {
				return
			}
		}
	}
	t.Fatalf("missing method contract diagnostic: %s", output.String())
}
