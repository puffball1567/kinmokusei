package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceExportNavigationAndRename(t *testing.T) {
	t.Parallel()
	for _, inline := range []bool{false, true} {
		root := t.TempDir()
		library := filepath.Join(root, "library.km")
		entry := filepath.Join(root, "entry.km")
		libraryText := "export { value }\nfunction value(n:int):int{return n+1}"
		if inline {
			libraryText = "export function value(n:int):int{return n+1}"
		}
		entryText := "import { value } from \"./library\"\nfunction run():int{return value(2)}"
		if err := os.WriteFile(library, []byte(libraryText), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(entry, []byte(entryText), 0o644); err != nil {
			t.Fatal(err)
		}
		uri := fileURI(entry)
		messages := serveMessages(t,
			openDocument(fileURI(library), libraryText), openDocument(uri, entryText),
			requestAt("textDocument/hover", 2, fileURI(library), positionOf(libraryText, "value", 0), ""),
			requestAt("textDocument/definition", 3, fileURI(library), positionOf(libraryText, "value", 0), ""),
			requestAt("textDocument/references", 4, uri, positionOf(entryText, "value", 1), `"context":{"includeDeclaration":true}`),
			requestAt("textDocument/rename", 5, uri, positionOf(entryText, "value", 1), `"newName":"increment"`),
		)
		hover, ok := messages[2]["result"].(map[string]any)
		if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "value(") {
			t.Fatalf("hover: %v", messages[2])
		}
		definition, ok := messages[3]["result"].(map[string]any)
		if !ok {
			t.Fatalf("definition: %v", messages[3])
		}
		line := float64(1)
		count := 4
		if inline {
			line = 0
			count = 3
		}
		if definition["range"].(map[string]any)["start"].(map[string]any)["line"] != line {
			t.Fatal(definition)
		}
		if refs, ok := messages[4]["result"].([]any); !ok || len(refs) != count {
			t.Fatalf("references: %v", messages[4])
		}
		rename, ok := messages[5]["result"].(map[string]any)
		if !ok {
			t.Fatalf("rename: %v", messages[5])
		}
		changes := rename["changes"].(map[string]any)
		if len(changes[fileURI(library)].([]any))+len(changes[uri].([]any)) != count {
			t.Fatalf("rename edits: %v", changes)
		}
	}
}

func TestSourceExportCompletionVisibility(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "library.km")
	other := filepath.Join(root, "other.km")
	entry := filepath.Join(root, "entry.km")
	libraryText := "export function visible():int{return hidden()}\nfunction hidden():int{return 1}"
	if err := os.WriteFile(library, []byte(libraryText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte(`function legacy():int{return 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	entryText := "import { visible, hidden } from \"./library\"\nimport { legacy } from \"./other\"\nfunction run():void {}"
	messages := serveMessages(t, openDocument(fileURI(entry), entryText), openDocument(fileURI(library), libraryText),
		requestAt("textDocument/completion", 2, fileURI(entry), positionOf(entryText, "function run", 0), ""),
		requestAt("textDocument/completion", 3, fileURI(library), positionOf(libraryText, "function hidden", 0), ""),
	)
	for _, test := range []struct {
		id              float64
		present, absent []string
	}{
		{2, []string{"visible", "legacy"}, []string{"hidden"}},
		{3, []string{"visible", "hidden"}, []string{"legacy"}},
	} {
		labels := map[string]bool{}
		for _, raw := range messages[test.id]["result"].([]any) {
			labels[raw.(map[string]any)["label"].(string)] = true
		}
		for _, name := range test.present {
			if !labels[name] {
				t.Fatalf("missing %s: %v", name, labels)
			}
		}
		for _, name := range test.absent {
			if labels[name] {
				t.Fatalf("unexpected %s: %v", name, labels)
			}
		}
	}
}
