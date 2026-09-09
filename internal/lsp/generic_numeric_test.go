package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericNumericSignatureAndCompletion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "generic_numeric.km")
	input := "function pick<T>(a:T,b:T):T{return b;}\nfunction f(v:float32):void{\nlet narrow=pick(1.25,v);\nlet promoted=pick(1,2.5);\n\n}\n"
	items := completionLabels(completionItemsAt(t, path, input, 4, 0))
	for name, want := range map[string]string{"narrow": "float32", "promoted": "float"} {
		if items[name] == nil || !strings.Contains(items[name]["detail"].(string), name+": "+want) {
			t.Errorf("completion %s=%v want=%s", name, items[name], want)
		}
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "1.25,v", 0)))
	if label != "pick(a: float32, b: float32): float32" {
		t.Fatalf("signature=%q", label)
	}
}
