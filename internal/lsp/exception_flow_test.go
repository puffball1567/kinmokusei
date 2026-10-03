package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestExceptionFlowDiagnostics(t *testing.T) {
	t.Parallel()
	for _, safe := range []bool{false, true} {
		body := "const n=p.value;"
		if safe {
			body = "if(p!==null){const n=p.value;}"
		}
		input := "class Item{public value:int=1;}\nfunction f():void{let p:Item|null=new Item();try{p=null;return;}finally{" + body + "}}"
		uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
		var output bytes.Buffer
		if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
			t.Fatal(err)
		}
		published := false
		for _, message := range decodeMessages(t, output.String()) {
			if message["method"] != "textDocument/publishDiagnostics" {
				continue
			}
			published = true
			diagnostics := message["params"].(map[string]any)["diagnostics"].([]any)
			if safe {
				if len(diagnostics) != 0 {
					t.Fatalf("safe handler rejected: %v", diagnostics)
				}
				continue
			}
			if len(diagnostics) != 1 {
				t.Fatalf("expected one nullable diagnostic: %v", diagnostics)
			}
			diagnostic := diagnostics[0].(map[string]any)
			if !strings.Contains(diagnostic["message"].(string), "must be checked against null") {
				t.Fatalf("wrong diagnostic: %v", diagnostic)
			}
			start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
			line := strings.Split(input, "\n")[1]
			if start["line"] != float64(1) || start["character"] != float64(strings.Index(line, "p.value")) {
				t.Fatalf("wrong source range: %v", diagnostic)
			}
		}
		if !published {
			t.Fatal("missing diagnostics publication")
		}
	}
}
