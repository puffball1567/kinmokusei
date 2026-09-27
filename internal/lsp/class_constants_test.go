package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestClassConstantEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "constants.km")
	uri := fileURI(path)
	input := `class Base<T>{
public static const limit:int=3;
private static const hidden:int=1;
}
class Child extends Base<string>{}
function f():int{return Child.limit;}`
	read := positionOf(input, "Child.limit", 0)
	read.Character += len("Child.")
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, read, ""),
		requestAt("textDocument/hover", 3, uri, read, ""),
		requestAt("textDocument/rename", 4, uri, read, `"newName":"size"`),
		requestAt("textDocument/references", 5, uri, read, `"context":{"includeDeclaration":true}`))
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(1) {
		t.Fatalf("definition=%v", messages[2])
	}
	if !strings.Contains(messages[3]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string), "static const limit: int") {
		t.Fatalf("hover=%v", messages[3])
	}
	if messages[4]["error"] != nil {
		t.Fatalf("rename=%v", messages[4])
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 || len(messages[5]["result"].([]any)) != 2 {
		t.Fatalf("rename=%v references=%v", changes, messages[5])
	}
	completion := strings.Replace(input, "Child.limit", "Child.", 1)
	items := completionLabels(completionItemsAt(t, path, completion, 5, len("function f():int{return Child.")))
	if items["limit"] == nil || items["limit"]["kind"] != float64(21) || !strings.Contains(items["limit"]["detail"].(string), "static const limit: int") || items["hidden"] != nil {
		t.Fatalf("completion=%v", items)
	}
}
