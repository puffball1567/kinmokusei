package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalArrowGroupNavigation(t *testing.T) {
	t.Parallel()
	input := `const later=():string=>"outer";function run():int{const first=():int=>later();const later=():int=>42;return first();}`
	uri := fileURI(filepath.Join(t.TempDir(), "groups.km"))
	at := positionOf(input, "later()", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"answer"`),
		requestAt("textDocument/completion", 5, uri, at, ""),
	)
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "=> int") {
		t.Fatalf("hover %v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.LastIndex(input, "later=")) {
		t.Fatalf("definition %v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename %v", messages[4])
	}
	found := false
	for _, raw := range messages[5]["result"].([]any) {
		item := raw.(map[string]any)
		if item["label"] == "later" {
			found = true
			if !strings.Contains(item["detail"].(string), "=> int") {
				t.Fatal(item)
			}
		}
	}
	if !found {
		t.Fatal("missing forward peer completion")
	}
}

func TestLocalArrowGroupCompletionScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, at, want string }{
		{`const later=():string=>"outer";function run():int{const first=(later:int):int=>later;const later=():int=>42;return first(1);}`, "=>later;", "later: int"},
		{`const later=():string=>"outer";function run():int{const first=():int=>{const later=1;return later;};const later=():int=>42;return first();}`, "return later;", "const later: int"},
		{`function run():int{const first=():int=>missing();const value=1;const later=():int=>42;return first();}`, "missing()", ""},
	} {
		uri := fileURI(filepath.Join(t.TempDir(), "scope.km"))
		at := positionOf(test.input, test.at, 0)
		if strings.HasPrefix(test.at, "=>") {
			at.Character += 2
		}
		messages := serveMessages(t, openDocument(uri, test.input), requestAt("textDocument/completion", 2, uri, at, ""))
		found := ""
		for _, raw := range messages[2]["result"].([]any) {
			item := raw.(map[string]any)
			if item["label"] == "later" {
				found = item["detail"].(string)
			}
		}
		if found != test.want {
			t.Fatalf("%s: got %q want %q", test.input, found, test.want)
		}
	}
}
