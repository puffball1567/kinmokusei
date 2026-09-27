package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticPropertyEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "static_properties.km")
	uri := fileURI(path)
	input := `let raw=0;
class Base<T>{
public static get value():int{return raw;}
public static set value(v:int){raw=v;}
private static get hidden():int{return raw;}
public get item():int{return raw;}
}
class Child extends Base<int>{}
function use(c:Child):int{
Child.value=2;
return Base.value;
}`
	read := positionOf(input, "Base.value", 0)
	read.Character += len("Base.")
	write := positionOf(input, "Child.value", 0)
	write.Character += len("Child.")
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, read, ""),
		requestAt("textDocument/definition", 3, uri, write, ""),
		requestAt("textDocument/hover", 4, uri, read, ""),
		requestAt("textDocument/rename", 5, uri, write, `"newName":"count"`),
		requestAt("textDocument/references", 6, uri, read, `"context":{"includeDeclaration":true}`),
	)
	for id, line := range map[float64]float64{2: 2, 3: 3} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok || result["range"].(map[string]any)["start"].(map[string]any)["line"] != line {
			t.Fatalf("definition=%v want line=%v", messages[id], line)
		}
	}
	if !strings.Contains(messages[4]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string), "static get value()") {
		t.Fatalf("hover=%v", messages[4])
	}
	if messages[5]["error"] != nil {
		t.Fatalf("rename=%v", messages[5])
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 4 || len(messages[6]["result"].([]any)) != 4 {
		t.Fatalf("rename=%v references=%v", changes, messages[6])
	}
	for _, name := range []string{"Base", "Child"} {
		completion := strings.Replace(input, "return Base.value;", "return "+name+".;", 1)
		items := completionLabels(completionItemsAt(t, path, completion, 10, len("return "+name+".")))
		if items["value"] == nil || items["value"]["kind"] != float64(10) || !strings.Contains(items["value"]["detail"].(string), "static ") || !strings.Contains(items["value"]["detail"].(string), "value: int") || items["hidden"] != nil || items["item"] != nil {
			t.Fatalf("%s completion=%v", name, items)
		}
	}
	completion := strings.Replace(input, "return Base.value;", "return c.;", 1)
	items := completionLabels(completionItemsAt(t, path, completion, 10, len("return c.")))
	if items["value"] != nil || items["item"] == nil {
		t.Fatalf("instance completion=%v", items)
	}
}
