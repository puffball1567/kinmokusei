package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedMainEditor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, library := filepath.Join(root, "entry.km"), filepath.Join(root, "library.km")
	input := `import {main} from "./library";function use():int{return main(41);}`
	if err := os.WriteFile(library, []byte(`export const main=(value:int):int=>value+1;`), 0o644); err != nil {
		t.Fatal(err)
	}
	uri := fileURI(entry)
	at := positionOf(input, "main(41)", 0)
	messages := serveExternalPackageMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"calculate"`))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "main") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["uri"] != fileURI(library) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[fileURI(library)].([]any)) != 1 || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
}
