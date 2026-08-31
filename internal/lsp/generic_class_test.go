package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericClassNavigationRenameSymbolsCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_class.otm")
	uri := fileURI(path)
	text := `class Box<T> {
  constructor(public value: T) {}
  public function get(): T { return this.value; }
  public function set(value: T): void { this.value = value; }
}
function use(value: string): string {
  const box = new Box<string>(value);
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
	if !strings.Contains(hover, "class Box<T>") {
		t.Fatalf("generic class hover = %q", hover)
	}
	typeDefinition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if typeDefinition["line"] != float64(0) || typeDefinition["character"] != float64(10) {
		t.Fatalf("type parameter definition = %#v", typeDefinition)
	}
	if got := len(messages[4]["result"].([]any)); got != 4 {
		t.Fatalf("type parameter references = %d, want declaration and three member uses", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 4 {
		t.Fatalf("type parameter rename edits = %d, want 4", got)
	}
	classDefinition := messages[6]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if classDefinition["line"] != float64(0) || classDefinition["character"] != float64(6) {
		t.Fatalf("generic class definition = %#v", classDefinition)
	}
	symbols := messages[7]["result"].([]any)
	boxSymbol := symbols[0].(map[string]any)
	if boxSymbol["detail"] != "class Box<T>" || len(boxSymbol["children"].([]any)) != 3 {
		t.Fatalf("generic class symbol = %#v", boxSymbol)
	}

	completionText := strings.Replace(text, "box.set(value);", "box.;", 1)
	line := strings.Split(completionText, "\n")[7]
	items := completionLabels(completionItemsAt(t, path, completionText, 7, strings.Index(line, ".")+1))
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
	if label != "new Box(value: string): Box<string>" || active != 0 || len(parameters) != 1 {
		t.Fatalf("generic constructor signature = %q active=%v parameters=%#v", label, active, parameters)
	}

	topLevelItems := completionLabels(completionItemsAt(t, path, text, 5, strings.Index(strings.Split(text, "\n")[5], "function")))
	if topLevelItems["Box"] == nil || topLevelItems["Box"]["detail"] != "class Box<T>" {
		t.Fatalf("generic class top-level completion = %#v", topLevelItems["Box"])
	}
}
