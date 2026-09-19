package lsp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestUnsafeCollectionEditor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	manifest := "[project]\nname = \"unsafe-editor\"\nversion = \"0.1.0\"\ngo-module = \"unsafe-editor.test\"\ngo-version = \"1.23\"\n[go.interop]\nunsafe = \"allow\"\n"
	if err := os.WriteFile(filepath.Join(root, "kinmokusei.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	input := "import go u from \"unsafe\";\nconstraint S=~int[];\nfunction f<T extends S>(v:T):void{\nconst pointer=u.SliceData(v);\nconst values=u.Slice(pointer,2.0);\n\n}\n"
	items := completionLabels(completionItemsAt(t, path, input, 5, 0))
	for name, want := range map[string]string{"pointer": "*int", "values": "int[]"} {
		if items[name] == nil || !strings.Contains(items[name]["detail"].(string), name+": "+want) {
			t.Errorf("completion %s=%v want=%s", name, items[name], want)
		}
	}
	for _, test := range []struct{ source, want string }{
		{input, ""},
		{strings.ReplaceAll(input, "2.0", "-1.0"), "cannot be negative"},
	} {
		var output bytes.Buffer
		if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(fileURI(path), test.source), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
			t.Fatal(err)
		}
		published := false
		for _, message := range decodeMessages(t, output.String()) {
			if message["method"] != "textDocument/publishDiagnostics" {
				continue
			}
			published = true
			var diagnostics []string
			for _, raw := range message["params"].(map[string]any)["diagnostics"].([]any) {
				diagnostics = append(diagnostics, raw.(map[string]any)["message"].(string))
			}
			got := strings.Join(diagnostics, "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%s", got, test.want)
			}
		}
		if !published {
			t.Fatal("missing diagnostics publication")
		}
	}
}
