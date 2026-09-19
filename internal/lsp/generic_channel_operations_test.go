package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericChannelOperationsEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "channels.km")
	uri := fileURI(path)
	input := `class Item{public value:int=7;}
constraint Readable=~GoChannel<Item|null>|~GoReceiveChannel<Item|null>;
function read<C extends Readable>(channel:C):int{
const [value,open]=<-channel;
if(value!==null){return value.value;}
return 0;
}`
	at := positionOf(input, "value!==", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"item"`))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Item | null") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(3) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename=%v", messages[4])
	}
	items := completionItemsAt(t, path, input, at.Line, at.Character)
	foundValue, foundOpen := false, false
	for _, item := range items {
		if item["label"] == "value" {
			foundValue = item["detail"] == "const value: Item | null"
		}
		if item["label"] == "open" {
			foundOpen = item["detail"] == "const open: boolean"
		}
	}
	if !foundValue || !foundOpen {
		t.Fatalf("completion=%v", items)
	}
	for _, invalid := range []bool{false, true} {
		source := `constraint Writable=~GoChannel<int>|~GoSendChannel<int>;function put<C extends Writable>(ch:C):void{select{case ch<-1{}default{}}}`
		if invalid {
			source = strings.ReplaceAll(source, "GoSendChannel", "GoReceiveChannel")
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
