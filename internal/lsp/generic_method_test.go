package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericMethodNavigationSignatureAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_method.km")
	uri := fileURI(path)
	text := `constraint Integer = ~int | ~int8;
class Box<T> {
  public function echo<U extends Integer>(value: U): U {
    return value;
  }
}
function use(box: Box<string>): int { return box.echo<int>(1); }`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "echo<int>", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "echo<int>", 0), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "U extends", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "U extends", 0), `"newName":"V"`),
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "function echo<U extends Integer>(value: U): U") {
		t.Fatalf("generic method hover = %q", hover)
	}
	definition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(2) {
		t.Fatalf("generic method definition = %#v", definition)
	}
	if got := len(messages[4]["result"].([]any)); got != 3 {
		t.Fatalf("generic method type parameter references = %d, want declaration and two uses", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 3 {
		t.Fatalf("generic method type parameter rename edits = %d, want 3", got)
	}
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "1);", 0)))
	if label != "box.echo(value: int): int" || active != 0 || len(parameters) != 1 {
		t.Fatalf("generic method signature = %q active=%v parameters=%#v", label, active, parameters)
	}
	items := completionLabels(completionItemsAt(t, path, text, 3, 4))
	if items["T"] == nil || items["U"] == nil {
		t.Fatalf("generic method type parameter completions = %#v", items)
	}
}
