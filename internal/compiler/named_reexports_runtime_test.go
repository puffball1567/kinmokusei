package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNamedReexportsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":    "module named-reexports.test\n\ngo 1.23\n",
		"state.km":  `export let trace:int=0;export function mark(n:int):int{trace=trace*10+n;return n;}`,
		"first.km":  `import {mark} from "./state";export const first=mark(1);`,
		"second.km": `import {mark} from "./state";export const second=mark(2);`,
		"library.km": `export let count:int=0;
export type Count=distinct int;
export class Box<T>{constructor(public value:T){}}
export interface Reader{function read():int;}
class Counter implements Reader{public function read():int{return count;}}
export function reader():Reader{return new Counter();}
export function make(n:int):Box<Count>{return new Box<Count>(Count(n));}
export const load=():Result<int>=>{return ok(count);};
export function bump(n:int):void{count+=n;}
function helper():int{return 99;}
`,
		"left.km":  `export {count,Count,Box,Reader,reader,make,load,bump} from "./library";`,
		"right.km": `import {count,Count,Box,Reader,reader,make,load,bump} from "./library";export {count,Count,Box,Reader,reader,make,load,bump};`,
		"change.km": `import {count,Count,Box,make,bump} from "./right";
export function change(n:int):Box<Count>{bump(n);return make(count);}`,
		"barrel.km": `export {first} from "./first";
import {second} from "./second";
export {second};
export {count,Count,Box,Reader,reader,make,load,bump} from "./left";
export {change} from "./change";
export {trace} from "./state";
function load():int{return -1;}
`,
		"entry.km": `import {first,second,count,Count,Box,Reader,reader,make,load,bump,change,trace} from "./barrel";
export function Run(n:int):Result<int>{
  const p=&count;const watcher:Reader=reader();bump(n);
  const box:Box<Count>=change(n+1);const f=load;const value=f()?;
  const own=make(n);return ok(trace*10000+first*1000+second*100+value+int(box.value)+watcher.read()+*p+int(own.value));
}
function helper():int{return 1;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "reexports")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var count int
func Run(n int)(int,error){count+=n;count+=n+1;return 12*10000+1000+200+4*count+n,nil}
`
	comparison := `package reexports_test
import("testing";"fmt";g "named-reexports.test";r "named-reexports.test/reference")
func TestContracts(t *testing.T){for _,n:=range []int{0,1,-2,7,3}{gv,ge:=g.Run(n);rv,re:=r.Run(n);if gv!=rv||fmt.Sprint(ge)!=fmt.Sprint(re){t.Errorf("Run(%d)=(%d,%v) want (%d,%v)",n,gv,ge,rv,re)}}}
`
	runGeneratedGoDifferentialTest(t, root, "named-reexports.test", generated, reference, comparison)
}
