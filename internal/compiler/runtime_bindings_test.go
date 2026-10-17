package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeHelperBindingsDiagnosedBeforeEmission(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"return marker":     `function f():int{const __kinmokuseiReturn=5;try{return __kinmokuseiReturn;}finally{}}`,
		"catch marker":      `function f(e:error):void{const __kinmokuseiException=1;try{throw e;}catch(_:error){}}`,
		"task type":         `function value():int{return 1;}function f():int{const __kinmokuseiTask=0;const task=go value();return await task;}`,
		"generic parameter": `function f<__kinmokuseiReturnValue>(v:__kinmokuseiReturnValue):__kinmokuseiReturnValue{try{return v;}finally{}}`,
		"constructor":       `function f():Exception{const NewException=1;return new Exception("message");}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "invalid.km")
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			checked, err := CheckFiles([]string{path})
			if err != nil || len(checked.Diagnostics) == 0 {
				t.Fatalf("check err=%v diagnostics=%v", err, checked.Diagnostics)
			}
			generated, diagnostics, err := EmitGo([]string{path}, "invalid")
			if err != nil || len(diagnostics) == 0 || len(generated) != 0 {
				t.Fatalf("emit err=%v diagnostics=%v bytes=%d", err, diagnostics, len(generated))
			}
			for _, diagnostic := range diagnostics {
				if diagnostic.Span.Path != path || diagnostic.Span.Start.Line < 1 || strings.Contains(diagnostic.Message, "generated Go failed") {
					t.Fatalf("expected source diagnostic: %v", diagnostic)
				}
			}
		})
	}
}

func TestLinkedRuntimeHelperBindings(t *testing.T) {
	t.Parallel()
	for name, sources := range map[string][2]string{
		"dependency local": {`import {value} from "./dependency";function f():int{return value();}`, `export function value():int{const __kinmokuseiReturn=5;try{return __kinmokuseiReturn;}finally{}}`},
		"source alias":     {`import {value as __kinmokuseiReturn} from "./dependency";function f():int{return __kinmokuseiReturn();}`, `export function value():int{return 5;}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			entryPath := filepath.Join(root, "entry.km")
			for file, contents := range map[string]string{"entry.km": sources[0], "dependency.km": sources[1]} {
				if err := os.WriteFile(filepath.Join(root, file), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			checked, err := CheckFiles([]string{entryPath})
			if err != nil || len(checked.Diagnostics) == 0 {
				t.Fatalf("check err=%v diagnostics=%v", err, checked.Diagnostics)
			}
			if got := checked.Diagnostics[0]; !strings.Contains(got.Message, "compiler built-in") || filepath.Dir(got.Span.Path) != root {
				t.Fatalf("expected source diagnostic: %v", got)
			}
		})
	}
}
