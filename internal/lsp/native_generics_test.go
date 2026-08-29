package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeGenericFunctionNavigationAndRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_navigation.otm")
	uri := fileURI(path)
	text := `function identity<T>(value: T): T { return value; }
function use(value: string): string { return identity(value); }`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/hover", 2, uri, positionOf(text, "identity", 1), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "T", 1), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "T", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "T", 2), `"newName":"Value"`),
	)
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "function identity<T>(value: T): T") {
		t.Fatalf("generic hover = %q", hover)
	}
	definition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(0) || definition["character"] != float64(18) {
		t.Fatalf("type parameter definition = %#v", definition)
	}
	if got := len(messages[4]["result"].([]any)); got != 3 {
		t.Fatalf("type parameter references = %d, want declaration and two uses", got)
	}
	changes := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(changes[uri].([]any)); got != 3 {
		t.Fatalf("type parameter rename edits = %d, want 3", got)
	}
}

func TestNativeGenericFunctionSignatureAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generic_signature.otm")
	text := `function second<T, U>(left: T, right: U): U { return right; }
function use(value: string): string {
  const inferred = second(1, value);
  return second<int, string>(2, inferred);
}`
	tests := []struct {
		name   string
		needle string
		want   string
	}{
		{"inferred", "value);", "second(left: int, right: string): string"},
		{"angle explicit", "inferred);", "second(left: int, right: string): string"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, test.needle, 0)))
			if label != test.want || active != 1 || len(parameters) != 2 {
				t.Fatalf("signature = %q active=%v parameters=%#v", label, active, parameters)
			}
		})
	}

	genericLine := strings.Split(text, "\n")[0]
	genericItems := completionLabels(completionItemsAt(t, path, text, 0, strings.Index(genericLine, "return")))
	if genericItems["T"] == nil || genericItems["U"] == nil {
		t.Fatalf("type parameter completions = %#v", genericItems)
	}
	items := completionLabels(completionItemsAt(t, path, text, 2, 2))
	if detail := items["second"]["detail"]; detail != "function second<T, U>(left: T, right: U): U" {
		t.Fatalf("generic completion detail = %v", detail)
	}
}
