package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInterfacePropertyEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "interface_properties.km")
	uri := fileURI(path)
	input := `interface Read<T>{get value():T;}
interface Write<T>{set value(v:T);}
interface Cell<T> extends Read<T>,Write<T>{}
abstract class Base<T>{
public abstract get value():T;
public abstract set value(v:T);
}
class Child extends Base<int>{
private raw:int=0;
public override get value():int{return this.raw;}
public override set value(v:int){this.raw=v;}
}
class Leaf extends Child implements Cell<int>{}
function use(c:Cell<int>,r:Read<int>,w:Write<int>,l:Leaf):int{
w.value=2;
c.value++;
return r.value+l.value;
}`
	read := positionOf(input, "r.value", 0)
	read.Character += 2
	write := positionOf(input, "w.value", 0)
	write.Character += 2
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, read, ""),
		requestAt("textDocument/definition", 3, uri, write, ""),
		requestAt("textDocument/hover", 4, uri, read, ""),
		requestAt("textDocument/rename", 5, uri, write, `"newName":"item"`),
		requestAt("textDocument/references", 6, uri, read, `"context":{"includeDeclaration":true}`),
	)
	for id, line := range map[float64]float64{2: 0, 3: 1} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("definition=%v", messages[id])
		}
		if got := result["range"].(map[string]any)["start"].(map[string]any)["line"]; got != line {
			t.Fatalf("definition=%v want=%v", got, line)
		}
	}
	if !strings.Contains(messages[4]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string), "get value()") {
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
	for _, name := range []string{"c", "r", "w", "l"} {
		completion := strings.Replace(input, "return r.value+l.value;", "return "+name+".;", 1)
		items := completionLabels(completionItemsAt(t, path, completion, 16, len("return "+name+".")))
		if items["value"] == nil || items["value"]["kind"] != float64(10) || !strings.Contains(items["value"]["detail"].(string), "value: int") {
			t.Fatalf("%s completion=%v", name, items)
		}
	}
}
