package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchConstantDiagnostics(t *testing.T) {
	t.Parallel()
	uri := fileURI(filepath.Join(t.TempDir(), "switch.km"))
	for _, test := range []struct{ name, input, want string }{
		{"string expression", `const word="a"+"b";function f(v:string):void{switch(v){case word{}case "ab"{}}}`, "duplicate value switch case"},
		{"rounding", `function f(v:float32):void{switch(v){case 16777216.0{}case 16777217.0{}}}`, "duplicate value switch case"},
		{"default overflow", `function f():void{switch(9223372036854775808){default{}}}`, "overflows"},
		{"boolean first match", `function f(v:boolean):void{switch(v){case true{}case true{}}}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, test.input), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
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
					t.Fatalf("diagnostics=%s want=%q", got, test.want)
				}
			}
			if !published {
				t.Fatal("missing diagnostics publication")
			}
		})
	}
}
