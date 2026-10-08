package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArrayLengthReferencesAndRenameRespectScope(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "length.km"))
	text := `const Width=3;
function outer(a:[Width]int):[Width]int{return a;}
function inner():int{const Width=2;const a:[Width]int=[1,2];return len(a);}`
	messages := serveMessages(t, openDocument(uri, text),
		requestAt("textDocument/references", 2, uri, positionOf(text, "Width", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/references", 3, uri, positionOf(text, "Width", 3), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/definition", 4, uri, positionOf(text, "Width", 4), ""),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "Width", 1), `"newName":"Count"`),
		requestAt("textDocument/rename", 6, uri, positionOf(text, "Width", 4), `"newName":"LocalSize"`),
	)
	for id, count := range map[float64]int{2: 3, 3: 2} {
		result, ok := messages[id]["result"].([]any)
		if !ok || len(result) != count {
			t.Fatalf("references %v=%#v; want %d", id, messages[id], count)
		}
	}
	definition, ok := messages[4]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(2) {
		t.Fatalf("local definition=%#v", messages[4])
	}
	for id, count := range map[float64]int{5: 3, 6: 2} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("rename %v=%#v", id, messages[id])
		}
		changes := result["changes"].(map[string]any)
		if len(changes[uri].([]any)) != count {
			t.Fatalf("rename %v=%#v; want %d edits", id, changes, count)
		}
	}
}

func TestArrayLengthImportAliasesRetainNavigation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	libURI := fileURI(filepath.Join(root, "lib.km"))
	libText := `export const Width=3;`
	if err := os.WriteFile(filepath.Join(root, "lib.km"), []byte(libText), 0o644); err != nil {
		t.Fatal(err)
	}
	uri := fileURI(filepath.Join(root, "entry.km"))
	text := `import {Width as Count} from "./lib";
import go {Size as DigestSize} from "crypto/sha256";
alias Row=[Count]int;
alias Digest=[DigestSize]byte;
function use():int{const Count=2;const a:[Count]int=[1,2];return len(a);}`
	messages := serveMessages(t, openDocument(libURI, libText), openDocument(uri, text),
		requestAt("textDocument/definition", 2, uri, positionOf(text, "Count", 1), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(text, "Count", 1), `"newName":"RowSize"`),
		requestAt("textDocument/definition", 4, uri, positionOf(text, "DigestSize", 1), ""),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "DigestSize", 1), `"newName":"HashSize"`),
		requestAt("textDocument/prepareRename", 6, uri, positionOf(text, "Size as", 0), ""),
	)
	for id, line := range map[float64]int{2: 0, 4: 1} {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(line) {
			t.Fatalf("alias definition %v=%#v", id, messages[id])
		}
	}
	for _, id := range []float64{3, 5} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("alias rename %v=%#v", id, messages[id])
		}
		changes := result["changes"].(map[string]any)
		if len(changes) != 1 || len(changes[uri].([]any)) != 2 {
			t.Fatalf("alias rename crossed source identities: %#v", changes)
		}
	}
	if messages[6]["result"] != nil {
		t.Fatalf("Go export must remain read-only: %#v", messages[6])
	}
}
