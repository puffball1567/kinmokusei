package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportAliasRenameBoundaries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	texts := map[string]string{
		"library.km": `export function original(n:int):int{return n+1}`,
		"bridge.km":  `import {original as local} from "./library";export {local as Public};function own():int{return local(2);}`,
		"entry.km":   `import {Public as run} from "./bridge";function result():int{return run(41);}`,
	}
	for name, input := range texts {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := filepath.Join(root, "entry.km")
	for _, test := range []struct {
		file, word   string
		count, files int
	}{
		{"bridge.km", "original", 2, 2},
		{"bridge.km", "local", 3, 1},
		{"entry.km", "Public", 2, 2},
		{"entry.km", "run", 2, 1},
	} {
		messages := serveMessages(t, openDocument(fileURI(entry), texts["entry.km"]), openDocument(fileURI(filepath.Join(root, test.file)), texts[test.file]),
			requestAt("textDocument/rename", 2, fileURI(filepath.Join(root, test.file)), positionOf(texts[test.file], test.word, 0), `"newName":"updated"`))
		result, ok := messages[2]["result"].(map[string]any)
		if !ok {
			t.Fatalf("%s: %v", test.word, messages[2])
		}
		changes := result["changes"].(map[string]any)
		count := 0
		for _, raw := range changes {
			count += len(raw.([]any))
		}
		if count != test.count || len(changes) != test.files {
			t.Fatalf("%s: %v", test.word, changes)
		}
	}
}

func TestImportAliasEditor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(filepath.Join(root, "library.km"), []byte(`export class Box<T>{constructor(public value:T){}}export function make(n:int):int{return n;}`), 0644); err != nil {
		t.Fatal(err)
	}
	input := `import {Box as Crate,make as create} from "./library";import go {Sprint as render} from "fmt";function run():string{const box=new Crate<int>(create(42));return render(box.value);}`
	uri := fileURI(entry)
	completionAt := positionOf(input, "value", 0)
	completionAt.Character += len("value")
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Crate", 1), ""),
		requestAt("textDocument/hover", 3, uri, positionOf(input, "Crate", 1), ""),
		requestAt("textDocument/completion", 4, uri, completionAt, ""),
		requestAt("textDocument/rename", 5, uri, positionOf(input, "render", 1), `"newName":"format"`),
		requestAt("textDocument/completion", 6, uri, positionOf(input, "const box", 0), ""),
		requestAt("textDocument/rename", 7, uri, positionOf(input, "Sprint", 0), `"newName":"Sprintln"`))
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != uri {
		t.Fatalf("definition=%v", messages[2])
	}
	hover, ok := messages[3]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Crate") {
		t.Fatalf("hover=%v", messages[3])
	}
	found := false
	for _, raw := range messages[4]["result"].([]any) {
		if raw.(map[string]any)["label"] == "value" {
			found = true
		}
	}
	if !found {
		t.Fatalf("completion=%v", messages[4])
	}
	rename, ok := messages[5]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[5])
	}
	labels := map[string]bool{}
	for _, raw := range messages[6]["result"].([]any) {
		labels[raw.(map[string]any)["label"].(string)] = true
	}
	for _, name := range []string{"Crate", "create", "render"} {
		if !labels[name] {
			t.Errorf("alias %s missing: %v", name, messages[6])
		}
	}
	for _, name := range []string{"Box", "Sprint"} {
		if labels[name] {
			t.Errorf("selected export %s leaked: %v", name, messages[6])
		}
	}
	if messages[7]["error"] == nil {
		t.Fatalf("Go export renamed: %v", messages[7])
	}
	for _, word := range []string{"create(42)", "render(box.value)"} {
		at := positionOf(input, word, 0)
		at.Character += strings.Index(word, "(") + 1
		label, _, _ := signatureResult(t, signatureHelpAt(t, entry, input, at))
		if label == "" {
			t.Fatalf("missing signature for %s", word)
		}
	}
}

func TestImportAliasSameNameAndCollision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(filepath.Join(root, "library.km"), []byte(`export function value():int{return 1;}`), 0644); err != nil {
		t.Fatal(err)
	}
	input := `import {value as value} from "./library";function run():int{const taken=2;return value()+taken;}`
	uri := fileURI(entry)
	for _, test := range []struct {
		name  string
		valid bool
	}{{"updated", true}, {"taken", false}} {
		messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/rename", 2, uri, positionOf(input, "value", 1), `"newName":"`+test.name+`"`))
		result, ok := messages[2]["result"].(map[string]any)
		if test.valid {
			if !ok || len(result["changes"].(map[string]any)[uri].([]any)) != 2 {
				t.Fatal(messages[2])
			}
		} else if messages[2]["error"] == nil {
			t.Fatal(messages[2])
		}
	}
}

func TestImportAliasBuiltinSignature(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "signature.km")
	for _, ending := range []string{"len(42);}", "len("} {
		input := `import go {Sprint as len} from "fmt";function run():string{return ` + ending
		at := positionOf(input, "len(", 0)
		at.Character += len("len(")
		label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, at))
		if !strings.HasPrefix(label, "len(") || !strings.HasSuffix(label, "string") {
			t.Fatalf("imported function lost to built-in: %q", label)
		}
	}
}
