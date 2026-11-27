package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConsoleStandardModuleEditorSupport(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "main.km")
	input := `import { console, print, println } from "kinmokusei/console"
function main(): void {
  console.log("Hello", 42)
}
`
	completion := strings.Replace(input, `console.log("Hello", 42)`, "console.", 1)
	cursor := positionOf(completion, "console.", 0)
	cursor.Character += len("console.")
	items := completionLabels(completionItemsAt(t, path, completion, cursor.Line, cursor.Character))
	for _, name := range []string{"log", "warn", "error"} {
		if items[name] == nil {
			t.Fatalf("missing console.%s in member completions: %v", name, items)
		}
	}
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "42)", 0)))
	if !strings.Contains(label, "log") || active != 0 || len(parameters) != 1 {
		t.Fatalf("signature: %q, active %v, parameters %v", label, active, parameters)
	}
}
