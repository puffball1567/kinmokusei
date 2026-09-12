package compiler

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstantiatedGoFunctionValueDiagnostics(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go slices from "slices";const value=slices.Index;`,
		`import go { Index } from "slices";const value=Index;`,
		`import go slices from "slices";function run():void{const value=slices.Index;}`,
		`import go slices from "slices";const value=()=>slices.Index;`,
		`import go slices from "slices";const value=[slices.Index];`,
		`import go slices from "slices";function apply(f:()=>void):void{}function run():void{apply(slices.Index);}`,
	} {
		path := filepath.Join(t.TempDir(), "invalid.km")
		checked, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range checked.Diagnostics {
			found = found || strings.Contains(d.Message, "generic Go functions must be called")
		}
		if !found {
			t.Fatalf("%s: %v", input, checked.Diagnostics)
		}
	}
	for _, input := range []string{
		`import go slices from "slices";const value=slices.Index([1,2],2);`,
		`import go { Index } from "slices";const value=Index<int[],int>([1,2],2);`,
		`import go strings from "strings";const value=strings.TrimSpace;`,
	} {
		_, accepted, err := compilePipelineProperty(input)
		if err != nil || !accepted {
			t.Fatalf("%s: accepted=%v err=%v", input, accepted, err)
		}
	}
}
