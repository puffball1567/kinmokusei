package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkedClassFieldInitializerNavigationAndRename(t *testing.T) {
	root := t.TempDir()
	dependency := filepath.Join(root, "defaults.km")
	if err := os.WriteFile(dependency, []byte(`function seed(value: int): int { return value; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "entry.km")
	uri := fileURI(path)
	input := `import { seed } from "./defaults";
class Holder { public value: int = seed(3); }`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "seed(3)", 0), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "seed(3)", 0), `"newName":"initialValue"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != fileURI(dependency) {
		t.Fatalf("definition = %#v", messages[2])
	}
	result, ok := messages[3]["result"].(map[string]any)
	if !ok {
		t.Fatalf("rename = %#v", messages[3])
	}
	changes := result["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 2 || len(changes[fileURI(dependency)].([]any)) != 1 {
		t.Fatalf("rename = %#v", changes)
	}
}

func TestClassFieldInitializerNavigationRenameAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fields.km")
	uri := fileURI(path)
	input := `const seed = 7;
function initial(value: int): int { return value; }
constraint Number = ~int | ~int8;
class Box<T extends Number> {
  public value: T = T(initial(seed));
  constructor(seed: string) {}
}`
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "seed", 1), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "seed", 1), `"newName":"defaultValue"`),
		requestAt("textDocument/rename", 4, uri, positionOf(input, "T(initial", 0), `"newName":"ValueType"`),
		requestAt("textDocument/hover", 5, uri, positionOf(input, "initial(seed)", 0), ""),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("definition = %#v", messages[2])
	}
	start := definition["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(6) {
		t.Fatalf("definition = %#v", definition)
	}
	for id, want := range map[float64]int{3: 2, 4: 3} {
		result, ok := messages[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("rename = %#v", messages[id])
		}
		changes := result["changes"].(map[string]any)
		if got := len(changes[uri].([]any)); got != want {
			t.Fatalf("rename %v edits = %d, want %d", id, got, want)
		}
	}
	hover, ok := messages[5]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "initial(value: int): int") {
		t.Fatalf("hover = %#v", messages[5])
	}
	label, active, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "seed", 1)))
	if label != "initial(value: int): int" || active != 0 {
		t.Fatalf("signature = %q active=%v", label, active)
	}
}
