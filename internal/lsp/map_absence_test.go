package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapAbsenceDiagnosticsAndRecovery(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ bad, good, want string }{
		"lookup":  {`class C{public n:int=1;}function f(m:Map<int,C>):int{return m[0].n;}`, `class C{public n:int=1;}function f(m:Map<int,C>):int{return (m[0]??new C()).n;}`, "nullable"},
		"checked": {`class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];return v.n;}`, `class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];if(!ok){return 0;}return v.n;}`, "may be absent"},
	} {
		t.Run(name, func(t *testing.T) {
			uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
			change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, test.good)
			var output bytes.Buffer
			if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, test.bad), change, `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
				t.Fatal(err)
			}
			var publications [][]any
			for _, message := range decodeMessages(t, output.String()) {
				if message["method"] == "textDocument/publishDiagnostics" {
					publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
				}
			}
			if len(publications) != 2 || len(publications[0]) == 0 || len(publications[1]) != 0 {
				t.Fatalf("diagnostics=%v", publications)
			}
			found := false
			for _, raw := range publications[0] {
				diagnostic := raw.(map[string]any)
				if strings.Contains(diagnostic["message"].(string), test.want) {
					found = true
					start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
					if start["line"] != float64(0) {
						t.Fatal(diagnostic)
					}
				}
			}
			if !found {
				t.Fatalf("missing %q: %v", test.want, publications[0])
			}
		})
	}
}

func TestMapAbsenceHoverPreservesNullableType(t *testing.T) {
	t.Parallel()
	input := `class C{}function f(m:Map<int,C>):void{const value=m[0];}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/hover", 2, uri, positionOf(input, "value", 0), ""))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatal(messages[2])
	}
	text := hover["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(text, "C") || !strings.Contains(text, "null") {
		t.Fatalf("hover erased absence: %s", text)
	}
}
