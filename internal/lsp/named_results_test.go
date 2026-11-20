package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedResultNavigationAndCompletion(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"function scale(x: int): (answer: int, err: error) {\n  answer = x * 2\n  return\n}",
		"const scale = (x: int): (answer: int, err: error) => {\n  answer = x * 2\n  return\n}",
	} {
		path := filepath.Join(t.TempDir(), "named.km")
		uri := fileURI(path)
		use := positionOf(input, "answer", 1)
		messages := serveMessages(t, openDocument(uri, input),
			requestAt("textDocument/definition", 2, uri, use, ""),
			requestAt("textDocument/hover", 3, uri, use, ""),
			requestAt("textDocument/rename", 4, uri, use, `"newName":"result"`),
		)
		definition, ok := messages[2]["result"].(map[string]any)
		if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "answer")) {
			t.Fatalf("definition=%v", messages[2])
		}
		hover, ok := messages[3]["result"].(map[string]any)
		if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "answer: int") {
			t.Fatalf("hover=%v", messages[3])
		}
		rename, ok := messages[4]["result"].(map[string]any)
		if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
			t.Fatalf("rename=%v", messages[4])
		}
		items := completionLabels(completionItemsAt(t, path, input, 1, 3))
		if items["answer"] == nil || items["answer"]["detail"] != "answer: int" {
			t.Fatalf("completion=%v", items)
		}
	}
}
