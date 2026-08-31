package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericDefinedTypeNavigationRenameSymbolsAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_defined_type.otm")
	uri := fileURI(path)
	text := `type Lookup<K, V> = distinct Map<K, V>;
function use(values: Lookup<string, int>): int { return values["answer"]; }`
	symbolsRequest := fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "Lookup", 1), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "K", 1), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "V", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "K", 1), `"newName":"Key"`),
		requestAt("textDocument/definition", 6, uri, positionOf(text, "Lookup", 1), ""),
		symbolsRequest,
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "type Lookup<K, V> = distinct Map<K, V>") {
		t.Fatalf("generic defined type hover = %q", hover)
	}
	parameterDefinition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if parameterDefinition["line"] != float64(0) || parameterDefinition["character"] != float64(12) {
		t.Fatalf("type parameter definition = %#v", parameterDefinition)
	}
	if got := len(messages[4]["result"].([]any)); got != 2 {
		t.Fatalf("type parameter references = %d, want declaration and underlying use", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 2 {
		t.Fatalf("type parameter rename edits = %d, want 2", got)
	}
	typeDefinition := messages[6]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if typeDefinition["line"] != float64(0) || typeDefinition["character"] != float64(5) {
		t.Fatalf("generic defined type definition = %#v", typeDefinition)
	}
	symbols := messages[7]["result"].([]any)
	lookup := symbols[0].(map[string]any)
	if lookup["detail"] != "type Lookup<K, V> = distinct Map<K, V>" || len(lookup["children"].([]any)) != 2 {
		t.Fatalf("generic defined type symbol = %#v", lookup)
	}

	items := completionLabels(completionItemsAt(t, path, text, 0, strings.Index(text, "Map")))
	for name, detail := range map[string]string{
		"K":      "type parameter K",
		"V":      "type parameter V",
		"Lookup": "type Lookup<K, V> = distinct Map<K, V>",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want detail %q", name, items[name], detail)
		}
	}
}

func TestGenericDefinedTypeReceiverCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_defined_type_method.otm")
	text := `type Values<T> = distinct T[];
public function size<U>(this: Values<U>): int { return len(this); }
public function push<U>(this: *Values<U>, value: U): void { *this = append(*this, value); }
function use(values: Values<string>, value: string): int {
  let copy = values;
  copy.push(value);
  return copy.size();
}`
	completionText := strings.Replace(text, "copy.push(value);", "copy.;", 1)
	line := strings.Split(completionText, "\n")[5]
	items := completionLabels(completionItemsAt(t, path, completionText, 5, strings.Index(line, ".")+1))
	for name, detail := range map[string]string{
		"size": "public function size(): int",
		"push": "public function push(value: string): void",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want %q", name, items[name], detail)
		}
	}
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "value);", 1)))
	if label != "copy.push(value: string): void" || active != 0 || len(parameters) != 1 {
		t.Fatalf("generic defined method signature = %q active=%v parameters=%#v", label, active, parameters)
	}
}
