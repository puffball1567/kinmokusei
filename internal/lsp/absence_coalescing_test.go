package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbsenceCoalescingDiagnosticsAndNavigation(t *testing.T) {
	t.Parallel()
	for name, expression := range map[string]string{"map": "m[0]", "receive": "(<-ch)"} {
		t.Run(name, func(t *testing.T) {
			prefix := `class C{public n:int=1;}struct Row{public value:C;}function f(m:Map<int,Row>,ch:GoChannel<Row>):int{const row=`
			bad := prefix + expression + `;return row.value.n;}`
			good := prefix + expression + `??Row{value:new C()};return row.value.n;}`
			uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
			change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, good)
			var output bytes.Buffer
			if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, bad), change, requestAt("textDocument/hover", 2, uri, positionOf(good, "row.value.n", 0), ""), requestAt("textDocument/definition", 3, uri, positionOf(good, "value.n", 0), ""), `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
				t.Fatal(err)
			}
			var publications [][]any
			for _, message := range decodeMessages(t, output.String()) {
				if message["method"] == "textDocument/publishDiagnostics" {
					publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
				}
				if message["id"] == float64(2) {
					hover, ok := message["result"].(map[string]any)
					if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Row") {
						t.Fatalf("hover: %v", message)
					}
				}
				if message["id"] == float64(3) {
					definition, ok := message["result"].(map[string]any)
					if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(good, "value:C")) {
						t.Fatalf("definition: %v", message)
					}
				}
			}
			if len(publications) != 2 || len(publications[0]) == 0 || len(publications[1]) != 0 {
				t.Fatalf("diagnostics=%v", publications)
			}
		})
	}
}
