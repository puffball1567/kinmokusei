package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstraintIntersectionEditor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "library.km")
	if err := os.WriteFile(library, []byte("export constraint Base<E> = ~E[];\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "entry.km")
	uri := fileURI(path)
	input := `import { Base } from "./library";
constraint Slice<E> = Base<E> & ~E[];
function copy<S extends Slice<E>,E>(xs:S):E[]{let result:E[]=[];for(const x of xs){result=append(result,x);}return result;}
function use(xs:int[]):int[]{return copy(xs);}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Base<E>", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "Base<E>", 0), `"newName":"Storage"`),
		requestAt("textDocument/hover", 4, uri, positionOf(input, "Slice", 1), ""),
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
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "constraint Slice<E> = Base<E> & ~E[]") {
		t.Fatalf("hover=%#v", messages[4])
	}
	items := completionLabels(completionItemsAt(t, path, input, 3, 0))
	if items["Slice"] == nil || items["Slice"]["detail"] != "constraint Slice<E> = Base<E> & ~E[]" {
		t.Fatalf("completion=%#v", items["Slice"])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "xs);}", 0)))
	if label != "copy(xs: int[]): int[]" {
		t.Fatalf("signature=%q", label)
	}
}
