package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitStorageDiagnosticsAndNavigation(t *testing.T) {
	t.Parallel()
	prefix := `class Box{public n:int=7;}function f(m:Map<int,Box>):int{const [value,present]=m[0];`
	bad := prefix + `return value.n;}`
	good := prefix + `if(!present){return 0;}return value.n;}`
	checkSplitStorageNavigation(t, bad, good)
}

func checkSplitStorageNavigation(t *testing.T, bad, good string) {
	t.Helper()
	uri := fileURI(filepath.Join(t.TempDir(), "split.km"))
	change := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`, uri, good)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		openDocument(uri, bad), change,
		requestAt("textDocument/hover", 2, uri, positionOf(good, "value.n", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(good, "value.n", 0), ""),
		`{"jsonrpc":"2.0","id":4,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)), &output); err != nil {
		t.Fatal(err)
	}
	var publications [][]any
	hoverSeen, definitionSeen := false, false
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			publications = append(publications, message["params"].(map[string]any)["diagnostics"].([]any))
		}
		if message["id"] == float64(2) {
			hover, ok := message["result"].(map[string]any)
			if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Box") {
				t.Fatalf("hover: %v", message)
			}
			hoverSeen = true
		}
		if message["id"] == float64(3) {
			definition, ok := message["result"].(map[string]any)
			if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(good, "value")) {
				t.Fatalf("definition: %v", message)
			}
			definitionSeen = true
		}
	}
	if !hoverSeen || !definitionSeen || len(publications) != 2 || len(publications[0]) == 0 || len(publications[1]) != 0 {
		t.Fatalf("diagnostics=%v hover=%v definition=%v", publications, hoverSeen, definitionSeen)
	}
}

func TestInferredTupleDiagnosticsAndNavigation(t *testing.T) {
	t.Parallel()
	prefix := `class Box{public n:int=7;}function pair():(Box,int){return new Box(),0;}function f():int{const [`
	bad := prefix + `value,n,extra]=pair();return value.n;}`
	good := prefix + `value,n]=pair();return value.n;}`
	checkSplitStorageNavigation(t, bad, good)
}

func TestResultBoundaryDiagnosticsAndNavigation(t *testing.T) {
	t.Parallel()
	prefix := `class Box{public n:int=7;}function source():Result<Box>{return ok(new Box());}function f():Result<int>{const value=`
	bad := prefix + `source();return ok(value.n);}`
	good := prefix + `source()?;return ok(value.n);}`
	checkSplitStorageNavigation(t, bad, good)
}

func TestResultPresenceDiagnosticsAndNavigation(t *testing.T) {
	t.Parallel()
	prefix := `class Box{public n:int=7;}function source():Result<Box>{return ok(new Box());}function f():int{const [value,err]=source();`
	bad := prefix + `_ = err;return value.n;}`
	good := prefix + `if(err!=nil){return 0;}return value.n;}`
	checkSplitStorageNavigation(t, bad, good)
}
