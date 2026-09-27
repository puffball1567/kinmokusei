package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectOperandFlowDiagnostics(t *testing.T) {
	t.Parallel()
	for _, valid := range []bool{false, true} {
		body := "return 0;"
		if !valid {
			body = "return await task;"
		}
		input := "function work():int{return 1;}\nfunction f(output:GoChannel<int>):int{const task=go work();select{case output<-await task{return 1;}default{" + body + "}}}"
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
			if valid {
				if len(diagnostics) != 0 {
					t.Fatalf("safe select rejected: %v", diagnostics)
				}
				continue
			}
			if len(diagnostics) != 1 {
				t.Fatalf("expected one Task diagnostic: %v", diagnostics)
			}
			diagnostic := diagnostics[0].(map[string]any)
			if !strings.Contains(diagnostic["message"].(string), "already") {
				t.Fatalf("wrong diagnostic: %v", diagnostic)
			}
			start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
			line := strings.Split(input, "\n")[1]
			if start["line"] != float64(1) || start["character"] != float64(strings.LastIndex(line, "task")) {
				t.Fatalf("wrong source range: %v", diagnostic)
			}
		}
		if !published {
			t.Fatal("missing diagnostics publication")
		}
	}
}
