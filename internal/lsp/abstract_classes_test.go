package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAbstractClassEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "abstract.km")
	uri := fileURI(path)
	input := `abstract class Base<T>{public abstract function read(value:T):T;}
class Leaf extends Base<int>{public override function read(value:int):int{return value;}}
function use(base:Base<int>):int{return base.read(2);}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "Base", 0), ""),
		requestAt("textDocument/hover", 3, uri, positionOf(input, "read", 0), ""),
		requestAt("textDocument/definition", 4, uri, positionOf(input, "read", 2), ""),
		requestAt("textDocument/rename", 5, uri, positionOf(input, "read", 0), `"newName":"load"`))
	for id, want := range map[float64]string{2: "abstract class Base<T>", 3: "abstract function read(value: T): T"} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok || !strings.Contains(result["contents"].(map[string]any)["value"].(string), want) {
			t.Fatalf("hover=%v", messages[id])
		}
	}
	definition := messages[4]["result"].(map[string]any)
	if definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(0) {
		t.Fatal(definition)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 3 {
		t.Fatal(changes)
	}
	completionText := strings.Replace(input, "base.read(2)", "base.", 1)
	items := completionLabels(completionItemsAt(t, path, completionText, 2, len("function use(base:Base<int>):int{return base.")))
	if items["read"] == nil || !strings.Contains(items["read"]["detail"].(string), "abstract function read(value: int): int") {
		t.Fatal(items)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "2);", 0)))
	if label != "base.read(value: int): int" {
		t.Fatal(label)
	}
}

func TestAbstractMethodValueShadowsFunctionEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shadow.km")
	uri := fileURI(path)
	input := `abstract class Base{public abstract function read(value:int):int;}
function read():string{return "global";}
function use(base:Base):int{const read=base.read;return read(2);}`
	at := positionOf(input, "read(2)", 0)
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/definition", 2, uri, at, ""), requestAt("textDocument/rename", 3, uri, at, `"newName":"load"`))
	definition := messages[2]["result"].(map[string]any)
	if definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(2) {
		t.Fatal(definition)
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 {
		t.Fatal(changes)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "2);", 0)))
	if label != "read(arg1: int): int" {
		t.Fatal(label)
	}
}
