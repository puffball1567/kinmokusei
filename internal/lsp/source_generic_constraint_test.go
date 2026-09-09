package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceGenericConstraintNavigationAndInference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bounds.km")
	uri := fileURI(path)
	input := `constraint Slice<E>=~E[];
function elements<S extends Slice<E>, E>(values:S):E[] {
 let result:E[]=[];for(const value of values){result=append(result,value);}return result;
}
function use(values:int[]):int[]{return elements(values);}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "E[]", 0), ""),
		requestAt("textDocument/references", 3, uri, positionOf(input, "E[]", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "E[]", 0), `"newName":"Element"`),
		requestAt("textDocument/hover", 5, uri, positionOf(input, "Slice", 1), ""),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition=%#v", messages[2])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	want := positionOf(input, "E>", 0)
	if start["line"] != float64(want.Line) || start["character"] != float64(want.Character) {
		t.Fatalf("definition=%#v want=%#v", start, want)
	}
	if references, ok := messages[3]["result"].([]any); !ok || len(references) != 2 {
		t.Fatalf("references crossed parameter scopes: %#v", messages[3])
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 {
		t.Fatalf("rename=%#v", changes)
	}
	hover, ok := messages[5]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "constraint Slice<E> = ~E[]") {
		t.Fatalf("hover=%#v", messages[5])
	}
	items := completionLabels(completionItemsAt(t, path, input, 4, 0))
	if items["Slice"] == nil || items["Slice"]["detail"] != "constraint Slice<E> = ~E[]" {
		t.Fatalf("completion=%#v", items["Slice"])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "values);}", 0)))
	if label != "elements(values: int[]): int[]" {
		t.Fatalf("signature=%q", label)
	}
}

func TestSourceGenericConstraintReuseNavigation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reuse.km")
	uri := fileURI(path)
	input := "constraint Base<T> = ~T[];\nconstraint Slice<E> = Base<E>;\n"
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Base", 1), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "Base", 1), `"newName":"Storage"`),
		requestAt("textDocument/hover", 4, uri, positionOf(input, "Slice", 0), ""),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition=%#v", messages[2])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	want := positionOf(input, "Base", 0)
	if start["line"] != float64(want.Line) || start["character"] != float64(want.Character) {
		t.Fatalf("definition=%#v want=%#v", start, want)
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 {
		t.Fatalf("rename=%#v", changes)
	}
	hover, ok := messages[4]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "constraint Slice<E> = Base<E>") {
		t.Fatalf("hover=%#v", messages[4])
	}
}
