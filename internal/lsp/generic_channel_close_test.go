package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericChannelCloseEditor(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "close.km"))
	input := `constraint Writable=~GoChannel<int>|~GoSendChannel<string>;
function finish<C extends Writable>(channel:C):void{closeGoChannel(channel);}
function use(channel:GoChannel<int>):void{finish(channel);}`
	at := positionOf(input, "channel);}", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"queue"`))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "channel: C") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(1) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
	for _, invalid := range []bool{false, true} {
		source := input
		if invalid {
			source = strings.ReplaceAll(source, "GoSendChannel<string>", "GoReceiveChannel<string>")
		}
		var output bytes.Buffer
		if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, source), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
			t.Fatal(err)
		}
		published, found := false, false
		for _, message := range decodeMessages(t, output.String()) {
			if message["method"] != "textDocument/publishDiagnostics" {
				continue
			}
			published = true
			for _, raw := range message["params"].(map[string]any)["diagnostics"].([]any) {
				message := raw.(map[string]any)["message"].(string)
				if !invalid || !strings.Contains(message, "send-capable") {
					t.Fatalf("unexpected diagnostic: %s", message)
				}
				found = true
			}
		}
		if !published || found != invalid {
			t.Fatalf("invalid=%v output=%s", invalid, output.String())
		}
	}
}
