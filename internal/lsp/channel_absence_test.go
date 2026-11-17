package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestChannelAbsenceDiagnosticsAndRecovery(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ bad, good, want string }{
		"receive": {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{return (<-ch).n;}`, `class C{public n:int=1;}function f(ch:GoChannel<C>):int{return (<-ch)!.n;}`, "nullable"},
		"select":  {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{select{case const [v,open]=<-ch{return v.n;}}}`, `class C{public n:int=1;}function f(ch:GoChannel<C>):int{select{case const [v,open]=<-ch{if(!open){return 0;}return v.n;}}}`, "may be absent"},
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
				if strings.Contains(raw.(map[string]any)["message"].(string), test.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %q: %v", test.want, publications[0])
			}
		})
	}
}

func TestSelectReceiveHoverRetainsNullability(t *testing.T) {
	t.Parallel()
	input := `class C{}function f(ch:GoChannel<C>):void{select{case const value=<-ch{const snapshot=value;}}}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/hover", 2, uri, positionOf(input, "value;", 0), ""))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatal(messages[2])
	}
	text := hover["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(text, "C") || !strings.Contains(text, "null") {
		t.Fatalf("hover erased absence: %s", text)
	}
}
