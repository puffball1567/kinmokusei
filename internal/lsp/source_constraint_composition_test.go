package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceConstraintCompositionEditor(t *testing.T) {
	t.Parallel()
	input := `constraint Readable<E>=Reader<E>&comparable;
interface Reader<E>{
 function read(fallback:E):E;
 get value():E;
}
function use<T extends Readable<string>>(v:T):string{return v.read("ok");}`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "read(", 1), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "read(", 1), `"newName":"load"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(2) {
		t.Fatalf("definition=%v", messages[2])
	}
	rename, ok := messages[3]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[3])
	}
	items := completionLabels(completionItemsAt(t, path, input, 5, strings.Index(strings.Split(input, "\n")[5], "v.read")+2))
	if items["read"] == nil || items["value"] == nil || items["Read"] != nil || items["get value"] != nil || !strings.Contains(items["read"]["detail"].(string), "string") {
		t.Fatalf("completion=%v", items)
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, `"ok"`, 0)))
	if label != "v.read(fallback: string): string" {
		t.Fatalf("signature=%q", label)
	}
}

func TestSourceConstraintCompositionDiagnosticsAfterEdit(t *testing.T) {
	t.Parallel()
	prefix := `constraint C=Reader&comparable;interface Reader{function read():int;}struct Box{public n:int;public function read():int{return this.n;}}struct Other{public n:int;}function keep<T extends C>(v:T):T{return v;}function use(box:Box,other:Other):int{const value=keep(`
	checkSplitStorageNavigation(t, prefix+`other);return value.n;}`, prefix+`box);return value.n;}`)
}
