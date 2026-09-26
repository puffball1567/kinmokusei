package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportAliasCaptureEditor(t *testing.T) {
	t.Parallel()
	for _, names := range [][2]string{{"pair", "pair"}, {"copy", "copy_"}, {"func", "func_"}} {
		t.Run(names[0], func(t *testing.T) { checkImportAliasCaptureEditor(t, names[0], names[1]) })
	}
}

func checkImportAliasCaptureEditor(t *testing.T, original, local string) {
	t.Helper()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(filepath.Join(root, "lib.km"), []byte(fmt.Sprintf(`export function %s(value:int):int{return value+1;}export class Box<T>{constructor(public value:T){}}`, original)), 0o644); err != nil {
		t.Fatal(err)
	}
	input := fmt.Sprintf("import {%s as load,Box as Crate} from \"./lib\";\nfunction f(%s:string):int{\nconst answer=load(42);\n\nreturn answer;\n}\nfunction typed<Box>(value:Box):void{const item=new Crate<Box>(value);_=item.value;}\n", original, local)
	items := completionLabels(completionItemsAt(t, entry, input, 3, 0))
	for _, name := range []string{local, "load", "answer"} {
		if items[name] == nil {
			t.Fatalf("missing source name %s: %v", name, items)
		}
	}
	if !strings.Contains(items[local]["detail"].(string), "string") || !strings.Contains(items["answer"]["detail"].(string), "int") {
		t.Fatalf("wrong lexical types: %v", items)
	}
	for name := range items {
		if strings.Contains(name, "_kinmokusei_") {
			t.Fatalf("internal name leaked: %s", name)
		}
	}
	at := positionOf(input, "load(42)", 0)
	at.Character += len("load(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, entry, input, at))
	if !strings.Contains(label, "value: int") {
		t.Fatalf("wrong import signature: %s", label)
	}
	uri := fileURI(entry)
	memberAt := positionOf(input, "item.value", 0)
	memberAt.Character += len("item.")
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "load(42)", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, local+":string", 0), `"newName":"local"`),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "load(42)", 0), `"newName":"read"`),
		requestAt("textDocument/completion", 5, uri, memberAt, ""))
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != uri {
		t.Fatalf("definition=%v", messages[2])
	}
	for id, count := range map[float64]int{3: 1, 4: 2} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("rename: %v", messages[id])
		}
		changes := result["changes"].(map[string]any)
		if len(changes) != 1 || len(changes[uri].([]any)) != count {
			t.Fatalf("rename crossed binding identities: %v", changes)
		}
	}
	found := false
	for _, raw := range messages[5]["result"].([]any) {
		item := raw.(map[string]any)
		if item["label"] == "value" && strings.Contains(item["detail"].(string), "Box") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing generic member completion: %v", messages[5])
	}
}
