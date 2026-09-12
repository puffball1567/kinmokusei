package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestArrowBindingNavigationAndInferredParameters(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "arrows.km")
	uri := fileURI(path)
	input := "export const twice:(n:int)=>int=(value)=>{return value*2}\nconst main=()=>{twice(21)}"
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "value*2", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "value*2", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "value*2", 0), `"newName":"amount"`),
		requestAt("textDocument/definition", 5, uri, positionOf(input, "twice(21)", 0), ""),
	)
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "value: int") {
		t.Fatalf("hover %v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "value")) {
		t.Fatalf("definition %v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename %v", messages[4])
	}
	if messages[5]["result"] == nil {
		t.Fatal("missing arrow binding definition")
	}
	at := positionOf(input, "twice(21)", 0)
	at.Character += len("twice(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, at))
	if !strings.Contains(label, "int") {
		t.Fatalf("signature %s", label)
	}
}

func TestInferredBlockArrowHover(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "inferred.km"))
	input := "const answer=()=>{return 42}\nconst main=()=>{answer()}"
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/hover", 2, uri, positionOf(input, "answer()", 0), ""))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "int") {
		t.Fatalf("hover %v", messages[2])
	}
}

func TestLocalRecursiveArrowNavigation(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "recursive.km"))
	input := `const f=():int=>99; function run():int{const f=(n:int):int=>{if(n<=1){return 1;}return n*f(n-1);};return f(5);}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "f(n-1)", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "f(n-1)", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "f(n-1)", 0), `"newName":"factorial"`),
		requestAt("textDocument/completion", 5, uri, positionOf(input, "f(n-1)", 0), ""),
	)
	if messages[2]["result"] == nil {
		t.Fatal("missing recursive binding hover")
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "f=(n")) {
		t.Fatalf("definition %v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename %v", messages[4])
	}
	found := false
	for _, raw := range messages[5]["result"].([]any) {
		item := raw.(map[string]any)
		if item["label"] == "f" {
			found = true
			if !strings.Contains(item["detail"].(string), "(int) => int") {
				t.Fatalf("completion resolves the outer binding: %v", item)
			}
		}
	}
	if !found {
		t.Fatal("missing recursive binding completion")
	}
}

func TestArrowCompletionUsesInnermostScope(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`const outer=(value:int):void=>{const inner=(value:string)=>{return value;};};`,
		`function outer(value:int):void{const inner:(value:string)=>string=(value)=>{return value;};}`,
	} {
		uri := fileURI(filepath.Join(t.TempDir(), "completion.km"))
		at := positionOf(input, "return value", 0)
		at.Character += len("return ")
		messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/completion", 2, uri, at, ""))
		found := false
		for _, raw := range messages[2]["result"].([]any) {
			item := raw.(map[string]any)
			if item["label"] == "value" {
				found = true
				if item["detail"] != "value: string" {
					t.Fatal(item)
				}
			}
		}
		if !found {
			t.Fatal("missing arrow parameter completion")
		}
	}
}
