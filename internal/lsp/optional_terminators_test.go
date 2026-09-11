package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionalTerminatorsKeepDiagnosticsAndNavigation(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "terminators.km"))
	input := "function double(value: int): int { return value * 2 }\nfunction run(): int {\n  const count: int = double(2)\n  return count\n}"
	open := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"version":1,"text":%q}}}`, uri, input)
	hover := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":3,"character":10}}}`, uri)
	definition := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":3,"character":10}}}`, uri)
	var output bytes.Buffer
	err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, open, hover, definition, `{"jsonrpc":"2.0","id":4,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[float64]map[string]any{}
	for _, message := range decodeMessages(t, output.String()) {
		if id, ok := message["id"].(float64); ok {
			byID[id] = message
		}
		if message["method"] == "textDocument/publishDiagnostics" {
			if diagnostics := message["params"].(map[string]any)["diagnostics"].([]any); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
		}
	}
	text := byID[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(text, "count: int") {
		t.Fatalf("hover: %s", text)
	}
	start := byID[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(2) || start["character"] != float64(8) {
		t.Fatalf("definition: %v", start)
	}
}
