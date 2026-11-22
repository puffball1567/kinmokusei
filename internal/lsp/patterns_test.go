package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRegexAndMatchEditor(t *testing.T) {
	t.Parallel()
	input := `struct Rule { public pattern: Regex }
function apply(rule: Rule, text: string): boolean {
  return match(rule) {
    case { pattern: const rx } if (rx.test(text)) => rx.test("confirmed")
    default => false
  }
}
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "rx.test(text)", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "pattern: const", 0), ""),
		requestAt("textDocument/hover", 4, uri, positionOf(input, "test(text)", 0), ""),
		requestAt("textDocument/rename", 5, uri, positionOf(input, "rx.test(text)", 0), `"newName":"filter"`),
	)
	for id, want := range map[float64]float64{2: 3, 3: 0} {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != want {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	hover, ok := messages[4]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "test(text: string): boolean") {
		t.Fatalf("hover=%v", messages[4])
	}
	rename, ok := messages[5]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename=%v", messages[5])
	}
	guard := positionOf(input, "rx.test(text)", 0)
	items := completionLabels(completionItemsAt(t, path, input, guard.Line, guard.Character))
	if items["rx"] == nil || items["rx"]["detail"] != "const rx: Regex" {
		t.Fatalf("binding completion=%v", items)
	}
	items = completionLabels(completionItemsAt(t, path, input, guard.Line, guard.Character+3))
	if items["test"] == nil || items["replaceAll"] == nil {
		t.Fatalf("Regex member completion=%v", items)
	}
}
