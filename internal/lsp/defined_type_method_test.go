package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinedTypeReceiverMethodNavigationRenameCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "defined_type_method.otm")
	uri := fileURI(path)
	text := `type Counter = distinct int;
public function add(this: *Counter, delta: Counter): void { *this += delta; }
public function read(this: Counter): int { return int(this); }
function use(value: Counter, delta: Counter): int {
  value.add(delta);
  return value.read();
}`
	symbolsRequest := fmt.Sprintf(`{"jsonrpc":"2.0","id":8,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "add", 1), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "read", 1), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "add", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "read", 1), `"newName":"value"`),
		requestAt("textDocument/references", 6, uri, positionOf(text, "this", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 7, uri, positionOf(text, "this", 2), `"newName":"self"`),
		symbolsRequest,
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "function add(delta: Counter): void") {
		t.Fatalf("defined type method hover = %q", hover)
	}
	definition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(2) || definition["character"] != float64(16) {
		t.Fatalf("defined type method definition = %#v", definition)
	}
	if got := len(messages[4]["result"].([]any)); got != 2 {
		t.Fatalf("method references = %d, want declaration and call", got)
	}
	methodChanges := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(methodChanges[uri].([]any)); got != 2 {
		t.Fatalf("method rename edits = %d, want declaration and call", got)
	}
	if got := len(messages[6]["result"].([]any)); got != 2 {
		t.Fatalf("receiver references = %d, want declaration and dereference", got)
	}
	if messages[7]["result"] != nil {
		t.Fatalf("receiver keyword rename should be rejected, got %#v", messages[7]["result"])
	}
	symbols := messages[8]["result"].([]any)
	if len(symbols) != 4 || symbols[1].(map[string]any)["detail"] != "function add(delta: Counter): void" || symbols[2].(map[string]any)["detail"] != "function read(): int" {
		t.Fatalf("defined type method symbols = %#v", symbols)
	}

	completionText := strings.Replace(text, "value.add(delta);", "value.;", 1)
	line := strings.Split(completionText, "\n")[4]
	items := completionLabels(completionItemsAt(t, path, completionText, 4, strings.Index(line, ".")+1))
	for name, detail := range map[string]string{
		"add":  "public function add(delta: Counter): void",
		"read": "public function read(): int",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want %q", name, items[name], detail)
		}
	}

	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "delta);", 0)))
	if label != "value.add(delta: Counter): void" || active != 0 || len(parameters) != 1 {
		t.Fatalf("defined type method signature = %q active=%v parameters=%#v", label, active, parameters)
	}
}
