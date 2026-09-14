package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstraintMethodEditor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := filepath.Join(root, "bounds.km")
	if err := os.WriteFile(library, []byte(`import go fmt from "fmt"; export constraint Printable=~int&fmt.Stringer;`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "entry.km")
	uri := fileURI(path)
	input := `import {Printable} from "./bounds";
constraint Number=Printable&~int;
function show<T extends Number>(value:T):string{return value.String();}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Printable&", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "Printable&", 0), `"newName":"Printed"`),
		requestAt("textDocument/hover", 4, uri, positionOf(input, "Number>", 0), ""),
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
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "constraint Number = Printable & ~int") {
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

func TestConstraintMethodCompletionScopes(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`function show<T extends Printed>(value:T):string{const other=value;return other.String();}`,
		`function show<T extends fmt.Stringer>(value:T):string{return value.String();}`,
		`class Box<T extends Printed>{public function show(value:T):string{return value.String();}}`,
		`class Box<E>{public function show<T extends Printed>(value:T):string{return value.String();}}`,
		`struct Box<T extends Printed>{public function show(value:T):string{return value.String();}}`,
		`struct Box<E>{public function show<T extends Printed>(value:T):string{return value.String();}}`,
	} {
		input := `import go fmt from "fmt"; constraint Printed=~int&fmt.Stringer;` + body
		path := filepath.Join(t.TempDir(), "entry.km")
		position := positionOf(input, "();}", 0)
		items := completionLabels(completionItemsAt(t, path, input, position.Line, position.Character))
		if items["String"] == nil || !strings.Contains(items["String"]["detail"].(string), "string") {
			t.Fatalf("%s: completion=%#v", body, items)
		}
	}
}
