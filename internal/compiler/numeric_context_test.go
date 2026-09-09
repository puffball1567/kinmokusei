package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNumericConstantContextsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"go.mod": "module numeric-contexts.test\n\ngo 1.23\n",
		"base.km": `const Offset=2.0;const Huge=1e400;
class Box<T>{constructor(public values:T[]){} public function read():T{return this.values[Offset];}}
`,
		"entry.km": `import {Offset,Huge,Box} from "./base";
import go time from "time";
function Index(a:int[]):int{return a[Offset]+a[2+0i]+a[time.Second/1e9];}
function Fixed(a:[4]int):int{return a[0x1p1];}
function Pointer(a:*[4]int):int{return a[2e0];}
function Text(s:string):byte{return s[3.0];}
function Slice(a:int[]):int[]{return a[1.0:Offset+1.0:4e0];}
function Sizes():int[]{const a=makeSlice<int>(Offset,4.0);const m=makeMap<int,int>(2e0);m[2.0]=7;const c=goChannel<int>(2+0i);return [len(a),cap(a),m[2e0],cap(c)];}
function Narrow():float32{return Offset;}
function LargeIntermediate():float{return Huge/Huge;}
function Rounded():float32{const n:float32=16777217.;return n-16777216.;}
function Stored(a:int[]):int{const b=new Box<int>(a);return b.read();}
function RuntimeAlias():float{const n=2.;const alias=n;return alias+1.;}
let count:int=0;
function next():int{count++;return 1;}
function Evaluation(a:int[]):int{count=0;const n=a[next()+0];return n+count*100;}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "numericcontexts")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "time"
const Offset=2.0
const Huge=1e400
func Index(a []int)int{return a[Offset]+a[2+0i]+a[time.Second/1e9]}
func Fixed(a [4]int)int{return a[0x1p1]}
func Pointer(a *[4]int)int{return a[2e0]}
func Text(s string)byte{return s[3.0]}
func Slice(a []int)[]int{return a[1.0:Offset+1.0:4e0]}
func Sizes()[]int{a:=make([]int,Offset,4.0);m:=make(map[int]int,2e0);m[2.0]=7;c:=make(chan int,2+0i);return []int{len(a),cap(a),m[2e0],cap(c)}}
func Narrow()float32{return Offset}
func LargeIntermediate()float64{return Huge/Huge}
func Rounded()float32{const n float32=16777217.;return n-16777216.}
type Box[T any]struct{values []T}
func(b *Box[T])read()T{return b.values[Offset]}
func Stored(a []int)int{return (&Box[int]{a}).read()}
func RuntimeAlias()float64{const n=2.;alias:=n;return alias+1.}
func Evaluation(a []int)int{count:=0;next:=func()int{count++;return 1};n:=a[next()+0];return n+count*100}
`
	comparison := `package numericcontexts_test
import("testing";"reflect";g "numeric-contexts.test";r "numeric-contexts.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestContexts(t *testing.T){
for _,a:=range [][4]int{{1,2,3,4},{-4,-3,-2,-1}}{
if g.Index(a[:])!=r.Index(a[:])||g.Fixed(a)!=r.Fixed(a)||g.Pointer(&a)!=r.Pointer(&a)||g.Stored(a[:])!=r.Stored(a[:])||g.Evaluation(a[:])!=r.Evaluation(a[:]){t.Fatal("indices/receiver/evaluation")}
x,y:=g.Slice(a[:]),r.Slice(a[:]);if !reflect.DeepEqual(x,y)||cap(x)!=cap(y){t.Fatal("slicing")}
}
for _,s:=range []string{"abcd","日本"}{if g.Text(s)!=r.Text(s){t.Fatal("string byte indexing")}}
if !reflect.DeepEqual(g.Sizes(),r.Sizes())||g.Narrow()!=r.Narrow()||g.LargeIntermediate()!=r.LargeIntermediate()||g.Rounded()!=r.Rounded()||g.RuntimeAlias()!=r.RuntimeAlias(){t.Fatal("constants/storage")}
if !panics(func(){g.Index([]int{1})})||!panics(func(){r.Index([]int{1})}){t.Fatal("runtime bounds")}
}
`
	runGeneratedGoDifferentialTest(t, root, "numeric-contexts.test", generated, reference, comparison)
}
