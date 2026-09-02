package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericAliasNavigationRenameSymbolsAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_alias.otm")
	uri := fileURI(path)
	text := `alias Lookup<K, V> = Map<K, V>;
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
	if !strings.Contains(hover, "alias Lookup<K, V> = Map<K, V>") {
		t.Fatalf("generic alias hover = %q", hover)
	}
	parameterDefinition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if parameterDefinition["line"] != float64(0) || parameterDefinition["character"] != float64(13) {
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
	if typeDefinition["line"] != float64(0) || typeDefinition["character"] != float64(6) {
		t.Fatalf("generic alias definition = %#v", typeDefinition)
	}
	symbols := messages[7]["result"].([]any)
	lookup := symbols[0].(map[string]any)
	if lookup["detail"] != "alias Lookup<K, V> = Map<K, V>" || len(lookup["children"].([]any)) != 2 {
		t.Fatalf("generic alias symbol = %#v", lookup)
	}

	items := completionLabels(completionItemsAt(t, path, text, 0, strings.Index(text, "Map")))
	for name, detail := range map[string]string{
		"K":      "type parameter K",
		"V":      "type parameter V",
		"Lookup": "alias Lookup<K, V> = Map<K, V>",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want detail %q", name, items[name], detail)
		}
	}
}

func TestGenericAliasToClassMemberCompletionUsesTypeArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_alias_member.otm")
	text := `class Box<T> {
  constructor(public value: T) {}
  public function get(): T { return this.value; }
}
alias BoxRef<T> = Box<T>;
function use(box: BoxRef<string>): string {
  return box.;
}`
	line := strings.Split(text, "\n")[6]
	items := completionLabels(completionItemsAt(t, path, text, 6, strings.Index(line, ".")+1))
	for name, detail := range map[string]string{
		"value": "public value: string",
		"get":   "public function get(): string",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want detail %q", name, items[name], detail)
		}
	}
}
