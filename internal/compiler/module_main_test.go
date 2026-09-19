package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedMainIsModuleScoped(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, library, use string }{
		{"constant", `export const main=42;`, `return main;`},
		{"variable", `export let main=41;`, `main++;return main;`},
		{"inferred arrow", `export const main=()=>42;`, `return main();`},
		{"mutable arrow", `export let main=(n:int):int=>n+1;`, `return main(41);`},
		{"function", `export function main(n:int):int{return n+1;}`, `return main(41);`},
		{"generic function", `export function main<T>(value:T):T{return value;}`, `return main(42);`},
		{"recursive arrow", `export const main=(n:int):int=>{if(n===0){return 0;}return main(n-1)+1;};`, `return main(42);`},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "entry.km")
			files := map[string]string{
				entry:                             `import {main} from "./library";export function Answer():int{` + test.use + `}`,
				filepath.Join(root, "library.km"): test.library,
			}
			result, err := CheckFilesWithOverlay([]string{entry}, files)
			if err != nil || len(result.Diagnostics) != 0 {
				t.Fatalf("err=%v diagnostics=%v", err, result.Diagnostics)
			}
			for name, source := range files {
				if err := os.WriteFile(name, []byte(source), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			generated, diagnostics, err := EmitGo([]string{entry}, "modulemain")
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
			}
			if strings.Contains(string(generated), "func main(") || strings.Contains(string(generated), "var main ") || strings.Contains(string(generated), "const main ") {
				t.Fatalf("library main escaped module scope:\n%s", generated)
			}
			reference := "package reference\nfunc Answer()int{return 42}\n"
			comparison := `package modulemain_test
import("testing";g "module-main.test";r "module-main.test/reference")
func TestAnswer(t *testing.T){if got,want:=g.Answer(),r.Answer();got!=want{t.Fatalf("got %d want %d",got,want)}}
`
			runGeneratedGoDifferentialTest(t, root, "module-main.test", generated, reference, comparison)
		})
	}
}

func TestImportedMainEntryAndReexportBoundaries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	library := filepath.Join(root, "library.km")
	bridge := filepath.Join(root, "bridge.km")
	files := map[string]string{
		entry:   `import {run} from "./bridge";const main=()=>{run();};`,
		library: `export const main=()=>42;`,
		bridge:  `export {main as run} from "./library";`,
	}
	result, err := CheckFilesWithOverlay([]string{entry}, files)
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("entry with reexport: %v %v", err, result.Diagnostics)
	}
	for name, source := range files {
		if err := os.WriteFile(name, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "main")
	if err != nil || len(diagnostics) != 0 || strings.Count(string(generated), "func main(") != 1 {
		t.Fatalf("entry generation: %v %v\n%s", err, diagnostics, generated)
	}
	// Imports alone must not supply an executable's entry point.
	if err := os.WriteFile(entry, []byte(`import {run} from "./bridge";function use():int{return run();}`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err = EmitGo([]string{entry}, "main")
	if err != nil || len(diagnostics) != 0 || strings.Contains(string(generated), "func main(") {
		t.Fatalf("dependency provided an entry: %v %v\n%s", err, diagnostics, generated)
	}
	// Explicit input roots retain the existing entry-name checks.
	result, err = CheckFilesWithOverlay([]string{library}, files)
	if err != nil || len(result.Diagnostics) == 0 {
		t.Fatalf("invalid explicit root main accepted: %v %v", err, result.Diagnostics)
	}
	files[entry] = `import {main} from "./library";const main=()=>{};`
	result, err = CheckFilesWithOverlay([]string{entry}, files)
	if err != nil || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "conflicts with a declaration") {
		t.Fatalf("same-scope duplicate main accepted: %v %v", err, result.Diagnostics)
	}
}
