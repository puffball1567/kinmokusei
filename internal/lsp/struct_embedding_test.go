package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStructEmbeddingEditor(t *testing.T) {
	t.Parallel()
	input := `struct Cell<T> {
  public value: T;
  public function read(): T { return this.value; }
}
struct Layer { Cell<int>; }
function use(x: Layer): int { return x.value + x.read() + x.Cell.value; }
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "value +", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "read()", 1), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "Cell.value", 0), `"newName":"Box"`),
	)
	for id, want := range map[float64]float64{2: 1, 3: 2} {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != want {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename=%v", messages[4])
	}
	line := strings.Split(input, "\n")[5]
	items := completionLabels(completionItemsAt(t, path, input, 5, strings.Index(line, "value +")))
	if items["value"] == nil || items["value"]["detail"] != "value: int" {
		t.Fatalf("field completion=%v", items)
	}
	if items["read"] == nil || !strings.Contains(items["read"]["detail"].(string), "int") {
		t.Fatalf("method completion=%v", items)
	}
}

func TestImportedGoStructEmbeddingEditor(t *testing.T) {
	t.Parallel()
	input := `import go bytes from "bytes"
struct Output { bytes.Buffer; }
function length(v: *Output): int { return v.Len(); }
function read(v: *Output): int { const f = (*Output).Len; return f(v); }
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Len", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "Len", 1), ""),
	)
	for id := float64(2); id <= 3; id++ {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || !strings.HasSuffix(definition["uri"].(string), "/bytes/buffer.go") {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	line := strings.Split(input, "\n")[2]
	items := completionLabels(completionItemsAt(t, path, input, 2, strings.Index(line, "Len")))
	if items["Len"] == nil || !strings.Contains(items["Len"]["detail"].(string), "int") || items["buf"] != nil {
		t.Fatalf("completion=%v", items)
	}
}

func TestGoInterfaceEmbeddingEditor(t *testing.T) {
	t.Parallel()
	input := `import go io from "io"
struct Forward { io.Writer; }
function send(v: Forward, p: byte[]): (int, error) { return v.Write(p); }
function take(v: Forward, p: byte[]): (int, error) { const f = Forward.Write; return f(v, p); }
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	member := positionOf(input, "Write(p)", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, member, ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "Write;", 0), ""),
	)
	for id := float64(2); id <= 3; id++ {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || !strings.HasSuffix(definition["uri"].(string), "/io/io.go") {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	items := completionLabels(completionItemsAt(t, path, input, member.Line, member.Character))
	if items["Writer"] == nil || items["Write"] == nil || !strings.Contains(items["Write"]["detail"].(string), "error") {
		t.Fatalf("completion=%v", items)
	}
}
