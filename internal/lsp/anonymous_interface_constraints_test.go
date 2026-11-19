package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAnonymousInterfaceConstraintEditor(t *testing.T) {
	t.Parallel()
	input := `function use<T extends interface {
 read(fallback:string):string;
}>(v:T):string{return v.read("ok");}
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "read", 1), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "read", 1), `"newName":"load"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(1) {
		t.Fatalf("definition=%v", messages[2])
	}
	rename, ok := messages[3]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[3])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, `"ok"`, 0)))
	if label != "v.read(fallback: string): string" {
		t.Fatalf("signature=%q", label)
	}
	items := completionLabels(completionItemsAt(t, path, input, 2, strings.Index(strings.Split(input, "\n")[2], "read")))
	if items["read"] == nil || items["Read"] != nil || !strings.Contains(items["read"]["detail"].(string), "string") {
		t.Fatalf("completion=%v", items)
	}
}

func TestAnonymousInterfaceConstraintIncompleteSignature(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		`function use<T extends interface{read(fallback:string):string;}>(v:T):string{return v.read(`,
		`function use<T extends interface{read(...values:string[]):string;}>(v:T):string{return v.read(`,
	} {
		label, _, _ := signatureResult(t, signatureHelpAt(t, filepath.Join(t.TempDir(), "entry.km"), text, position{Character: len(text)}))
		if !strings.Contains(label, "v.read(") || !strings.Contains(label, "string") || !strings.HasSuffix(label, "): string") {
			t.Fatalf("signature=%q", label)
		}
	}
}

func TestAnonymousInterfaceEmbeddingEditor(t *testing.T) {
	t.Parallel()
	input := `import go io from "io"
alias Input = interface { io.ReadCloser; size(): int; }
function use(v: Input, buf: byte[]): (int, error) { return v.Read(buf); }
function generic<T extends Input>(v: T, buf: byte[]): (int, error) { return v.Read(buf); }
`
	path := filepath.Join(t.TempDir(), "embedded.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "Read(buf)", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "Read(buf)", 1), ""),
	)
	for _, id := range []float64{2, 3} {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || !strings.HasSuffix(definition["uri"].(string), "/io/io.go") {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "buf);", 0)))
	if !strings.HasPrefix(label, "v.Read(p: byte[])") {
		t.Fatalf("signature=%q", label)
	}
	items := completionLabels(completionItemsAt(t, path, input, 2, strings.Index(strings.Split(input, "\n")[2], "Read")))
	if items["Close"] == nil || items["Read"] == nil || items["size"] == nil {
		t.Fatalf("completion=%v", items)
	}
}
