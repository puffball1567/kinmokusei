package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNumericLiteralInferredCompletion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "numbers.km")
	input := "function f():void{\nlet count=0xFF;\nlet fraction=1e-2;\nlet signal=1+2i;\n\n}\n"
	items := completionLabels(completionItemsAt(t, path, input, 4, 0))
	for name, want := range map[string]string{"count": "int", "fraction": "float", "signal": "complex128"} {
		if items[name] == nil || !strings.Contains(items[name]["detail"].(string), name+": "+want) {
			t.Errorf("completion %s=%v want type=%s", name, items[name], want)
		}
	}
}

func TestNumericConstantContextCompletion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "contexts.km")
	input := "const offset=2.0;\nfunction f(a:int[]):void{\nlet value=a[offset];\nlet storage=makeSlice<int>(offset,4.0);\n\n}\n"
	items := completionLabels(completionItemsAt(t, path, input, 4, 0))
	for name, want := range map[string]string{"value": "int", "storage": "int[]"} {
		if items[name] == nil || !strings.Contains(items[name]["detail"].(string), name+": "+want) {
			t.Errorf("completion %s=%v want type=%s", name, items[name], want)
		}
	}
}
