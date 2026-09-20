package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnonymousInterfaceSourceEmitsGoInterface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := `
class Reader { public function read(offset:int):string{return "ok";} }
function read(value: interface { read(offset:int):string; }):string{return value.read(0);}
function use():string{const reader=new Reader();return read(reader);}
`
	path := filepath.Join(root, "main.km")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "anonymousinterface")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	if !strings.Contains(text, "interface {") || !strings.Contains(text, "Read(offset int) string") {
		t.Fatalf("anonymous interface was not emitted:\n%s", text)
	}
}
