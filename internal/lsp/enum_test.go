package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnumNavigationRenameSymbolsAndCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enum.otm")
	uri := fileURI(path)
	text := `enum Status: int16 { Pending, Running = 4, Complete, }
function use(value: Status): Status {
  if (value === Status.Pending) { return Status.Running; }
  return Status.Complete;
}`
	symbolsRequest := fmt.Sprintf(`{"jsonrpc":"2.0","id":8,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":%q}}}`, uri)
	messages := serveMessages(t,
		openDocument(uri, text),
		requestAt("textDocument/definition", 2, uri, positionOf(text, "Status", 1), ""),
		requestAt("textDocument/definition", 3, uri, positionOf(text, "Pending", 1), ""),
		requestAt("textDocument/references", 4, uri, positionOf(text, "Running", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 5, uri, positionOf(text, "Complete", 1), `"newName":"Finished"`),
		requestAt("textDocument/references", 6, uri, positionOf(text, "Status", 0), `"context":{"includeDeclaration":true}`),
		requestAt("textDocument/rename", 7, uri, positionOf(text, "Status", 2), `"newName":"State"`),
		symbolsRequest,
	)
	typeDefinition := messages[2]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if typeDefinition["line"] != float64(0) || typeDefinition["character"] != float64(5) {
		t.Fatalf("enum type definition = %#v", typeDefinition)
	}
	memberDefinition := messages[3]["result"].(map[string]any)["range"].(map[string]any)["start"].(map[string]any)
	if memberDefinition["line"] != float64(0) || memberDefinition["character"] != float64(21) {
		t.Fatalf("enum member definition = %#v", memberDefinition)
	}
	if got := len(messages[4]["result"].([]any)); got != 2 {
		t.Fatalf("enum member references = %d, want declaration and use", got)
	}
	memberChanges := messages[5]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(memberChanges[uri].([]any)); got != 2 {
		t.Fatalf("enum member rename edits = %d, want declaration and use", got)
	}
	if got := len(messages[6]["result"].([]any)); got != 6 {
		t.Fatalf("enum type references = %d, want declaration, annotation and four selectors", got)
	}
	typeChanges := messages[7]["result"].(map[string]any)["changes"].(map[string]any)
	if got := len(typeChanges[uri].([]any)); got != 6 {
		t.Fatalf("enum type rename edits = %d, want 6", got)
	}
	symbols := messages[8]["result"].([]any)
	if len(symbols) != 2 {
		t.Fatalf("enum symbols = %#v", symbols)
	}
	status := symbols[0].(map[string]any)
	children := status["children"].([]any)
	if status["detail"] != "enum Status: int16" || len(children) != 3 {
		t.Fatalf("enum symbol = %#v", status)
	}
	if got := children[1].(map[string]any)["detail"]; got != "Status.Running = 4" {
		t.Fatalf("enum member symbol detail = %v", got)
	}

	completionText := strings.Replace(text, "return Status.Complete;", "return Status.;", 1)
	line := strings.Split(completionText, "\n")[3]
	items := completionLabels(completionItemsAt(t, path, completionText, 3, strings.Index(line, "Status.")+len("Status.")))
	for _, name := range []string{"Pending", "Running", "Complete"} {
		if items[name] == nil || items[name]["detail"] != "Status."+name {
			t.Errorf("enum completion %q = %#v", name, items[name])
		}
	}

	lexical := completionLabels(completionItemsAt(t, path, text, 1, 0))
	if lexical["enum"] == nil || lexical["Status"] == nil || lexical["Status"]["detail"] != "enum Status: int16" {
		t.Fatalf("enum lexical completion = %#v / %#v", lexical["enum"], lexical["Status"])
	}
}

func TestEnumReceiverCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enum_method.otm")
	text := `enum Status: int8 { Pending, Running, Complete }
public function active(this: Status): boolean { return this === Status.Running; }
public function advance(this: *Status, steps: int8): Status { *this = Status(int8(*this) + steps); return *this; }
function use(value: Status): boolean {
  value.advance(1);
  return value.active();
}`
	completionText := strings.Replace(text, "return value.active();", "return value.;", 1)
	line := strings.Split(completionText, "\n")[5]
	items := completionLabels(completionItemsAt(t, path, completionText, 5, strings.Index(line, "value.")+len("value.")))
	for name, detail := range map[string]string{
		"active":  "public function active(): boolean",
		"advance": "public function advance(steps: int8): Status",
	} {
		if items[name] == nil || items[name]["detail"] != detail {
			t.Errorf("enum method completion %q = %#v, want %q", name, items[name], detail)
		}
	}
	if items["Pending"] != nil {
		t.Fatalf("enum instance completion leaked static member: %#v", items["Pending"])
	}
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, text, positionOf(text, "1);", 0)))
	if label != "value.advance(steps: int8): Status" || active != 0 || len(parameters) != 1 {
		t.Fatalf("enum method signature = %q active=%v parameters=%#v", label, active, parameters)
	}
}
