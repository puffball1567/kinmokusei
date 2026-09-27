package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeCollectionEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "make.km")
	uri := fileURI(path)
	input := `type Numbers=distinct int[];
class Item{public value:int=1;}
alias Maybe=Item|null;
function f():void{
const numbers=make[Numbers](1,2);
const items=make[Maybe[]](2);
const channel=make[GoChannel<int>](2);
const lookup=make[Map<string,int>]();

const length=len(numbers);
}
`
	items := completionLabels(completionItemsAt(t, path, input, 8, 0))
	for name, want := range map[string]string{"numbers": "Numbers", "items": "Item | null[]", "channel": "GoChannel<int>", "lookup": "Map<string, int>"} {
		if items[name] == nil || !strings.Contains(items[name]["detail"].(string), name+": "+want) {
			t.Errorf("completion %s=%v want=%s", name, items[name], want)
		}
	}
	if items["make"] == nil {
		t.Fatal("missing make completion")
	}
	for _, test := range []struct{ needle, label string }{
		{"1,2", "make(length: int, capacity?: int): Numbers"},
		{"2);\nconst lookup", "make(capacity?: int): GoChannel<int>"},
		{");\n\n", "make(capacity?: int): Map<string, int>"},
	} {
		label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, test.needle, 0)))
		if label != test.label {
			t.Errorf("signature=%q want=%q", label, test.label)
		}
	}
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "numbers);", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "Numbers](", 0), ""))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "numbers: Numbers") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(0) {
		t.Fatalf("definition=%v", messages[3])
	}
}
