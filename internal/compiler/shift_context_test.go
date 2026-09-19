package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShiftContextsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":    "module shift-context.test\n\ngo 1.23\n",
		"values.km": `export const One=1.0;`,
		"bridge.km": `export {One} from "./values";`,
		"entry.km": `import {One} from "./bridge";
import go cmp from "cmp";
type Byte=distinct byte;
constraint Small=~byte|~uint16;
function narrow<T extends Small>(n:int):T{return One<<n;}
function take(value:byte):byte{return value;}
function choose<T>(a:T,b:T):T{return a;}
class Bits{constructor(public count:int){}public function get():byte{return One<<this.count;}}
export function Narrow(n:int):byte{return One<<n;}
export function Nested(n:int):byte{return +((One<<n)+2);}
export function Peer(n:int,value:byte):byte{return (One<<n)+value;}
export function Equal(n:int,value:byte):boolean{return (One<<n)==value;}
export function Signed(n:int):int8{return -1>>n;}
export function Contexts(n:int):byte[]{const value:byte=One<<n;let assigned:byte=0;assigned=One<<n;const arrow=():byte=>One<<n;return [value,assigned,arrow(),take(One<<n),byte(Byte(One<<n)),narrow<byte>(n),new Bits(n).get()];}
export function Wide(n:int):uint64{return 9223372036854775808<<n;}
export function RuntimeConversion(n:int,value:int):byte{return byte(value<<n);}
export function Inferred(n:int,value:byte):byte{return choose(One<<n,value);}
export function GoGeneric(n:int,value:byte):int{return cmp.Compare(One<<n,value)+cmp.Compare<byte>(1<<n,value);}
export function Sizes(n:int):int[]{const a=make[int[]](One<<n,2.0<<n);const b=makeSlice<int>(One<<n,2.0<<n);const c=goChannel<int>(One<<n);const d=make[Map<int,int>](One<<n);const xs:int[]=[10,20,30,40,50,60,70,80];return [len(a),cap(a),len(b),cap(b),cap(c),len(d),xs[One<<n],len(xs[One<<n:2.0<<n])];}
export function Evaluation():int[]{let calls=0;const count=():int=>{calls++;return calls;};const n:byte=One<<count();const values=make[int[]](One<<count(),One<<count());return [int(n),calls,len(values),cap(values)];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "shifts")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "cmp"
const one=1.0
type small interface{~byte|~uint16}
func narrow[T small](n int)T{return one<<n}
func Narrow(n int)byte{return one<<n}
func Nested(n int)byte{return +((one<<n)+2)}
func Peer(n int,value byte)byte{return (one<<n)+value}
func Equal(n int,value byte)bool{return (one<<n)==value}
func Signed(n int)int8{return -1>>n}
type namedByte byte
func take(value byte)byte{return value}
type bits struct{count int}
func(b bits)get()byte{return one<<b.count}
func Contexts(n int)[]byte{var value byte=one<<n;var assigned byte;assigned=one<<n;arrow:=func()byte{return one<<n};return []byte{value,assigned,arrow(),take(one<<n),byte(namedByte(one<<n)),narrow[byte](n),bits{n}.get()}}
func Wide(n int)uint64{return 9223372036854775808<<n}
func RuntimeConversion(n,value int)byte{return byte(value<<n)}
func choose[T any](a,b T)T{return a}
func Inferred(n int,value byte)byte{return choose(one<<n,value)}
func GoGeneric(n int,value byte)int{return cmp.Compare(one<<n,value)+cmp.Compare[byte](1<<n,value)}
func Sizes(n int)[]int{a:=make([]int,one<<n,2.0<<n);b:=make([]int,one<<n,2.0<<n);c:=make(chan int,one<<n);d:=make(map[int]int,one<<n);xs:=[]int{10,20,30,40,50,60,70,80};return []int{len(a),cap(a),len(b),cap(b),cap(c),len(d),xs[one<<n],len(xs[one<<n:2.0<<n])}}
func Evaluation()[]int{calls:=0;count:=func()int{calls++;return calls};var n byte=one<<count();length:=int(one<<count());capacity:=int(one<<count());values:=make([]int,length,capacity);return []int{int(n),calls,len(values),cap(values)}}
`
	comparison := `package shifts_test
import("testing";"reflect";g "shift-context.test";r "shift-context.test/reference")
func capture(f func()byte)(value byte,panicked bool){defer func(){panicked=recover()!=nil}();value=f();return}
func TestInference(t *testing.T){for _,n:=range []int{0,1,7,8,63,64,100}{if g.Inferred(n,128)!=r.Inferred(n,128)||g.GoGeneric(n,128)!=r.GoGeneric(n,128){t.Fatal("inference",n)}}}
func TestShifts(t *testing.T){for _,n:=range []int{0,1,7,8,31,63,64,100}{if g.Narrow(n)!=r.Narrow(n)||g.Nested(n)!=r.Nested(n)||g.Peer(n,255)!=r.Peer(n,255)||g.Equal(n,128)!=r.Equal(n,128)||g.Signed(n)!=r.Signed(n)||g.Wide(n)!=r.Wide(n)||g.RuntimeConversion(n,300)!=r.RuntimeConversion(n,300){t.Fatal("shift",n)};if !reflect.DeepEqual(g.Contexts(n),r.Contexts(n)){t.Fatal("contexts",n)}};for _,n:=range []int{0,1,2}{if !reflect.DeepEqual(g.Sizes(n),r.Sizes(n)){t.Fatal("sizes",n)}};if !reflect.DeepEqual(g.Evaluation(),r.Evaluation()){t.Fatal("evaluation")};gv,gp:=capture(func()byte{return g.Narrow(-1)});rv,rp:=capture(func()byte{return r.Narrow(-1)});if gv!=rv||gp!=rp||!gp{t.Fatal("negative shift",gv,gp,rv,rp)}}
`
	runGeneratedGoDifferentialTest(t, root, "shift-context.test", generated, reference, comparison)
}
