package compiler

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDecoratorsCannotBeSilentlyDiscarded(t *testing.T) {
	for _, input := range []string{`@D export class C{}`, `export class C{@D public value:int=1;}`, `export class C{constructor(@D value:int){}}`, `export class C{public function f(@D value:int):void{}}`, `export class C{public function f():void{const a=(@D x:int)=>x;}}`} {
		t.Run(input, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "main.km")
			dependency := filepath.Join(root, "lib.km")
			checked, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: `import { C } from "./lib"; function main():void{}`, dependency: input})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range checked.Diagnostics {
				if strings.Contains(d.Message, "decorator execution and metadata generation are not implemented yet") && d.Span.Path == dependency {
					return
				}
			}
			t.Fatalf("missing dependency decorator diagnostic: %v", checked.Diagnostics)
		})
	}
}
