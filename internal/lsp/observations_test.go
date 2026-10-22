package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestObservationCompletionAndSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inspect.km")
	input := "function f<T>(value:T):bstring{return inspectKind(value);}\n"
	items := completionLabels(completionItemsAt(t, path, input, 1, 0))
	for _, definition := range ast.ObservationBuiltins() {
		if items[definition.Name] == nil {
			t.Fatalf("missing completion %s", definition.Name)
		}
	}
	if items["jsonSnapshot"] == nil {
		t.Fatal("missing snapshot completion")
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "value);", 0)))
	if !strings.HasPrefix(label, "inspectKind(value:") || !strings.HasSuffix(label, "): bstring") {
		t.Fatalf("signature=%q", label)
	}
	for _, input := range []string{
		`function f<T>(v:T):Result<byte[]>{return jsonSnapshot(v,"","  ");}`,
		`function f<T>(v:T):Result<byte[]>{return jsonSnapshot(v, `,
	} {
		at := positionOf(input, `"  "`, 0)
		if !strings.Contains(input, `"  "`) {
			at = position{Line: 0, Character: len(input)}
		}
		label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, at))
		if !strings.HasPrefix(label, "jsonSnapshot(value:") || !strings.HasSuffix(label, "): Result<byte[]>") || !strings.Contains(label, "prefix") || !strings.Contains(label, "indent") {
			t.Fatalf("snapshot signature=%q", label)
		}
	}
}
