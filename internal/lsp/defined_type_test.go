package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinedTypeNavigationRenameSymbolsAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "defined_type.otm")
	uri := fileURI(path)
	text := `type UserID = distinct string;
alias UserIDText = string;
function use(value: UserIDText): UserID { return UserID(value); }`
	symbolsRequest := fmt.Sprintf(`{"jsonrpc":"2.0","id":6,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "UserID", 4), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "UserID", 4), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "UserID", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "UserID", 3), `"newName":"AccountID"`),
		symbolsRequest,
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "type UserID = distinct string") {
		t.Fatalf("defined type hover = %q", hover)
	}
	definition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(0) || definition["character"] != float64(5) {
		t.Fatalf("defined type definition = %#v", definition)
	}
	if got := len(messages[4]["result"].([]any)); got != 3 {
		t.Fatalf("defined type references = %d, want declaration, return type, and conversion", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 3 {
		t.Fatalf("defined type rename edits = %d, want 3", got)
	}
	symbols := messages[6]["result"].([]any)
	if len(symbols) != 3 || symbols[0].(map[string]any)["detail"] != "type UserID = distinct string" || symbols[1].(map[string]any)["detail"] != "alias UserIDText = string" {
		t.Fatalf("defined type document symbols = %#v", symbols)
	}

	items := completionLabels(completionItemsAt(t, path, text, 2, strings.Index(strings.Split(text, "\n")[2], "return")))
	for name, detail := range map[string]string{
		"UserID":     "type UserID = distinct string",
		"UserIDText": "alias UserIDText = string",
		"type":       "keyword",
		"alias":      "keyword",
		"distinct":   "keyword",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want detail %q", name, items[name], detail)
		}
	}
}
