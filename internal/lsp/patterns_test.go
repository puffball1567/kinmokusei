package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTypedMatchEditor(t *testing.T) {
	t.Parallel()
	input := `import go os from "os"
alias PathError = *os.PathError
function path(problem: error): bstring {
  return match(problem) {
    case const detail as PathError => detail.Path
    default => b""
  }
}
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	use := positionOf(input, "detail.Path", 0)
	target := positionOf(input, "PathError =>", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, use, ""),
		requestAt("textDocument/definition", 3, uri, target, ""),
		requestAt("textDocument/rename", 4, uri, use, `"newName":"cause"`),
	)
	for id, line := range map[float64]float64{2: 4, 3: 1} {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != line {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	rename, ok := messages[4]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[4])
	}
	items := completionLabels(completionItemsAt(t, path, input, use.Line, use.Character+len("detail.")))
	if items["Path"] == nil || items["Op"] == nil {
		t.Fatalf("type-specific completion=%v", items)
	}
	withComment := strings.Replace(input, "detail.Path", "detail /* receiver */ .Path", 1)
	selector := positionOf(withComment, " .Path", 0)
	items = completionLabels(completionItemsAt(t, path, withComment, selector.Line, selector.Character+2))
	if items["Path"] == nil || items["Op"] == nil {
		t.Fatalf("type-specific completion after a comment=%v", items)
	}
}

func TestSequenceRestEditor(t *testing.T) {
	t.Parallel()
	input := `function count(values: int[]): int {
  return match(values) {
    case [...const prefix, _] => len(prefix)
    case [] => 0
  }
}
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	use := positionOf(input, "prefix)", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, use, ""),
		requestAt("textDocument/rename", 3, uri, use, `"newName":"remaining"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(2) {
		t.Fatalf("definition=%v", messages[2])
	}
	rename, ok := messages[3]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[3])
	}
	items := completionLabels(completionItemsAt(t, path, input, use.Line, use.Character))
	if items["prefix"] == nil || items["prefix"]["detail"] != "const prefix: int[]" {
		t.Fatalf("rest completion=%v", items)
	}
}

func TestResultMatchEditor(t *testing.T) {
	t.Parallel()
	input := `function read(provider: () => Result<int>): int {
  return match(provider()) {
    case ok(const value) => value + 1
    case fail(const failure) => len(failure.Error())
  }
}
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	use := positionOf(input, "failure.Error()", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, use, ""),
		requestAt("textDocument/rename", 3, uri, use, `"newName":"cause"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(3) {
		t.Fatalf("definition=%v", messages[2])
	}
	rename, ok := messages[3]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[3])
	}
	items := completionLabels(completionItemsAt(t, path, input, use.Line, use.Character))
	if items["failure"] == nil || items["failure"]["detail"] != "const failure: error" || items["value"] != nil {
		t.Fatalf("arm-local completion=%v", items)
	}
}

func TestEnumMatchEditor(t *testing.T) {
	t.Parallel()
	input := `enum Status { Waiting, Ready }
function ready(status: Status): boolean {
  return match(status) {
    case Status.Ready => true
    default => false
  }
}
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	use := positionOf(input, "Ready =>", 0)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, use, ""),
		requestAt("textDocument/rename", 3, uri, use, `"newName":"Active"`),
	)
	definition, ok := messages[2]["result"].(map[string]any)
	if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != float64(0) {
		t.Fatalf("definition=%v", messages[2])
	}
	rename, ok := messages[3]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 2 {
		t.Fatalf("rename=%v", messages[3])
	}
	completionInput := strings.Replace(input, "Status.Ready =>", "Status. =>", 1)
	items := completionLabels(completionItemsAt(t, path, completionInput, use.Line, use.Character))
	if items["Ready"] == nil || items["Waiting"] == nil {
		t.Fatalf("enum pattern completion=%v", items)
	}
}

func TestRegexAndMatchEditor(t *testing.T) {
	t.Parallel()
	input := `struct Rule { public pattern: Regex }
function apply(rule: Rule, text: string): boolean {
  return match(rule) {
    case { pattern: const rx } if (rx.test(text)) => rx.test("confirmed")
    default => false
  }
}
`
	path := filepath.Join(t.TempDir(), "entry.km")
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input),
		requestAt("textDocument/definition", 2, uri, positionOf(input, "rx.test(text)", 0), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(input, "pattern: const", 0), ""),
		requestAt("textDocument/hover", 4, uri, positionOf(input, "test(text)", 0), ""),
		requestAt("textDocument/rename", 5, uri, positionOf(input, "rx.test(text)", 0), `"newName":"filter"`),
	)
	for id, want := range map[float64]float64{2: 3, 3: 0} {
		definition, ok := messages[id]["result"].(map[string]any)
		if !ok || definition["uri"] != uri || definition["range"].(map[string]any)["start"].(map[string]any)["line"] != want {
			t.Fatalf("definition=%v", messages[id])
		}
	}
	hover, ok := messages[4]["result"].(map[string]any)
	if !ok || !strings.Contains(hover["contents"].(map[string]any)["value"].(string), "test(text: string): boolean") {
		t.Fatalf("hover=%v", messages[4])
	}
	rename, ok := messages[5]["result"].(map[string]any)
	if !ok || len(rename["changes"].(map[string]any)[uri].([]any)) != 3 {
		t.Fatalf("rename=%v", messages[5])
	}
	guard := positionOf(input, "rx.test(text)", 0)
	items := completionLabels(completionItemsAt(t, path, input, guard.Line, guard.Character))
	if items["rx"] == nil || items["rx"]["detail"] != "const rx: Regex" {
		t.Fatalf("binding completion=%v", items)
	}
	items = completionLabels(completionItemsAt(t, path, input, guard.Line, guard.Character+3))
	if items["test"] == nil || items["replaceAll"] == nil {
		t.Fatalf("Regex member completion=%v", items)
	}
}
