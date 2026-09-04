package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericStructNavigationRenameSymbolsCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_struct.km")
	uri := fileURI(path)
	text := `struct Box<T> {
  public value: T;
  public function get(): T { return this.value; }
  public pointer function set(value: T): void { this.value = value; }
}
function use(box: *Box<string>, value: string): string {
  box.set(value);
  return box.get();
}`
	symbolsRequest := fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "Box", 1), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "T", 1), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "T", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "T", 2), `"newName":"Value"`),
		requestAt("textDocument/definition", 6, uri, positionOf(text, "Box", 1), ""),
		symbolsRequest,
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "struct Box<T>") {
		t.Fatalf("generic struct hover = %q", hover)
	}
	typeDefinition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if typeDefinition["line"] != float64(0) || typeDefinition["character"] != float64(11) {
		t.Fatalf("type parameter definition = %#v", typeDefinition)
	}
	if got := len(messages[4]["result"].([]any)); got != 4 {
		t.Fatalf("type parameter references = %d, want declaration and three member uses", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 4 {
		t.Fatalf("type parameter rename edits = %d, want 4", got)
	}
	structDefinition := messages[6]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if structDefinition["line"] != float64(0) || structDefinition["character"] != float64(7) {
		t.Fatalf("generic struct definition = %#v", structDefinition)
	}
	symbols := messages[7]["result"].([]any)
	box := symbols[0].(map[string]any)
	if box["detail"] != "struct Box<T>" || len(box["children"].([]any)) != 4 {
		t.Fatalf("generic struct symbol = %#v", box)
	}

	completionText := strings.Replace(text, "box.set(value);", "box.;", 1)
	line := strings.Split(completionText, "\n")[6]
	items := completionLabels(completionItemsAt(t, path, completionText, 6, strings.Index(line, ".")+1))
	for name, detail := range map[string]string{
		"value": "public value: string",
		"get":   "public function get(): string",
		"set":   "public function set(value: string): void",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want %q", name, items[name], detail)
		}
	}

	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "value);", 0)))
	if label != "box.set(value: string): void" || active != 0 || len(parameters) != 1 {
		t.Fatalf("generic method signature = %q active=%v parameters=%#v", label, active, parameters)
	}
}

func TestExternalGenericStructReceiverNavigationCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "external_generic_struct.km")
	uri := fileURI(path)
	text := `struct Box<T> { public value: T; }
public function get<U>(this: Box<U>): U { return this.value; }
public function set<U>(this: *Box<U>, value: U): void { this.value = value; }
function use(box: *Box<string>, value: string): string {
  box.set(value);
  return box.get();
}`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/definition", 2, uri, positionOf(text, "U", 1), ""),
		requestAt("textDocument/references", 3, uri, positionOf(text, "U", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 4, uri, positionOf(text, "U", 1), `"newName":"Value"`),
	)
	definition := messages[2]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(1) || definition["character"] != float64(20) {
		t.Fatalf("receiver type parameter definition = %#v", definition)
	}
	if got := len(messages[3]["result"].([]any)); got != 3 {
		t.Fatalf("receiver type parameter references = %d, want declaration, receiver, and result", got)
	}
	changes := messages[4]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 3 {
		t.Fatalf("receiver type parameter rename edits = %d, want 3", got)
	}

	completionText := strings.Replace(text, "box.set(value);", "box.;", 1)
	line := strings.Split(completionText, "\n")[4]
	items := completionLabels(completionItemsAt(t, path, completionText, 4, strings.Index(line, ".")+1))
	for name, detail := range map[string]string{
		"get": "public function get(): string",
		"set": "public function set(value: string): void",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want %q", name, items[name], detail)
		}
	}

	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "value);", 0)))
	if label != "box.set(value: string): void" || active != 0 || len(parameters) != 1 {
		t.Fatalf("external generic method signature = %q active=%v parameters=%#v", label, active, parameters)
	}
}
