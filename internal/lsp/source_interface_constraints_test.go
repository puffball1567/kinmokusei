package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceInterfaceConstraintEditor(t *testing.T) {
	t.Parallel()
	input := `interface Reader {
 function read(fallback:string):string;
 get value():string;
}
function use<T extends Reader>(v:T):string{return v.read("ok");}
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
	items := completionLabels(completionItemsAt(t, path, input, 4, strings.Index(strings.Split(input, "\n")[4], "read")))
	if items["read"] == nil || items["value"] == nil || items["Read"] != nil || items["get value"] != nil || !strings.Contains(items["read"]["detail"].(string), "string") {
		t.Fatalf("completion=%v", items)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, `"ok"`, 0)))
	if label != "v.read(fallback: string): string" {
		t.Fatalf("signature=%q", label)
	}
}

func TestSourceInterfaceConstraintIncompleteSignature(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		`interface Reader{function read(fallback:string):string;}function use<T extends Reader>(v:T):string{return v.read(`,
		`interface Reader{function read(...values:string[]):string;}function use<T extends Reader>(v:T):string{return v.read(`,
	} {
		label, _, _ := signatureResult(t, signatureHelpAt(t, filepath.Join(t.TempDir(), "entry.km"), text, position{Character: len(text)}))
		if !strings.Contains(label, "v.read(") || !strings.Contains(label, "string") || !strings.HasSuffix(label, "): string") {
			t.Fatalf("signature=%q", label)
		}
	}
}
