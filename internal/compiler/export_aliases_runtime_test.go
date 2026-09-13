package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportAliasesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module export-aliases.test\n\ngo 1.23\n",
		"library.km": `let count:int=0;
type Count=distinct int;
class Box<T>{constructor(public value:T){}}
interface Reader{function read():int;}
class Counter implements Reader{public function read():int{return count;}}
function reader():Reader{return new Counter();}
function make(n:int):Box<Count>{return new Box<Count>(Count(n));}
const load=():Result<int>=>{return ok(count);};
function len():int{return 10;}
export {count as First,count as Second,Count as Units,Box as Container,Reader as View,reader as Watch,make as Create,load as Load,len as Measure};`,
		"left.km": `export {First as Shared,Units as Amount,Container as Bin,View as Reader,Watch,Create,Load,Measure} from "./library";`,
		"right.km": `import {Second,Units,Container,Create} from "./library";
export {Second as Other};
export function update(n:int):Container<Units>{Second+=n;return Create(Second);}`,
		"facade.km": `export {Shared,Amount,Bin,Reader,Watch,Create,Load,Measure} from "./left";
export {Other,update as Change} from "./right";
function Load():int{return -1;}`,
		"entry.km": `import {Shared,Other,Amount,Bin,Reader,Watch,Create,Load,Measure,Change} from "./facade";
export function Run(n:int):Result<int>{
const p=&Shared;const q=&Other;const watcher:Reader=Watch();Shared+=n;
const box:Bin<Amount>=Change(n+1);const own=Create(n);const f=Load;const value=f()?;
if(p!=q){return ok(-1);}return ok(value+int(box.value)+int(own.value)+watcher.read()+*p+len([1,2])+Measure());
}
class Box{}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "aliases")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var count int
func Run(n int)(int,error){count+=n;count+=n+1;return 4*count+n+2+10,nil}
`
	comparison := `package aliases_test
import("testing";"fmt";g "export-aliases.test";r "export-aliases.test/reference")
func TestContracts(t *testing.T){for _,n:=range []int{0,1,-2,7,3}{gv,ge:=g.Run(n);rv,re:=r.Run(n);if gv!=rv||fmt.Sprint(ge)!=fmt.Sprint(re){t.Errorf("Run(%d)=(%d,%v) want (%d,%v)",n,gv,ge,rv,re)}}}
`
	runGeneratedGoDifferentialTest(t, root, "export-aliases.test", generated, reference, comparison)
}
