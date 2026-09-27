package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualPropertyEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "virtual_properties.km")
	uri := fileURI(path)
	input := `abstract class Base<T>{
public abstract get value():T;
public abstract set value(v:T);
}
class Child extends Base<int>{
private raw:int=0;
public override get value():int{return this.raw;}
public override set value(v:int){this.raw=v;}
}
class Leaf extends Child{
public final override get value():int{return super.value+1;}
}
function use(b:Base<int>,c:Child,d:Leaf):int{
c.value++;
return b.value+d.value;
}`
	read := positionOf(input, "b.value", 0)
	read.Character += 2
	write := positionOf(input, "c.value", 0)
	write.Character += 2
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, read, ""),
		requestAt("textDocument/definition", 3, uri, write, ""),
		requestAt("textDocument/hover", 4, uri, read, ""),
		requestAt("textDocument/rename", 5, uri, write, `"newName":"item"`),
		requestAt("textDocument/references", 6, uri, read, `"context":{"includeDeclaration":true}`),
	)
	for id, line := range map[float64]float64{2: 1, 3: 7} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("definition=%v", messages[id])
		}
		if got := result["range"].(map[string]any)["start"].(map[string]any)["line"]; got != line {
			t.Fatalf("definition=%v want=%v", got, line)
		}
	}
	if !strings.Contains(messages[4]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string), "abstract get value()") {
		t.Fatalf("hover=%v", messages[4])
	}
	if messages[5]["error"] != nil {
		t.Fatalf("rename=%v", messages[5])
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	want := strings.Count(input, "value")
	if len(changes[uri].([]any)) != want || len(messages[6]["result"].([]any)) != want {
		t.Fatalf("want %d family members; rename=%v references=%v", want, changes, messages[6])
	}
	completion := strings.Replace(input, "return b.value+d.value;", "return b.;", 1)
	items := completionLabels(completionItemsAt(t, path, completion, 14, len("return b.")))
	if items["value"] == nil || !strings.Contains(items["value"]["detail"].(string), "value: int") {
		t.Fatalf("completion=%v", items)
	}
}
