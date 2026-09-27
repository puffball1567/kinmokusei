package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssignmentContractsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"types.km": `export alias Maybe = *int | null;
export type Items<T> = distinct T[];
export type Callback = distinct () => Maybe;
export interface Reader<T> { function read(): T; }
export class Holder<T> implements Reader<T> {
 constructor(public value: T) {}
 public function read(): T { return this.value; }
}
export class Child<T> extends Holder<Items<T>> {
 constructor(values: Items<T>) { super(values); }
}`,
		"bridge.km": `export {Maybe, Items as List, Reader, Child as Box, Callback} from "./types";`,
		"entry.km": `import {Maybe, List, Reader, Box, Callback} from "./bridge";
export function Run(n: int): int[] {
 let storedValue = n;
 const original: Maybe[] = [null, &storedValue];
 const named: List<Maybe> = original;
 const holder = new Box<Maybe>(named);
 const reader: Reader<List<Maybe>> = holder;
 const stored = reader.read();
 stored[0] = &storedValue;
 let first = 0; const p = original[0]; if(p !== null) { first = *p; }
 const callback: Callback = (): Maybe => { return &storedValue; };
 const unnamed: () => Maybe = callback;
 const value = unnamed();
 let second = 0; if(value !== null) { second = *value; }
 storedValue++;
 let changed = 0; const q = named[1]; if(q !== null) { changed = *q; }
 return [first, second, changed, len(stored)];
}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "contracts")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type items[T any] []T
type reader[T any] interface{Read() T}
type holder[T any] struct{value T}
func(h *holder[T])Read()T{return h.value}
type child[T any] struct{holder[items[T]]}
type callback func()*int
func Run(n int)[]int{number:=n;original:=[]*int{nil,&number};var named items[*int]=original;box:=&child[*int]{holder[items[*int]]{named}};var r reader[items[*int]]=box;stored:=r.Read();stored[0]=&number;first:=0;if p:=original[0];p!=nil{first=*p};var cb callback=func()*int{return &number};var unnamed func()*int=cb;value:=unnamed();second:=0;if value!=nil{second=*value};number++;changed:=0;if q:=named[1];q!=nil{changed=*q};return []int{first,second,changed,len(stored)}}`
	tests := `package contracts_test
import("reflect";"testing";g "assignment-contracts.test";r "assignment-contracts.test/reference")
func TestRun(t *testing.T){for _,n:=range []int{-7,0,42}{if got,want:=g.Run(n),r.Run(n);!reflect.DeepEqual(got,want){t.Fatal(got,want)}}}`
	runGeneratedGoDifferentialTest(t, root, "assignment-contracts.test", generated, reference, tests)
}

func TestLinkedAssignmentContractDiagnostics(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`function f(value: List<Maybe>): List<*int> { return value; }`,
		`function f(value: List<Maybe>): void { accept(value); }`,
		`function f(value: Holder<List<Maybe>>): Reader<List<*int>> { return value; }`,
	} {
		root := t.TempDir()
		entry, library, bridge := filepath.Join(root, "entry.km"), filepath.Join(root, "lib.km"), filepath.Join(root, "bridge.km")
		input := `import {List, Maybe, Holder, Reader, accept} from "./bridge";` + body
		files := map[string]string{
			entry: input,
			library: `export alias Maybe = *int | null; export type Items<T> = distinct T[];
export interface Reader<T> { function read(): T; }
export class Holder<T> implements Reader<T> { constructor(public value:T){} public function read():T{return this.value;} }
export function accept(value:Items<*int>):void{}`,
			bridge: `export {Items as List, Maybe, Holder, Reader, accept} from "./lib";`,
		}
		result, err := CheckFilesWithOverlay([]string{entry}, files)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.Diagnostics {
			if d.Span.Path == entry && strings.Contains(d.Message, "cannot use") && input[d.Span.Start.Offset:d.Span.End.Offset] == "value" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing source assignment diagnostic for %s: %v", body, result.Diagnostics)
		}
		files[library] = strings.ReplaceAll(files[library], "*int | null", "*int")
		matched, err := CheckFilesWithOverlay([]string{entry}, files)
		if err != nil || len(matched.Diagnostics) != 0 {
			t.Fatalf("matching contracts rejected: %v %v", err, matched.Diagnostics)
		}
	}
}
