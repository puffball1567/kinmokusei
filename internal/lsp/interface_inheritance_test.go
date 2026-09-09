package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInterfaceInheritanceNavigationCompletionSignatureAndRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inheritance.km")
	uri := fileURI(path)
	text := `interface Parent<T, U> { function transform(value: T): U; }
interface Left<A, B> extends Parent<B, A> {}
interface Right<A, B> extends Parent<B, A> {}
interface Child<A, B> extends Left<A, B>, Right<A, B> {}
class Length implements Child<int, string> {
  public function transform(value: string): int { return len(value); }
}
function use(child: Child<int, string>): int {
  return child.transform("hello");
}`
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/definition", 2, uri, positionOf(text, "Parent", 1), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "transform", 2), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(text, "transform", 0), `"newName":"convert"`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "Parent", 0), `"newName":"Root"`),
		requestAt("textDocument/hover", 6, uri, positionOf(text, "Child", 0), ""),
	)
	for _, id := range []int{2, 3} {
		if messages[float64(id)]["result"] == nil {
			t.Fatalf("definition %d returned %#v; messages = %#v", id, messages[float64(id)], messages)
		}
		start := messages[float64(id)]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
		if start["line"] != float64(0) {
			t.Fatalf("definition %d = %#v", id, start)
		}
	}
	for _, id := range []int{4, 5} {
		changes := messages[float64(id)]["result"].(map[string]any)["changes"].(map[string]any)
		if len(changes[uri].([]any)) != 3 {
			t.Fatalf("rename %d = %#v", id, changes)
		}
	}
	hover := messages[6]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "interface Child<A, B> extends Left<A, B>, Right<A, B>") {
		t.Fatalf("hover = %s", hover)
	}
	completionText := strings.Replace(text, `return child.transform("hello");`, `return child.;`, 1)
	items := completionLabels(completionItemsAt(t, path, completionText, 8, len("  return child.")))
	if items["transform"] == nil || items["transform"]["detail"] != "function transform(value: string): int" {
		t.Fatalf("completion = %#v", items)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, `"hello"`, 0)))
	if label != "child.transform(value: string): int" {
		t.Fatalf("signature = %q", label)
	}
}
