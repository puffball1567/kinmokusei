package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStructConstraintNavigationAndInference(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "rows.km")
	uri := fileURI(path)
	input := `constraint Rows<E>=Row<E>;
struct Row<E>{public value:E;}
function keep<E,T extends Rows<E>>(v:T):T{return v;}
function use(v:Row<int>):int{const copied=keep(v);return copied.value;}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Row<", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "Row<", 0), `"newName":"Record"`),
		requestAt("textDocument/hover", 4, uri, positionOf(input, "copied", 0), ""),
	)
	definition := messages[2]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(1) || definition["character"] != float64(7) {
		t.Fatal(definition)
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 3 {
		t.Fatal(changes)
	}
	hover := messages[4]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "Row<int>") {
		t.Fatal(hover)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "v);", 0)))
	if !strings.Contains(label, "Row<int>") {
		t.Fatal(label)
	}
}

func TestStructConstraintDiagnosticsAfterEdit(t *testing.T) {
	t.Parallel()
	prefix := `constraint Boxes=Box;struct Box{public n:int;}struct Other{public n:int;}function keep<T extends Boxes>(v:T):T{return v;}function use(box:Box,other:Other):int{const value=keep(`
	checkSplitStorageNavigation(t, prefix+`other);return value.n;}`, prefix+`box);return value.n;}`)
}
