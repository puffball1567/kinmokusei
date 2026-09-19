package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericArrayTargetsEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "targets.km")
	uri := fileURI(path)
	input := `constraint Array<E>=~[2]E|~[3]E;
function convert<E,A extends Array<E>>(values:E[]):A{return copyArray[A](values);}
class Item{public value:int=1;}
function use(values:Item[]):int{const result=convert<Item,[2]Item>(values);return result[0].value;}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "result[0]", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "result[0]", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "result[0]", 0), `"newName":"copied"`))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "result: [2]Item") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(3) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "values);return", 0)))
	if label != "convert(values: Item[]): [2]Item" {
		t.Fatalf("signature=%q", label)
	}
	for _, invalid := range []bool{false, true} {
		source := input
		if invalid {
			source = strings.Replace(source, "~[3]E", "~E[]", 1)
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
				diagnostic := raw.(map[string]any)["message"].(string)
				if !invalid {
					t.Fatalf("unexpected diagnostic: %s", diagnostic)
				}
				found = found || strings.Contains(diagnostic, "target must be a fixed array type")
			}
		}
		if !published || found != invalid {
			t.Fatalf("invalid=%v output=%s", invalid, output.String())
		}
	}
}
