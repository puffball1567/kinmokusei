package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericSliceBuiltinsEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "slices.km")
	uri := fileURI(path)
	input := `type Numbers=distinct int[];
constraint Slice=~int[];
function grow<S extends Slice>(values:S):S{const result=append(values,1);copy(result,values);return result;}
function use(values:Numbers):void{const inferred=grow(values);}
`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, positionOf(input, "result;", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "result;", 0), ""),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "result;", 0), `"newName":"extended"`))
	for _, message := range messages {
		if message["method"] == "textDocument/publishDiagnostics" && len(message["params"].(map[string]any)["diagnostics"].([]any)) != 0 {
			t.Fatalf("diagnostics=%v", message)
		}
	}
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "result: S") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(2) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename=%v", messages[4])
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "values);}", 0)))
	if label != "grow(values: Numbers): Numbers" {
		t.Fatalf("signature=%q", label)
	}
}
