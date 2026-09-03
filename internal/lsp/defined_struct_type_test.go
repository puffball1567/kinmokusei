package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinedStructFieldCompletionAndNavigation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "defined_struct_type.otm")
	uri := fileURI(path)
	text := `struct Box<T> {
  public value: T;
  public function original(): T { return this.value; }
}
type NamedBox<T> = distinct Box<T>;
public function read<U>(this: NamedBox<U>): U { return this.value; }
function make(value: string): NamedBox<string> { return NamedBox<string> { value: value }; }
function use(value: NamedBox<string>): string { return value.value; }`

	fieldUse := positionOf(text, "value.value", 0)
	fieldUse.Character += len("value.")
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/definition", 2, uri, fieldUse, ""),
	)
	definition := messages[2]["result"].(map[string]any)
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(1) || start["character"] != float64(9) {
		t.Fatalf("defined struct field definition = %#v", definition)
	}

	completionText := strings.Replace(text, "return value.value;", "return value.;", 1)
	line := strings.Split(completionText, "\n")[7]
	items := completionLabels(completionItemsAt(t, path, completionText, 7, strings.Index(line, ".")+1))
	for name, detail := range map[string]string{
		"value": "public value: string",
		"read":  "public function read(): string",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("completion %q = %#v, want %q", name, items[name], detail)
		}
	}
	if items["original"] != nil {
		t.Fatalf("distinct struct completion inherited base method: %#v", items["original"])
	}
}
