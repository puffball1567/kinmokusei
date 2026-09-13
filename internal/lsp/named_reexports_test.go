package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedReexportNavigationAndRename(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "library.km")
	barrel := filepath.Join(root, "barrel.km")
	forward := filepath.Join(root, "forward.km")
	entry := filepath.Join(root, "entry.km")
	libraryText := `export function value(n:int):int{return n+1}`
	barrelText := `export {value} from "./forward";function value():int{return 99;}`
	forwardText := `import {value} from "./library";export {value};`
	entryText := `import {value} from "./barrel";function run():int{return value(2);}`
	for path, input := range map[string]string{library: libraryText, barrel: barrelText, forward: forwardText, entry: entryText} {
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	messages := serveMessages(t, openDocument(fileURI(entry), entryText),
		requestAt("textDocument/definition", 2, fileURI(entry), positionOf(entryText, "value", 0), ""),
		requestAt("textDocument/references", 3, fileURI(entry), positionOf(entryText, "value", 1), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 4, fileURI(entry), positionOf(entryText, "value", 1), `"newName":"increment"`),
		requestAt("textDocument/hover", 5, fileURI(entry), positionOf(entryText, "value", 0), ""),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != fileURI(library) {
		t.Fatalf("definition %v", messages[2])
	}
	if refs, ok := messages[3]["result"].([]any); !ok || len(refs) != 6 {
		t.Fatalf("references %v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok {
		t.Fatalf("rename %v", messages[4])
	}
	changes := rename["changes"].(map[string]any)
	if len(changes) != 4 || len(changes[fileURI(entry)].([]any)) != 2 || len(changes[fileURI(barrel)].([]any)) != 1 {
		t.Fatalf("changes %v", changes)
	}
	hover, ok := messages[5]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "value(") {
		t.Fatalf("hover %v", messages[5])
	}
	at := positionOf(entryText, "value(2)", 0)
	at.Character += len("value(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, entry, entryText, at))
	if !strings.Contains(label, "int") {
		t.Fatalf("signature %q", label)
	}
}

func TestNamedReexportClassCompletion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "library.km")
	barrel := filepath.Join(root, "barrel.km")
	entry := filepath.Join(root, "entry.km")
	input := `import {Box} from "./barrel";function run():int{const box=new Box(42);return box.value;}`
	for path, text := range map[string]string{library: `export class Box{constructor(public value:int){}}`, barrel: `export {Box} from "./library";class Box{}`, entry: input} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	messages := serveMessages(t, openDocument(fileURI(entry), input), requestAt("textDocument/completion", 2, fileURI(entry), positionOf(input, "value;", 0), ""))
	found := false
	for _, raw := range messages[2]["result"].([]any) {
		if raw.(map[string]any)["label"] == "value" {
			found = true
		}
	}
	if !found {
		t.Fatalf("completion %v", messages[2])
	}
	at := positionOf(input, "Box(42)", 0)
	at.Character += len("Box(")
	label, _, _ := signatureResult(t, signatureHelpAt(t, entry, input, at))
	if !strings.Contains(label, "value: int") {
		t.Fatalf("signature %q", label)
	}
}
