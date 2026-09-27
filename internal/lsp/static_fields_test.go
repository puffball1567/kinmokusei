package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticFieldEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "static_fields.km")
	uri := fileURI(path)
	input := `class Base<T>{
public static count:int=1;
private static hidden:int=2;
public item:int=3;
}
class Child extends Base<int>{}
function f(c:Child):int{
Child.count++;
return Base.count;
}`
	read := positionOf(input, "Base.count", 0)
	read.Character += len("Base.")
	write := positionOf(input, "Child.count", 0)
	write.Character += len("Child.")
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, read, ""),
		requestAt("textDocument/definition", 3, uri, write, ""),
		requestAt("textDocument/hover", 4, uri, read, ""),
		requestAt("textDocument/rename", 5, uri, write, `"newName":"total"`),
		requestAt("textDocument/references", 6, uri, read, `"context":{"includeDeclaration":true}`))
	for _, id := range []float64{2, 3} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok || result["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(1) {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	if !strings.Contains(messages[4]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string), "static count: int") {
		t.Fatalf("hover=%v", messages[4])
	}
	if messages[5]["error"] != nil {
		t.Fatalf("rename=%v", messages[5])
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 3 || len(messages[6]["result"].([]any)) != 3 {
		t.Fatalf("rename=%v references=%v", changes, messages[6])
	}
	for _, name := range []string{"Base", "Child"} {
		completion := strings.Replace(input, "return Base.count;", "return "+name+".;", 1)
		items := completionLabels(completionItemsAt(t, path, completion, 8, len("return "+name+".")))
		if items["count"] == nil || !strings.Contains(items["count"]["detail"].(string), "static count: int") || items["hidden"] != nil || items["item"] != nil {
			t.Fatalf("completion=%v", items)
		}
	}
	completion := strings.Replace(input, "return Base.count;", "return c.;", 1)
	items := completionLabels(completionItemsAt(t, path, completion, 8, len("return c.")))
	if items["count"] != nil || items["item"] == nil {
		t.Fatalf("instance completion=%v", items)
	}
}

func TestStaticMemberCompletionUsesModuleTypes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "static_scope.km")
	input := `alias T=int;
class Base<T>{
public static value:T=1;
public static get copied():T{return Base.value;}
}
class Child extends Base<string>{}
function f():void{Child.;}`
	items := completionLabels(completionItemsAt(t, path, input, 6, len("function f():void{Child.")))
	for _, name := range []string{"value", "copied"} {
		if items[name] == nil || strings.Contains(items[name]["detail"].(string), "string") {
			t.Fatalf("static module type was substituted: %v", items)
		}
	}
	checked := strings.Replace(input, "Child.;", "const n=Child.value;", 1)
	uri := fileURI(path)
	fieldType := positionOf(checked, "value:T", 0)
	fieldType.Character += len("value:")
	accessorType := positionOf(checked, "copied():T", 0)
	accessorType.Character += len("copied():")
	messages := serveMessages(t, openDocument(uri, checked),
		requestAt("textDocument/definition", 2, uri, fieldType, ""),
		requestAt("textDocument/definition", 3, uri, accessorType, ""))
	for _, id := range []float64{2, 3} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok || result["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(0) {
			t.Fatalf("static type must resolve to module alias: %v", messages[id])
		}
	}
}
