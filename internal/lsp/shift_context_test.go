package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestShiftContextEditor(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shift.km")
	input := "function pick<T>(a:T,b:T):T{return a;}\nfunction f(n:uint,v:byte):void{\nconst narrow=pick(1.0<<n,v);\nconst values=make[int[]](1.0<<n);\n\n}\n"
	items := completionLabels(completionItemsAt(t, path, input, 4, 0))
	for name, want := range map[string]string{"narrow": "byte", "values": "int[]"} {
		if items[name] == nil || !strings.Contains(items[name]["detail"].(string), name+": "+want) {
			t.Errorf("completion %s=%v want=%s", name, items[name], want)
		}
	}
	label, _, _ := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "1.0<<n,v", 0)))
	if label != "pick(a: byte, b: byte): byte" {
		t.Fatalf("signature=%q", label)
	}
	for _, test := range []struct{ source, want string }{
		{input, ""},
		{`function f(n:uint):byte{return 300<<n;}`, "overflows"},
		{`function f(n:uint):float{return 1<<n;}`, "must be integer"},
	} {
		published := false
		var output bytes.Buffer
		if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(fileURI(path), test.source), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
			t.Fatal(err)
		}
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
