package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedConstraintEditor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "bounds.km")
	if err := os.WriteFile(library, []byte(`import go cmp from "cmp"; export constraint Integer=cmp.Ordered&~int;`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "entry.km")
	uri := fileURI(path)
	input := `import {Integer} from "./bounds";
import go fmt from "fmt";
constraint Named=Integer&fmt.Stringer;
function show<T extends Named>(value:T):string{return value.String();}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Integer&", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "Integer&", 0), `"newName":"Number"`),
		requestAt("textDocument/hover", 4, uri, positionOf(input, "Named>", 0), ""),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != fileURI(library) {
		t.Fatalf("definition=%#v", messages[2])
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 || len(changes[fileURI(library)].([]any)) != 1 {
		t.Fatalf("rename=%#v", changes)
	}
	hover, ok := messages[4]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "constraint Named = Integer & fmt.Stringer") {
		t.Fatalf("hover=%#v", messages[4])
	}
	position := positionOf(input, "();}", 0)
	items := completionLabels(completionItemsAt(t, path, input, position.Line, position.Character))
	if items["String"] == nil {
		t.Fatalf("method completion=%#v", items)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, ");}", 0)))
	if !strings.Contains(label, "String()") || !strings.Contains(label, "string") {
		t.Fatalf("signature=%q", label)
	}
}
