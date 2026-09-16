package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNumericConstantAliasEditor(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "constants.km"))
	input := `const original=2.0;const offset=original;function use(xs:int[]):int{return xs[offset];}`
	at := positionOf(input, "offset];", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/rename", 4, uri, at, `"newName":"index"`))
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "const offset") {
		t.Fatalf("hover=%v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(strings.Index(input, "offset=")) {
		t.Fatalf("definition=%v", messages[3])
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
}

func TestScalarConstantAliasEditor(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`const original="ab";const alias=original+"c";function use():string{return alias;}`,
		`const original=true;const alias=!original;function use():boolean{return alias;}`,
		`const original="温泉";const alias=len(original);function use():int{return alias;}`,
		`function use(original:[3]int):int{const alias=len(original);return alias;}`,
		`function use(original:*[3]int):int{const alias=cap(original);return alias;}`,
		`const original=255;const alias=min(original,256);function use():byte{return alias;}`,
		`const original="温泉";const alias=max(original,"a");function use():string{return alias;}`,
	} {
		t.Run(input, func(t *testing.T) {
			uri := fileURI(filepath.Join(t.TempDir(), "constants.km"))
			at := positionOf(input, "alias;", 0)
			messages := serveMessages(t, openDocument(uri, input),
				requestAt("textDocument/hover", 2, uri, at, ""),
				requestAt("textDocument/definition", 3, uri, at, ""),
				requestAt("textDocument/rename", 4, uri, at, `"newName":"copied"`))
			hover, ok := messages[2]["result"].(map[string]any)
			if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "const alias") {
				t.Fatalf("hover=%v", messages[2])
			}
			definition, ok := messages[3]["result"].(map[string]any)
			if !ok || definition["range"].(map[string]any)["start"].(map[string]any)["character"] != float64(utf16Length(input[:strings.Index(input, "alias=")])) {
				t.Fatalf("definition=%v", messages[3])
			}
			rename, ok := messages[4]["result"].(map[string]any)
			if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
				t.Fatalf("rename=%v", messages[4])
			}
		})
	}
}
