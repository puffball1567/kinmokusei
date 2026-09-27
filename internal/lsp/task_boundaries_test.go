package lsp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskBoundaryDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		body  string
		valid bool
	}{
		{`const task=go value();const n=load()?;const joined=await task;return ok(n+joined);`, false},
		{`const task=go value();const callback=():int=>{return 2;};const joined=await task;const n=load()?;return ok(n+joined+callback());`, true},
		{`while(true){const task=go value();if(value()>0){break;}const n=await task;}return ok(0);`, false},
		{`const task=go value();while(true){break;}return ok(await task);`, true},
		{`const task=go value();let n=0;if(value()>0){goto done;}n=await task;done:return ok(n);`, false},
		{`const task=go value();goto done;done:return ok(await task);`, true},
	} {
		input := "function value():int{return 7;}function load():Result<int>{return ok(2);}\nfunction f():Result<int>{" + test.body + "}"
		uri := fileURI(filepath.Join(t.TempDir(), "entry.km"))
		var output bytes.Buffer
		if err := Serve(strings.NewReader(framed(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`, openDocument(uri, input), `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`, `{"jsonrpc":"2.0","method":"exit"}`)), &output); err != nil {
			t.Fatal(err)
		}
		published := false
		for _, message := range decodeMessages(t, output.String()) {
			if message["method"] != "textDocument/publishDiagnostics" {
				continue
			}
			published = true
			diagnostics := message["params"].(map[string]any)["diagnostics"].([]any)
			if test.valid {
				if len(diagnostics) != 0 {
					t.Fatalf("valid Task flow rejected: %v", diagnostics)
				}
				continue
			}
			if len(diagnostics) != 1 {
				t.Fatalf("expected one Task diagnostic, got %v", diagnostics)
			}
			diagnostic := diagnostics[0].(map[string]any)
			if !strings.Contains(diagnostic["message"].(string), `Task "task"`) {
				t.Fatalf("wrong diagnostic: %v", diagnostic)
			}
			start := diagnostic["range"].(map[string]any)["start"].(map[string]any)
			if start["line"] != float64(1) || start["character"] != float64(strings.Index(strings.Split(input, "\n")[1], "task=")) {
				t.Fatalf("wrong source range: %v", diagnostic)
			}
		}
		if !published {
			t.Fatal("missing diagnostics publication")
		}
	}
}
