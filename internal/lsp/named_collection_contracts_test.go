package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedCollectionElementEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "entry.km")
	input := "alias Maybe = *int | null; type Queue<T> = distinct GoChannel<T>; type Seq<T> = distinct T[];\nfunction f(q: Queue<Maybe>, xs: Seq<Maybe>): void {\nconst received = <-q;\nfor(const item of xs) {\n\n}\n}\n"
	items := completionLabels(completionItemsAt(t, path, input, 4, 0))
	for _, name := range []string{"received", "item"} {
		if items[name] == nil || !strings.Contains(items[name]["detail"].(string), "*int | null") {
			t.Fatalf("missing nullable completion for %s: %v", name, items[name])
		}
	}
	checkSourceContractDiagnostic(t,
		"alias Maybe = *int | null; type Queue<T> = distinct GoChannel<T>;\nfunction f(q: Queue<Maybe>): *int { const value = <-q; return value; }",
		"cannot use")
}
