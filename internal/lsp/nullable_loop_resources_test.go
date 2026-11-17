package lsp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEditorChecksNestedFlowAndKeepsResponding(t *testing.T) {
	if os.Getenv("KINMOKUSEI_LSP_LOOP_HELPER") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEditorChecksNestedFlowAndKeepsResponding$", "-test.count=1")
		command.Env = append(os.Environ(), "KINMOKUSEI_LSP_LOOP_HELPER=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("editor did not finish nested-flow analysis: %v\n%s", err, output)
		}
		return
	}
	uri := fileURI(filepath.Join(t.TempDir(), "loops.km"))
	input := `class Item{public n:int=1;}function f(b:boolean,q:Item):void{const run=()=>{let p:Item|null=q;try{` + strings.Repeat("while(b){p=q;", 40) + "p=null;" + strings.Repeat("}", 40) + `}finally{}return 7;};}`
	invalid := `class Item{public n:int=1;}function f(p:Item|null):int{return p.n;}`
	change := func(version int, text string) string {
		return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":%d},"contentChanges":[{"text":%q}]}}`, uri, version, text)
	}
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":7}}}`, uri)
	var output bytes.Buffer
	if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input), change(2, invalid), change(3, "const answer:int=42;"), hover, `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
		t.Fatal(err)
	}
	var diagnostics [][]any
	foundHover := false
	for _, message := range decodeMessages(t, output.String()) {
		if message["method"] == "textDocument/publishDiagnostics" {
			diagnostics = append(diagnostics, message["params"].(map[string]any)["diagnostics"].([]any))
		}
		if message["id"] == float64(2) {
			result, ok := message["result"].(map[string]any)
			if ok {
				contents := result["contents"].(map[string]any)["value"].(string)
				foundHover = strings.Contains(contents, "answer: int")
			}
		}
	}
	if len(diagnostics) != 3 || len(diagnostics[0]) != 0 || len(diagnostics[1]) != 1 || len(diagnostics[2]) != 0 {
		t.Fatalf("diagnostics after edits: %v", diagnostics)
	}
	if !strings.Contains(diagnostics[1][0].(map[string]any)["message"].(string), "must be checked against null") {
		t.Fatal(diagnostics[1])
	}
	if !foundHover {
		t.Fatal("hover did not respond after nested-flow checks and edits")
	}
}
