package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestForPostFlowDiagnostics(t *testing.T) {
	t.Parallel()
	for _, safe := range []bool{false, true} {
		name, post := "nullable update", "p.value++"
		if safe {
			name, post = "restoring update", "p=new Item()"
		}
		t.Run(name, func(t *testing.T) {
			input := "class Item{public value:int=1;}\nfunction f(flag:boolean):void{for(let p:Item|null=new Item();flag;" + post + "){p=null;continue;}}"
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
						t.Fatalf("safe update rejected: %v", diagnostics)
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
				if start["line"] != float64(1) || start["character"] != float64(strings.Index(line, post)) {
					t.Fatalf("wrong source range: %v", diagnostic)
				}
			}
			if !published {
				t.Fatal("missing diagnostics publication")
			}
		})
	}
}
