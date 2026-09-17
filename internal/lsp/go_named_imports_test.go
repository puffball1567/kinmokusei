package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedGoImportNavigation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "named.km")
	uri := fileURI(path)
	input := "import go { Sprint } from \"fmt\"\nfunction run():string { return Sprint(42) }"
	at := positionOf(input, "Sprint(42)", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/hover", 2, uri, at, ""),
		requestAt("textDocument/definition", 3, uri, at, ""),
		requestAt("textDocument/prepareRename", 4, uri, at, ""),
		requestAt("textDocument/completion", 5, uri, at, ""),
	)
	hover, ok := messages[2]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "import go { Sprint }") {
		t.Fatalf("hover: %v", messages[2])
	}
	definition, ok := messages[3]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition: %v", messages[3])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(12) {
		t.Fatalf("definition: %v", definition)
	}
	if messages[4]["result"] != nil {
		t.Fatalf("external name must be read-only: %v", messages[4])
	}
	found := false
	for _, item := range messages[5]["result"].([]any) {
		label := item.(map[string]any)["label"].(string)
		if label == "Sprint" {
			found = true
		}
		if strings.HasPrefix(label, "__kinmokusei_go_") {
			t.Fatalf("internal alias in completion: %s", label)
		}
	}
	if !found {
		t.Fatal("missing named import completion")
	}
}

func TestNamedGoImportSignatureHelp(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "signature.km")
	for _, input := range []string{
		"import go { Compare } from \"cmp\"\nfunction run():int { return Compare(1, 2) }",
		"import go { Compare } from \"cmp\"\nfunction run():int { return Compare(1, ",
	} {
		at := positionOf(input, "Compare(1,", 0)
		at.Character += len("Compare(1,")
		label, active, _ := signatureResult(t, signatureHelpAt(t, path, input, at))
		if !strings.Contains(label, "Compare(") || active != 1 {
			t.Fatalf("signature=%s active=%v", label, active)
		}
	}
}

func TestNamedGoImportKeepsNamespaceNavigation(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "mixed.km"))
	input := "import go fmt from \"fmt\"\nimport go { Sprint } from \"fmt\"\nfunction run():string{return fmt.Sprint(1)+Sprint(2)}"
	at := positionOf(input, "fmt.Sprint", 0)
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/definition", 2, uri, at, ""))
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition: %v", messages[2])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(10) {
		t.Fatalf("namespace definition: %v", start)
	}
}
