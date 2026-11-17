package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionalChainingNavigation(t *testing.T) {
	t.Parallel()
	input := `class Box{public n:int=7;public function read(value:int):int{return value;}}function f(box:Box|null):int{return box?.read(1)??0;}`
	uri := fileURI(filepath.Join(t.TempDir(), "optional.km"))
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "read(1)", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "read(1)", 0), ""))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "read") {
		t.Fatalf("hover: %v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "read")) {
		t.Fatalf("definition: %v", messages[3])
	}
}

func TestOptionalChainingCompletion(t *testing.T) {
	t.Parallel()
	input := `class Box{public name:string="box";}function f(box:Box|null):void{box?.na;}`
	uri := fileURI(filepath.Join(t.TempDir(), "optional.km"))
	at := positionOf(input, ".na;", 0)
	at.Character += 3
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/completion", 2, uri, at, ""))
	for _, item := range messages[2]["result"].([]any) {
		if item.(map[string]any)["label"] == "name" {
			return
		}
	}
	t.Fatalf("completion: %v", messages[2])
}

func TestOptionalChainingSignatureHelp(t *testing.T) {
	t.Parallel()
	input := `class Box{public function read(value:int):int{return value;}}function f(box:Box|null):int{return box?.read(1)??0;}`
	uri := fileURI(filepath.Join(t.TempDir(), "optional.km"))
	at := positionOf(input, "read(1)", 0)
	at.Character += 5
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/signatureHelp", 2, uri, at, ""))
	result, ok := messages[2]["result"].(map[string]any)
	if !ok || len(result["signatures"].([]any)) == 0 {
		t.Fatalf("signature help: %v", messages[2])
	}
}
