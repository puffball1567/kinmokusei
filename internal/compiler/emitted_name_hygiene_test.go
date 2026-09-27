package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportsAvoidEmittedNameCapture(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"lib.km": `import go len from "strings";
export function copy(n:int):int{return n+10;}
export function func(n:int):int{return n+20;}
export function text():string{return len.TrimSpace(" a ");}`,
		"entry.km": `import {copy as read,func as run,text} from "./lib";import go textutil from "strings";
export function Run():string{const copy_=(n:int):int=>n+100;const func_=(n:int):int=>n+200;let len_=3;return text()+textutil.TrimSpace(" b ")+string(int32(read(1)+run(2)+copy_(3)+func_(4)+len_));}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "hygiene")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "emitted-name-hygiene.test", generated,
		`package reference;func Run()string{return "ab"+string(rune(11+22+103+204+3))}`,
		`package hygiene_test;import("testing";g "emitted-name-hygiene.test";r "emitted-name-hygiene.test/reference");func TestRun(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatal(got,want)}}`)
}

func TestEscapedLocalsDoNotCaptureBuiltins(t *testing.T) {
	t.Parallel()
	entry := filepath.Join(t.TempDir(), "entry.km")
	input := `function f():int{const len_=3;const copy_=4;const values:int[]=[0];return len(values)+copy(values,[1])+len_+copy_;}`
	result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: input})
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, result.Diagnostics)
	}
}

func TestRootEmittedNameCaptureDiagnostics(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function copy(n:int):int{return n+1;}function f():int{const copy_=(n:int):int=>n+100;return copy(1);}`,
		`function func(n:int):int{return n+1;}function f(func_:int):int{return func(1);}`,
		`class func{}function f<func_>():func{return new func();}`,
		`function f():int{let copy=1;{let copy_=2;return copy;}}`,
		`function f():int{let copy_=1;{let copy=2;return copy_;}}`,
		`function pair():(int,int){return 1,2;}function f():int{let copy=0;let other=0;{let copy_=2;[copy,other]=pair();}return copy;}`,
	} {
		entry := filepath.Join(t.TempDir(), "entry.km")
		result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: input})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.Diagnostics {
			if d.Span.Path == entry && strings.Contains(d.Message, "would be captured") {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing source diagnostic: %v", result.Diagnostics)
		}
	}
}
