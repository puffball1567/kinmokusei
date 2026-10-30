package compiler

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRejectsDeepSyntaxInImportedModule(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "main.km")
	library := filepath.Join(root, "library.km")
	for _, input := range []string{
		"const x=" + strings.Repeat("!", 10000) + "true;",
		"const x=0" + strings.Repeat("+1", 10000) + ";",
		"alias X=int" + strings.Repeat("[]", 10000) + ";",
	} {
		result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
			entry:   `import { value } from "./library"; function main():void{value();}`,
			library: input,
		})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, diagnostic := range result.Diagnostics {
			if strings.Contains(diagnostic.Message, "syntax nesting is too deep") && diagnostic.Span.Path == library {
				found = true
			}
		}
		if !found {
			t.Fatalf("diagnostics=%v", result.Diagnostics)
		}
	}
}
