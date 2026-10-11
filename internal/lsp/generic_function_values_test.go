package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericFunctionValueNavigationAndRename(t *testing.T) {
	t.Parallel()
	input := `type Amount=distinct int;
function identity<T>(value:T):T{return value;}
const first=identity<Amount>;
const second=identity[Amount];
function use(value:Amount):Amount{return first(value);}`
	uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "identity", 1), ""),
		requestAt("textDocument/rename", 3, uri, positionOf(input, "identity", 2), `"newName":"same"`),
		requestAt("textDocument/definition", 4, uri, positionOf(input, "Amount", 2), ""),
		requestAt("textDocument/rename", 5, uri, positionOf(input, "Amount", 1), `"newName":"Quantity"`),
	)
	definition := messages[2]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if definition["line"] != float64(1) {
		t.Fatalf("function definition=%v", definition)
	}
	changes := messages[3]["result"].(map[string]any)["changes"].(map[string]any)
	if len(changes[uri].([]any)) != 3 {
		t.Fatalf("function rename=%v", changes)
	}
	typeDefinition := messages[4]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if typeDefinition["line"] != float64(0) || typeDefinition["character"] != float64(5) {
		t.Fatalf("type argument definition=%v", typeDefinition)
	}
	typeChanges := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if len(typeChanges[uri].([]any)) != 5 {
		t.Fatalf("type rename must not duplicate bracket argument references: %v", typeChanges)
	}
}

func TestGenericFunctionValueSignatures(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function identity<T>(value:T):T{return value;}const f=identity<int>;function use():int{return f(|1);}`,
		`function identity<T>(value:T):T{return value;}const f=identity[int];function use():int{return f(|1);}`,
		`function identity<T>(value:T):T{return value;}function use():int{const f=identity<int>;return f(|1);}`,
	} {
		t.Run(input, func(t *testing.T) {
			text := strings.Replace(input, "|", "", 1)
			label, active, parameters := signatureResult(t, signatureHelpAt(t, filepath.Join(t.TempDir(), "entry.km"), text, positionOf(input, "|", 0)))
			if !strings.Contains(label, ": int") || !strings.HasSuffix(label, "): int") || len(parameters) != 1 || active != float64(0) {
				t.Fatalf("signature=%q active=%v parameters=%v", label, active, parameters)
			}
		})
	}
}
