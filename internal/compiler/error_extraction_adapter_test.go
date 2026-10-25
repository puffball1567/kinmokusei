package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

// Open native T deliberately never crosses an opaque writable boundary. The
// caller's concrete allocator/extractor proves the raw Go storage contract.
func TestErrorExtractionAdapterMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"adapter.km": `export function extract<T>(source:error,allocate:()=>*T,apply:(source:error,target:*T)=>boolean):(T,boolean){const target=allocate();const matched=apply(source,target);return *target,matched;}`,
		"entry.km": `import {extract} from "./adapter";
import go fs from "io/fs";
import go errors from "errors";
export function Extract(source:error):(*fs.PathError,boolean){return extract<*fs.PathError>(source,()=>{let value:*fs.PathError=nil;return &value;},(e,target)=>errors.As(e,target));}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "errorextraction")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "error-extraction-adapter.test", generated,
		`package reference
import("errors";"io/fs")
func Extract(source error)(*fs.PathError,bool){var value *fs.PathError;matched:=errors.As(source,&value);return value,matched}`,
		`package errorextraction_test
import("errors";"fmt";"io/fs";"testing";g "error-extraction-adapter.test";r "error-extraction-adapter.test/reference")
type custom struct{value *fs.PathError;matched bool}
func(custom)Error()string{return "custom"}
func(e custom)As(target any)bool{if p,ok:=target.(**fs.PathError);ok{*p=e.value};return e.matched}
func TestBehavior(t *testing.T){
 p:=&fs.PathError{Op:"open",Path:"safe",Err:errors.New("missing")};var typedNil *fs.PathError
 for _,source:=range []error{nil,errors.New("other"),p,typedNil,fmt.Errorf("wrapped: %w",p),errors.Join(errors.New("first"),p),custom{p,true},custom{p,false},fmt.Errorf("wrapped decline: %w",custom{p,false}),errors.Join(custom{p,false},errors.New("last"))}{
 a,b:=g.Extract(source);c,d:=r.Extract(source);if a!=c||b!=d{t.Fatalf("%T: %p/%v vs %p/%v",source,a,b,c,d)}
 }
 a,matched:=g.Extract(custom{p,false});if a!=p||matched{t.Fatal("declined As mutation lost")}
}`)
}
