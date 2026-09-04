package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericInterfaceNavigationRenameSymbolsCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_interface.km")
	uri := fileURI(path)
	text := `interface Transformer<T, U> {
  function transform(value: T): U;
}
class Length implements Transformer<string, int> {
  public function transform(value: string): int { return len(value); }
}
function use(transformer: Transformer<string, int>, value: string): int {
  return transformer.transform(value);
}`
	symbolsRequest := fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "Transformer", 2), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "T", 2), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "T", 1), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "T", 2), `"newName":"Input"`),
		requestAt("textDocument/definition", 6, uri, positionOf(text, "Transformer", 2), ""),
		symbolsRequest,
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "interface Transformer<T, U>") {
		t.Fatalf("generic interface hover = %q", hover)
	}
	typeDefinition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if typeDefinition["line"] != float64(0) || typeDefinition["character"] != float64(22) {
		t.Fatalf("type parameter definition = %#v", typeDefinition)
	}
	if got := len(messages[4]["result"].([]any)); got != 2 {
		t.Fatalf("type parameter references = %d, want declaration and method use", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 2 {
		t.Fatalf("type parameter rename edits = %d, want 2", got)
	}
	interfaceDefinition := messages[6]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if interfaceDefinition["line"] != float64(0) || interfaceDefinition["character"] != float64(10) {
		t.Fatalf("generic interface definition = %#v", interfaceDefinition)
	}
	symbols := messages[7]["result"].([]any)
	contract := symbols[0].(map[string]any)
	if contract["detail"] != "interface Transformer<T, U>" || len(contract["children"].([]any)) != 3 {
		t.Fatalf("generic interface symbol = %#v", contract)
	}

	completionText := strings.Replace(text, "return transformer.transform(value);", "return transformer.;", 1)
	line := strings.Split(completionText, "\n")[7]
	items := completionLabels(completionItemsAt(t, path, completionText, 7, strings.Index(line, ".")+1))
	if items["transform"] == nil || items["transform"]["detail"] != "function transform(value: string): int" {
		t.Fatalf("generic interface completion = %#v", items["transform"])
	}

	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "value);", 1)))
	if label != "transformer.transform(value: string): int" || active != 0 || len(parameters) != 1 {
		t.Fatalf("generic interface signature = %q active=%v parameters=%#v", label, active, parameters)
	}

	topLevelItems := completionLabels(completionItemsAt(t, path, text, 6, strings.Index(strings.Split(text, "\n")[6], "function")))
	if topLevelItems["Transformer"] == nil || topLevelItems["Transformer"]["detail"] != "interface Transformer<T, U>" {
		t.Fatalf("generic interface top-level completion = %#v", topLevelItems["Transformer"])
	}
}
