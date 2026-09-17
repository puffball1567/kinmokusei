package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderedConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module ordered-constants.test\n\ngo 1.23\n",
		"values.km": `import go math from "math";
const Byte=min(255,256);const Huge=max(1e400,2e400);const Word=min("z","a");
const Rounded:float32=16777217.;const Saved=max(Rounded,16777216.);const Circle=min(math.Pi,math.E);
export {Byte,Huge,Word,Saved,Circle};`,
		"bridge.km": `export {Byte as Limit,Huge,Word,Saved,Circle} from "./values";`,
		"entry.km": `import {Limit,Huge,Word,Saved,Circle} from "./bridge";
import go cmp from "cmp";
type Text=distinct string;type Score=distinct int8;
function keep<T>(value:T):T{return value;}
function lower<T extends cmp.Ordered>(a:T,b:T):T{return min(a,b);}
class Clamp<T extends cmp.Ordered>{constructor(public low:T,public high:T){}public function apply(value:T):T{return max(this.low,min(value,this.high));}}
export function Narrow():byte{const n=Limit;const m=max(n,1);return keep<byte>(m);}
export function Precision():float{const huge=Huge;const ratio=min(huge/1e400,3);return ratio;}
export function Rounding():float32{const value=Saved;return max(value,16777216.)-16777216.;}
export function Imported():float32{return Circle;}
export function Named(value:int8):int8{const score=Score(value);return int8(max(Score(-10),min(score,10)));}
export function Texts(value:string):string{const word:Text=Word;const named=Text(value);const chosen=min("m",named);const result=max(word,chosen);return string(lower(result,named));}
export function Bounds():int[]{const a:[3]int=[10,20,30];const index=min(1.,2.);const count=max(index,3.);const xs=makeSlice<int>(index,count);const end=min(len(a),4.);const empty:int[]=[];const ignored=len(copyArray[[3]int](empty[min(1,2):]));return [a[index],len(xs),cap(xs),end,ignored,len(max("温","泉")),cmp.Compare<byte>(Limit,255)];}
export function Generic(value:int):int{return new Clamp<int>(-3,5).apply(value);}
export function GenericText(value:string):string{return new Clamp<string>("b","m").apply(value);}
export function Runtime():int{let order=0;const mark=(digit:int,value:int):int=>{order=order*10+digit;return value;};const first=min(mark(1,7),mark(2,3),mark(3,5));const second=max(mark(4,2),mark(5,9));const address=&first;return order*100+*address*10+second;}
export function Storage():int{let value=5;const chosen=min(value,10);const alias=chosen;const pointer=&alias;value=20;return *pointer+value;}
export function Shift(n:int,value:byte):byte{return min(1<<n,value);}
export function NestedShift(n:int,value:byte):byte{return max(value,+(1<<n)+2);}
export function ShiftCalls():byte{let calls=0;const count=():int=>{calls++;return 2;};const value=min(1<<count(),byte(10));return value+byte(calls*10);}
export function Shadow():int{const min=(a:int,b:int):int=>a+b;const value=min(1,2);const p=&value;return *p;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "orderedconstants")
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const m = max(n, 1)", "const ratio = min(huge/1e400, 3)", "const index = min(1.", "const ignored = len(", "var chosen = min(value, 10)", "var alias = chosen", "var first = min(mark("} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import("math";"cmp")
type text string;type score int8
const limit=min(255,256);const huge=max(1e400,2e400);const word=min("z","a");const rounded float32=16777217.;const saved=max(rounded,16777216.);const circle=min(math.Pi,math.E)
func Narrow()byte{const n=limit;const m=max(n,1);return m}
func Precision()float64{const ratio=min(huge/1e400,3);return ratio}
func Rounding()float32{const value=saved;return max(value,16777216.)-16777216.}
func Imported()float32{return circle}
func Named(value int8)int8{s:=score(value);return int8(max(score(-10),min(s,10)))}
func lower[T cmp.Ordered](a,b T)T{return min(a,b)}
func Texts(value string)string{const w text=word;named:=text(value);chosen:=min("m",named);result:=max(w,chosen);return string(lower(result,named))}
func Bounds()[]int{a:=[3]int{10,20,30};const index=min(1.,2.);const count=max(index,3.);xs:=make([]int,index,count);const end=min(len(a),4.);empty:=[]int{};const ignored=len([3]int(empty[min(1,2):]));return []int{a[index],len(xs),cap(xs),end,ignored,len(max("温","泉")),cmp.Compare[byte](limit,255)}}
func Generic(value int)int{return max(-3,min(value,5))}
func GenericText(value string)string{return max("b",min(value,"m"))}
func Runtime()int{order:=0;mark:=func(digit,value int)int{order=order*10+digit;return value};first:=min(mark(1,7),mark(2,3),mark(3,5));second:=max(mark(4,2),mark(5,9));address:=&first;return order*100+*address*10+second}
func Storage()int{value:=5;chosen:=min(value,10);alias:=chosen;pointer:=&alias;value=20;return *pointer+value}
func Shift(n int,value byte)byte{return min(1<<n,value)}
func NestedShift(n int,value byte)byte{return max(value,+(1<<n)+2)}
func ShiftCalls()byte{calls:=0;count:=func()int{calls++;return 2};value:=min(1<<count(),byte(10));return value+byte(calls*10)}
func Shadow()int{min:=func(a,b int)int{return a+b};value:=min(1,2);p:=&value;return *p}
`
	comparison := `package orderedconstants_test
import("testing";"reflect";g "ordered-constants.test";r "ordered-constants.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestOrdered(t *testing.T){if g.Narrow()!=r.Narrow()||g.Precision()!=r.Precision()||g.Rounding()!=r.Rounding()||g.Imported()!=r.Imported()||g.Runtime()!=r.Runtime()||g.Storage()!=r.Storage()||g.ShiftCalls()!=r.ShiftCalls()||g.Shadow()!=r.Shadow(){t.Fatal("constant/evaluation")};if !reflect.DeepEqual(g.Bounds(),r.Bounds()){t.Fatal("bounds")};for _,v:=range []int8{-128,-10,0,10,127}{if g.Named(v)!=r.Named(v)||g.Generic(int(v))!=r.Generic(int(v)){t.Fatal("named/generic")}};for _,s:=range []string{"","a","c","z","温泉"}{if g.Texts(s)!=r.Texts(s)||g.GenericText(s)!=r.GenericText(s){t.Fatal("text")}};for _,n:=range []int{0,1,7,8,63,64,100}{if g.Shift(n,100)!=r.Shift(n,100)||g.NestedShift(n,100)!=r.NestedShift(n,100){t.Fatal("shift context")}};gp:=panics(func(){g.Shift(-1,10)});rp:=panics(func(){r.Shift(-1,10)});if gp!=rp||!gp{t.Fatal("negative shift")}}
`
	runGeneratedGoDifferentialTest(t, root, "ordered-constants.test", generated, reference, comparison)
}
