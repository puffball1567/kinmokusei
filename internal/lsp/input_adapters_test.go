package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInputAdapterStandardModuleEditorSupport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.km")
	input := `import {decodeInput,bindInput} from "kinmokusei/input";
function read():Result<bstring>{return ok(b"safe");}
function construct(raw:bstring):Result<string>{return string(raw);}
function use():Result<string>{return decodeInput(read,construct);}`
	items := completionLabels(completionItemsAt(t, path, input, 3, 0))
	if items["decodeInput"] == nil || items["bindInput"] == nil {
		t.Fatalf("missing imported adapter completions: %v", items)
	}
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "read,construct", 0)))
	if !strings.Contains(label, "decodeInput") || !strings.Contains(label, "Result<") || active != 0 || len(parameters) != 2 {
		t.Fatalf("signature: %q %v %v", label, active, parameters)
	}
	uri := fileURI(path)
	messages := serveMessages(t, openDocument(uri, input), requestAt("textDocument/hover", 2, uri, positionOf(input, "decodeInput(read", 0), ""))
	result, ok := messages[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("hover: %v", messages[2])
	}
	contents := result["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(contents, "decodeInput") || !strings.Contains(contents, "Result<") {
		t.Fatalf("hover: %q", contents)
	}
}
