package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/codegen"
)

func TestSourceExportModuleVisibility(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, library, entry, want string }{
		{"legacy", `function value():int{return 1}`, `import { value } from "./library"; function Run():int{return value()}`, ""},
		{"inline", `export function value():int{return hidden()} function hidden():int{return 3}`, `import { value } from "./library"; function Run():int{return value()}`, ""},
		{"list before declaration", `export { value }; const value=3;`, `import { value } from "./library"; function Run():int{return value}`, ""},
		{"private", `export const value=1; const hidden=2;`, `import { hidden } from "./library";`, `does not export "hidden"`},
		{"empty", `export {}; function value():int{return 1}`, `import { value } from "./library";`, `does not export "value"`},
		{"C ABI retains legacy", `export c("c_native") function native():int32{return 1} const value=2;`, `import { value } from "./library";`, ""},
		{"C ABI and source", `export c("native") function native():int32{return 1} export {};`, `import { native } from "./library";`, `does not export "native"`},
		{"missing", `export {};`, `import { missing } from "./library";`, `does not declare "missing"`},
		{"duplicate import", `export const value=1;`, `import { value, value } from "./library";`, `duplicate imported name "value"`},
		{"imported binding", `const value=1;`, `import { value } from "./library"; export { value };`, `not a local top-level declaration`},
		{"private type", `export {}; class Box {}`, `import { Box } from "./library";`, `does not export "Box"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "entry.km")
			result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: test.entry, filepath.Join(root, "library.km"): test.library})
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				if len(result.Diagnostics) != 0 {
					t.Fatal(result.Diagnostics)
				}
				return
			}
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Message, test.want) {
					return
				}
			}
			t.Fatalf("diagnostics=%v want=%s", result.Diagnostics, test.want)
		})
	}
}

func TestSourceExportsKeepGoEmission(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "library.km")
	input := `function Value():int{return helper()} function helper():int{return 42}`
	var outputs [][]byte
	for _, prefix := range []string{"", "export "} {
		result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: prefix + input})
		if err != nil || len(result.Diagnostics) != 0 {
			t.Fatalf("%v %v", err, result.Diagnostics)
		}
		output, err := codegen.Generate(result.Program, "library")
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, output)
	}
	if !bytes.Equal(outputs[0], outputs[1]) {
		t.Fatalf("export changed Go emission:\n%s\n%s", outputs[0], outputs[1])
	}
}

func TestSourceExportDeclarationKindsAndLinkedNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "main.km")
	library := filepath.Join(root, "library.km")
	for _, inline := range []bool{false, true} {
		declarations := []string{
			`function identity<T>(v:T):T{return v}`,
			`class Box<T>{constructor(public value:T){}}`,
			`struct Point{public x:int;}`,
			`interface Item{}`,
			`type Count = distinct int;`,
			`alias Number = int;`,
			`enum State{First,Second}`,
			`constraint Scalar = int | string;`,
			`const increment=(n:int):int=>n+1;`,
			`let current:int=1;`,
		}
		input := strings.Join(declarations, "\n") + "\nexport { identity, Box, Point, Item, Count, Number, State, Scalar, increment, current }"
		if inline {
			input = "export " + strings.Join(declarations, "\nexport ")
		}
		result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
			entry: `import { identity, Box, Point, Item, Count, Number, State, Scalar, increment, current } from "./library";
function Run():int { const box=new Box<int>(identity(2)); const point:Point=Point{x:box.value}; current=increment(current); return point.x+current; }
function constrained<T extends Scalar>(v:T):T{return v;}`,
			library: input,
		})
		if err != nil || len(result.Diagnostics) != 0 {
			t.Fatalf("inline=%v %v %v", inline, err, result.Diagnostics)
		}
		for _, exported := range result.Program.Exports {
			for _, name := range exported.Names {
				if name.ResolvedDeclaration.Path != library {
					t.Fatalf("unresolved export %+v", name)
				}
			}
		}
	}
	// Both private and public duplicate spellings must keep their source identity
	// regardless of the lexical order in which the linker processes modules.
	for _, filename := range []string{"aaa.km", "zzz.km"} {
		dependency := filepath.Join(root, filename)
		result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
			entry:      `import { value } from "./library"; function Run():int{return value()}`,
			library:    `import { duplicate } from "./` + strings.TrimSuffix(filename, ".km") + `"; export { value }; function value():int{return duplicate()}`,
			dependency: `export { duplicate }; function duplicate():int{return value()} function value():int{return 3}`,
		})
		if err != nil || len(result.Diagnostics) != 0 {
			t.Fatalf("%v %v", err, result.Diagnostics)
		}
		for _, exported := range result.Program.Exports {
			for _, name := range exported.Names {
				if name.ResolvedDeclaration.Path != exported.Span.Path {
					t.Fatalf("unresolved linked export: %+v", name)
				}
			}
		}
	}
}

func TestSourceExportsArePerFileWithMultipleRoots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, explicit, legacy := filepath.Join(root, "main.km"), filepath.Join(root, "explicit.km"), filepath.Join(root, "legacy.km")
	result, err := CheckFilesWithOverlay([]string{entry, explicit, legacy}, map[string]string{
		entry:    `import { value } from "./legacy"; function Run():int{return value()}`,
		explicit: `export {}; function hidden():int{return 1}`,
		legacy:   `function value():int{return 2}`,
	})
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("%v %v", err, result.Diagnostics)
	}
}

func TestSourceExportDiagnosticsKeepSourceSpellingAndSpan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry, library := filepath.Join(root, "main.km"), filepath.Join(root, "library.km")
	input := `import { other } from "./other"; export { value, value }; function value():int{return other()}`
	result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
		entry:                           `import { value } from "./library"; function hidden():int{return 0}`,
		library:                         input,
		filepath.Join(root, "other.km"): `function other():int{return value()} function value():int{return 1}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.Message == `duplicate exported name "value"` {
			found = true
			if d.Span.Path != library || d.Span.Start.Offset != strings.Index(input, "value, value")+len("value, ") {
				t.Fatal(d)
			}
		}
	}
	if !found {
		t.Fatal(result.Diagnostics)
	}
	input = `import { hidden } from "./library";`
	result, err = CheckFilesWithOverlay([]string{entry}, map[string]string{entry: input, library: `export {}; function hidden():int{return 1}`})
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, d := range result.Diagnostics {
		if d.Message == `module "./library" does not export "hidden"` {
			found = true
			if d.Span.Path != entry || input[d.Span.Start.Offset:d.Span.End.Offset] != "hidden" {
				t.Fatal(d)
			}
		}
	}
	if !found {
		t.Fatal(result.Diagnostics)
	}
}

func TestSourceExportsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"first.km":  `export { first }; function helper(n:int):int{return n+1} function first(n:int):int{return helper(n)} export class Box { public value:int; constructor(value:int){this.value=value;} }`,
		"second.km": `function helper(n:int):int{return n*2} function second(n:int):int{return helper(n)}`,
		"main.km":   `import { first, Box } from "./first"; import { second } from "./second"; export function Run(n:int):int{const box=new Box(first(n)); return second(box.value)}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := CheckFiles([]string{filepath.Join(root, "main.km")})
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("%v %v", err, result.Diagnostics)
	}
	for _, declaration := range result.Program.Declarations {
		name, _ := ast.DeclarationBinding(declaration)
		if strings.HasSuffix(name, "_helper") && declaration.GetSpan().Path == filepath.Join(root, "first.km") && ast.SourceExported(result.Program, declaration) {
			t.Fatal("linked private helper became exported")
		}
	}
	generated, err := codegen.Generate(result.Program, "sourceexports")
	if err != nil {
		t.Fatal(err)
	}
	runGeneratedGoDifferentialTest(t, root, "source-exports.test", generated,
		`package reference; func Run(n int) int { return (n+1)*2 }`,
		`package sourceexports
import("testing"; reference "source-exports.test/reference")
func TestBehavior(t *testing.T){for _,n:=range []int{-100,-1,0,1,100}{if got,want:=Run(n),reference.Run(n);got!=want{t.Errorf("Run(%d)=%d want %d",n,got,want)}}}`)
}
