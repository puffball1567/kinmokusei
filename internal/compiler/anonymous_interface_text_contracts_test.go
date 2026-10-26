package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnonymousInterfaceHiddenTextStorageRejected(t *testing.T) {
	t.Parallel()
	for _, declaration := range []string{
		`export struct Row{public texts:string[];public function read():int{return 1;}}`,
		`export struct Row<E>{public texts:E[];public function read():int{return 1;}}`,
		`export struct Row{public texts:string[];public pointer function read():int{return 1;}}`,
		`export type Row=distinct string[];public function read(this:Row):int{return len(this);}`,
	} {
		t.Run(declaration, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "entry.km")
			for name, input := range map[string]string{
				"models.km": declaration,
				"entry.km":  `import {Row} from "./models";import go reflect from "reflect";function inspect(v:interface{read():int;}):void{reflect.ValueOf(v);}`,
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			generated, diagnostics, err := EmitGo([]string{entry}, "contracts")
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range diagnostics {
				if d.Span.Path == entry && strings.Contains(d.Message, "cannot use") && strings.Contains(d.Message, " as any:") && strings.Contains(d.Message, "verified UTF-8 string") && strings.Contains(d.Message, "bstring/raw input storage") {
					if len(generated) != 0 {
						t.Fatal("invalid source emitted Go")
					}
					return
				}
			}
			t.Fatalf("missing hidden-storage diagnostic: %v", diagnostics)
		})
	}
}
