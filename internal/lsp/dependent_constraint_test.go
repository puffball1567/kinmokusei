package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestDependentConstraintNavigationAndInference(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "constraints")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): "[project]\nname = \"bounds\"\nversion = \"0.1.0\"\ngo-module = \"bounds-editor.test\"\ngo-version = \"1.23\"\n",
		filepath.Join(library, "go.mod"):             "module bounds-editor.test/constraints\n\ngo 1.23\n",
		filepath.Join(library, "constraints.go"):     "package constraints\ntype Slice[E any] interface { ~[]E }\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := project.AddDependency(root, "bounds-editor.test/constraints", "v0.0.0", "./constraints", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.km")
	uri := fileURI(path)
	input := `import go c from "bounds-editor.test/constraints";
function elements<S extends c.Slice<E>, E>(values:S):E[] {
 let result:E[]=[]; for(const value of values){result=append(result,value);} return result;
}
function use(values:int[]):int[] { const result=elements(values); return result; }
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "E>", 0), ""),
		requestAt("textDocument/references", 3, uri, positionOf(input, "E>", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "E>", 0), `"newName":"Element"`),
		requestAt("textDocument/hover", 5, uri, positionOf(input, "elements(values)", 0), ""),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition=%#v", messages[2])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	want := positionOf(input, "E>(values", 0)
	if start["line"] != float64(want.Line) || start["character"] != float64(want.Character) {
		t.Fatalf("definition=%#v want=%#v", start, want)
	}
	if references, ok := messages[3]["result"].([]any); !ok || len(references) != 4 {
		t.Fatalf("references=%#v", messages[3])
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 4 {
		t.Fatalf("rename=%#v", changes)
	}
	hover, ok := messages[5]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "function elements<S extends c.Slice<E>, E>(values: S): E[]") {
		t.Fatalf("hover=%#v", messages[5])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "values); return", 0)))
	if label != "elements(values: int[]): int[]" {
		t.Fatalf("signature=%q", label)
	}
}
