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
function use(box: Box<string>): int { return box.echo<int>(1); }
function direct(): int { return new Box<string>().echo<int>(2); }`
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
	directLabel, directActive, directParameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "2);", 0)))
	if directLabel != "echo(value: int): int" || directActive != 0 || len(directParameters) != 1 {
		t.Fatalf("expression-receiver generic method signature = %q active=%v parameters=%#v", directLabel, directActive, directParameters)
	}
	items := completionLabels(completionItemsAt(t, path, text, 3, 4))
	if items["T"] == nil || items["U"] == nil {
		t.Fatalf("generic method type parameter completions = %#v", items)
	}
}

func TestInheritedGenericMethodSignatureKeepsTypeParameterScopes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_method_scopes.km")
	text := `class Base<T> {
  constructor(public value: T) {}
  public function keep<U>(marker: U): T { return this.value; }
}
class Child<U> extends Base<U> { constructor(value: U) { super(value); } }
function use(child: Child<string>): string { return child.keep<int>(1); }`
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "1);", 0)))
	if label != "child.keep(marker: int): string" || active != 0 || len(parameters) != 1 {
		t.Fatalf("inherited generic method signature = %q active=%v parameters=%#v", label, active, parameters)
	}
}
