package lsp

import (
	"path/filepath"
	"testing"
)

func TestRecursiveLoopArrowCompletionScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, at, want string }{
		{`const f=():string=>"outer";function run():void{for(const f=(f:int):int=>f;false;){}}`, "=>f;", "f: int"},
		{`const f=():string=>"outer";function run():void{for(const f=():int=>{const f=1;return f;};false;){}}`, "return f;", "const f: int"},
		{`const f=():string=>"outer";function run():void{for(const f=f();false;){}}`, "=f();", "const f: () => string"},
		{`const f=():string=>"outer";function run():void{for(const f=():int=>f();false;){}f();}`, "}f();", "const f: () => string"},
	} {
		uri := fileURI(filepath.Join(t.TempDir(), "scope.km"))
		at := positionOf(test.input, test.at, 0)
		// Point at the identifier rather than its preceding punctuation/keyword.
		for index, char := range test.at {
			if char == 'f' {
				at.Character += index
				break
			}
		}
		messages := serveMessages(t, openDocument(uri, test.input), requestAt("textDocument/completion", 2, uri, at, ""))
		found := ""
		for _, raw := range messages[2]["result"].([]any) {
			item := raw.(map[string]any)
			if item["label"] == "f" {
				found = item["detail"].(string)
			}
		}
		if found != test.want {
			t.Fatalf("%s: got %q want %q", test.input, found, test.want)
		}
	}
}
