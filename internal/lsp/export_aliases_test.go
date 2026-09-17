package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportAliasRenameBoundaries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	texts := map[string]string{
		"library.km": `function original(n:int):int{return n+1}export {original as Public}`,
		"bridge.km":  `export {Public as Renamed} from "./library"`,
		"facade.km":  `import {Renamed} from "./bridge";export {Renamed}`,
		"entry.km":   `import {Renamed} from "./facade";function run():int{return Renamed(41)}`,
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
		{"library.km", "original", 2, 1},
		{"bridge.km", "Public", 2, 2},
		{"entry.km", "Renamed", 5, 3},
	} {
		messages := serveMessages(t, openDocument(fileURI(entry), texts["entry.km"]), openDocument(fileURI(filepath.Join(root, test.file)), texts[test.file]),
			requestAt("textDocument/rename", 2, fileURI(filepath.Join(root, test.file)), positionOf(texts[test.file], test.word, 0), `"newName":"updated"`))
		rename, ok := messages[2]["result"].(map[string]any)
		if !ok {
			t.Fatalf("%s rename: %v", test.word, messages[2])
		}
		changes := rename["changes"].(map[string]any)
		count := 0
		for _, raw := range changes {
			count += len(raw.([]any))
		}
		if len(changes) != test.files || count != test.count {
			t.Fatalf("%s changes %v", test.word, changes)
		}
	}
	messages := serveMessages(t, openDocument(fileURI(entry), texts["entry.km"]),
		requestAt("textDocument/definition", 2, fileURI(entry), positionOf(texts["entry.km"], "Renamed", 1), ""),
		requestAt("textDocument/hover", 3, fileURI(entry), positionOf(texts["entry.km"], "Renamed", 1), ""))
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != fileURI(filepath.Join(root, "bridge.km")) {
		t.Fatalf("definition %v", messages[2])
	}
	hover, ok := messages[3]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "Renamed(") {
		t.Fatalf("hover %v", messages[3])
	}
}

func TestExportAliasSameSpellingRename(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "library.km")
	entry := filepath.Join(root, "entry.km")
	lib := `function value():int{return 42}export {value as value}`
	input := `import {value} from "./library";function run():int{return value()}`
	if err := os.WriteFile(library, []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		file, text string
		count      int
	}{{library, lib, 2}, {entry, input, 3}} {
		messages := serveMessages(t, openDocument(fileURI(entry), input), openDocument(fileURI(library), lib), requestAt("textDocument/rename", 2, fileURI(test.file), positionOf(test.text, "value", 0), `"newName":"updated"`))
		result, ok := messages[2]["result"].(map[string]any)
		if !ok {
			t.Fatal(messages[2])
		}
		count := 0
		for _, raw := range result["changes"].(map[string]any) {
			count += len(raw.([]any))
		}
		if count != test.count {
			t.Fatalf("count=%d want=%d %v", count, test.count, messages[2])
		}
	}
}

func TestExportAliasClassEditor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "library.km")
	entry := filepath.Join(root, "entry.km")
	lib := `class Local{constructor(public value:int){}}export {Local as First,Local as Second}`
	input := `import {First,Second} from "./library";function run():int{const first=new First(42);const second:Second=first;return second.value;}`
	if err := os.WriteFile(library, []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	messages := serveMessages(t, openDocument(fileURI(entry), input),
		requestAt("textDocument/rename", 2, fileURI(entry), positionOf(input, "First", 0), `"newName":"Updated"`),
		requestAt("textDocument/completion", 3, fileURI(entry), positionOf(input, "value;", 0), ""),
		requestAt("textDocument/hover", 4, fileURI(entry), positionOf(input, "First", 0), ""))
	result, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatal(messages[2])
	}
	count := 0
	for _, raw := range result["changes"].(map[string]any) {
		count += len(raw.([]any))
	}
	if count != 3 {
		t.Fatal(messages[2])
	}
	found := false
	for _, raw := range messages[3]["result"].([]any) {
		if raw.(map[string]any)["label"] == "value" {
			found = true
		}
	}
	if !found {
		t.Fatal(messages[3])
	}
	hover, ok := messages[4]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "First") {
		t.Fatal(messages[4])
	}
	at := positionOf(input, "First(42)", 0)
	at.Character += len("First(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, entry, input, at))
	if !strings.Contains(label, "value: int") {
		t.Fatalf("signature %q", label)
	}
}
