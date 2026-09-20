package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestClassPropertyEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "properties.km")
	uri := fileURI(path)
	input := `class Box<T>{
constructor(private raw:T){}
public get value():T{return this.raw;}
public set value(v:T){this.raw=v;}
private get hidden():int{return 1;}
}
class Child extends Box<int>{constructor(){super(0);}}
function use(b:Child):int{
b.value=2;
return b.value;
}`
	read, write := positionOf(input, "b.value", 1), positionOf(input, "b.value", 0)
	read.Character += 2
	write.Character += 2
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, read, ""),
		requestAt("textDocument/definition", 3, uri, write, ""),
		requestAt("textDocument/hover", 4, uri, read, ""),
		requestAt("textDocument/rename", 5, uri, read, `"newName":"item"`),
		requestAt("textDocument/references", 6, uri, write, `"context":{"includeDeclaration":true}`),
	)
	for id, line := range map[float64]float64{2: 2, 3: 3} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("definition=%v", messages[id])
		}
		if got := result["range"].(map[string]any)["start"].(map[string]any)["line"]; got != line {
			t.Fatalf("definition line=%v want=%v", got, line)
		}
	}
	if !strings.Contains(messages[4]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string), "get value()") {
		t.Fatalf("hover=%v", messages[4])
	}
	if messages[5]["error"] != nil {
		t.Fatalf("rename=%v", messages[5])
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 4 {
		t.Fatalf("rename=%v", changes)
	}
	if len(messages[6]["result"].([]any)) != 4 {
		t.Fatalf("references=%v", messages[6])
	}
	completion := strings.Replace(input, "return b.value;", "return b.;", 1)
	items := completionLabels(completionItemsAt(t, path, completion, 9, len("return b.")))
	if items["value"] == nil || items["value"]["kind"] != float64(10) || !strings.Contains(items["value"]["detail"].(string), "value: int") {
		t.Fatalf("property completion=%v", items["value"])
	}
	if items["hidden"] != nil || items["get value"] != nil || items["GetValue"] != nil {
		t.Fatalf("hidden/accessor method completion=%v", items)
	}
}
