package compiler

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/codegen"
)

func TestExportAliases(t *testing.T) {
	t.Parallel()
	for i, test := range []struct{ library, entry, want string }{
		{`function local(n:int):int{return n+1}export {local as publicName}`, `import {publicName} from "./library";function run():int{return publicName(41)}`, ""},
		{`let local:int=1;export {local as first,local as second}`, `import {first,second} from "./library";function run():int{first++;return second}`, ""},
		{`class Local<T>{constructor(public value:T){}}export {Local as Public}`, `import {Public} from "./library";function run():int{return new Public<int>(42).value}`, ""},
		{`type Local=distinct int;export {Local as Public}`, `import {Public} from "./library";function run():int{return int(Public(42))}`, ""},
		{`enum Local{One,Two}export {Local as Public}`, `import {Public} from "./library";function run():int{return int(Public.Two)}`, ""},
		{`struct Local<T>{public value:T;}export {Local as Public}`, `import {Public} from "./library";function run():int{const p=Public<int>{value:42};return p.value}`, ""},
		{`class Local{public static function load():int{return 42}}export {Local as Public}`, `import {Public} from "./library";function run():int{return Public.load()}`, ""},
		{`constraint Local=int|string;export {Local as Scalar}`, `import {Scalar} from "./library";function identity<T extends Scalar>(value:T):T{return value}`, ""},
		{`alias Local<T>=T[];export {Local as Items}`, `import {Items} from "./library";function run():Items<int>{return [1,2]}`, ""},
		{`function local():int{return 1}export {local as renamed}`, `import {renamed} from "./library";function run():int{return local()}`, "undefined function"},
		{`class Local{}export {Local as Public}`, `import {Public} from "./library";function run():void{const x=new Local()}`, "unknown class"},
		{`class Local{}export {Local as Public}`, `import {Public} from "./library";function run(x:Local):void{}`, "unknown type"},
		{`let local:int=1;export {local as renamed}`, `import {renamed} from "./library";function run():int{return local}`, "undefined name"},
		{`function len():int{return 42}export {len as measure}`, `import {measure} from "./library";function run():int{return len([1,2])+measure()}`, ""},
		{`class Local{}export {Local as Time}`, `import {Time} from "./library";import go time from "time";function run(value:time.Time):Time{return new Time()}`, ""},
		{`function local():int{return 42}export {local as renamed}`, `import {renamed} from "./library";const result=()=>renamed();const value=result();`, ""},
		{`function local():int{return 42}export {local as renamed}`, `import {renamed} from "./library";const result=()=>local();const value=result();`, "undefined function"},
		{`let local:int=1;export {local as renamed}`, `import {renamed} from "./library";function run():void{local=2}`, "undefined name"},
		{`function local():int{return 1}export {local as renamed}`, `import {renamed} from "./library";function run():int{const local=42;return local}`, ""},
		{`class Local{}export {Local as Public}`, `import {Public} from "./library";function run<Local>(x:Local):Local{return x}`, ""},
		{`const local=1;export {local as renamed}`, `import {local} from "./library"`, "does not export"},
		{`const a=1;const b=2;export {a as same,b as same}`, ``, "duplicate exported name"},
		{`const a=1;export {a,a as a}`, ``, "duplicate exported name"},
		{`const a=1;export {a as b};function run():int{return b}`, ``, "undefined name"},
	} {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "entry.km")
			library := filepath.Join(root, "library.km")
			paths := []string{entry}
			if test.entry == "" {
				paths = []string{library}
			}
			result, err := CheckFilesWithOverlay(paths, map[string]string{entry: test.entry, library: test.library})
			if err != nil {
				t.Fatal(err)
			}
			if test.want != "" {
				for _, d := range result.Diagnostics {
					if strings.Contains(d.Message, test.want) {
						return
					}
				}
				t.Fatalf("%v want %s", result.Diagnostics, test.want)
			}
			if len(result.Diagnostics) != 0 {
				t.Fatal(result.Diagnostics)
			}
			if _, err := codegen.Generate(result.Program, "aliases"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
