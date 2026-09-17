package compiler

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/codegen"
)

func TestNamedReexports(t *testing.T) {
	t.Parallel()
	for index, test := range []struct{ library, barrel, entry, want string }{
		{`export function load():int{return 42}`, `export { load } from "./library"`, `import {load} from "./barrel";function run():int{return load()}`, ""},
		{`export const load=():int=>42`, `import {load} from "./library";export {load}`, `import {load} from "./barrel";function run():int{return load()}`, ""},
		{`export let count:int=1`, `export {count} from "./library"`, `import {count} from "./barrel";function run():int{count++;return count}`, ""},
		{`export class Box<T>{constructor(public value:T){}}`, `export {Box} from "./library"`, `import {Box} from "./barrel";function run():int{return new Box<int>(42).value}`, ""},
		{`export struct Point<T>{public value:T;}`, `export {Point} from "./library"`, `import {Point} from "./barrel";function run():int{const p=Point<int>{value:42};return p.value}`, ""},
		{`export type Count=distinct int;export alias Values<T>=T[];export enum State{One,Two}`, `export {Count,Values,State} from "./library"`, `import {Count,Values,State} from "./barrel";function run():int{const c=Count(3);const v:Values<Count>=[c];return int(v[0])+int(State.Two)}`, ""},
		{`export interface Item{function read():int;}export constraint Scalar=int|string;export function identity<T extends Scalar>(v:T):T{return v}`, `export {Item,Scalar,identity} from "./library"`, `import {Item,Scalar,identity} from "./barrel";class C implements Item{public function read():int{return identity(42)}}`, ""},
		{`export function load():Result<int>{return ok(42)}`, `export {load} from "./library"`, `import {load} from "./barrel";function run():Result<int>{const f=load;return f()}`, ""},
		{`function load():int{return 42}`, `export {load} from "./library";function load():int{return 1}`, `import {load} from "./barrel";function run():int{return load()}`, ""},
		{`export const value=1`, `export {value} from "./library"`, `import {value} from "./barrel";const x=value`, ""},
		{`export {};const hidden=1`, `export {hidden} from "./library"`, `import {hidden} from "./barrel"`, "does not export"},
		{`export const value=1`, `export {missing} from "./library"`, `import {missing} from "./barrel"`, "does not declare"},
		{`export const value=1`, `export {value} from "./library";function run():int{return value}`, `import {value} from "./barrel"`, "undefined name"},
		{`export const value=1`, `export {value} from "./library";export const value=2`, `import {value} from "./barrel"`, "duplicate exported name"},
		{`export const value=1`, `export {value,value} from "./library"`, `import {value} from "./barrel"`, "duplicate exported name"},
		{`export const value=1`, `export {value} from "./library";export {value} from "./library"`, `import {value} from "./barrel"`, "duplicate exported name"},
		{`export const value=1`, `import {value} from "./library"`, `import {value} from "./barrel"`, "does not declare"},
		{`export const value=1`, `import go {Println} from "fmt";export {Println}`, `import {Println} from "./barrel"`, "does not declare"},
		{`export {value} from "./barrel"`, `export {value} from "./library"`, `import {value} from "./barrel"`, "import cycle"},
		{``, `export {value} from "./missing"`, `import {value} from "./barrel"`, "cannot load imported module"},
		{``, `export {value} from "unknown/module"`, `import {value} from "./barrel"`, "package imports are not supported"},
		{``, `export {fetch} from "kinmokusei/http"`, `import {fetch} from "./barrel";function main():void{}`, ""},
	} {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "entry.km")
			result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{entry: test.entry, filepath.Join(root, "library.km"): test.library, filepath.Join(root, "barrel.km"): test.barrel})
			if err != nil {
				t.Fatal(err)
			}
			if test.want != "" {
				for _, d := range result.Diagnostics {
					if strings.Contains(d.Message, test.want) {
						return
					}
				}
				t.Fatalf("diagnostics=%v want=%s", result.Diagnostics, test.want)
			}
			if len(result.Diagnostics) != 0 {
				t.Fatal(result.Diagnostics)
			}
			generated, err := codegen.Generate(result.Program, "reexports")
			if err != nil {
				t.Fatalf("%v\n%s", err, generated)
			}
		})
	}
}

func TestNamedReexportCycleDiagnostic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := filepath.Join(root, "first.km")
	second := filepath.Join(root, "second.km")
	result, err := CheckFilesWithOverlay([]string{first}, map[string]string{
		first:  `export {second} from "./second";export const first=1;`,
		second: `export {first} from "./first";export const second=2;`,
	})
	if err != nil || len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "import cycle") {
		t.Fatalf("err=%v diagnostics=%v", err, result.Diagnostics)
	}
}
