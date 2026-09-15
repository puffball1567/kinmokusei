package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNumericConstantAliasesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module constant-aliases.test\n\ngo 1.23\n",
		"values.km": `import go math from "math";
const Small=Base+1;const Base=254;
const Huge=1e400;const HugeAlias=Huge;
const Offset=2.;const Index=Offset;
const Pi=math.Pi;const Circle=Pi;
const Rounded:float32=16777217.;const Saved=Rounded;
export {Small as ByteValue,HugeAlias,Index,Circle,Saved};`,
		"bridge.km": `export {ByteValue as Narrow,HugeAlias,Index,Circle,Saved} from "./values";`,
		"entry.km": `import {Narrow,HugeAlias,Index,Circle,Saved} from "./bridge";
import go cmp from "cmp";
function keep<T>(v:T):T{return v;}
class Box<T>{constructor(public values:T[]){}public function read():T{const offset=Index;return this.values[offset];}}
export function Narrowed():byte{const n=Narrow;const m=n;return keep<byte>(m);}
export function Precise():float{const n=HugeAlias;const m=n+n;return m/n;}
export function NarrowFloat():float32{const n=Circle;return n;}
export function Rounding():float32{const n=Saved;const m=n;return m-16777216.;}
export function Complex():complex64{const z=1+2i;const a=z;const b=a+a;return b;}
export function Indices(xs:int[]):int[]{const n=Index;const m=n+1.;const box=new Box<int>(xs);return [xs[n],xs[m],box.read(),len(xs[n:m])];}
export function Sizes():int[]{const n=Index;const m=n+1.;const xs=makeSlice<int>(n,m);const channel=goChannel<int>(n);return [len(xs),cap(xs),cap(channel),cmp.Compare<byte>(Narrow,255)];}
export function Integers():byte{const a=3;const b=a<<2;const c=^b;const d=-c;return d;}
type Small=distinct int8;
export function Conversions():int{const a=120;const b=int8(a);const c=b;const d=Small(a);const e=d;return int(c)+int(e);}
export function Shadow():int{const n=2.;const m=n;{const n=7.;return int(m+n);}}
export function Runtime():int{let count=0;const get=():int=>{count++;return 4;};const a=get();const b=a;let x=3;const y=x;x=9;return b+count*100+y;}
export function Loops():float{let result=0.;for(const n=2.;result<1.;){const m=n;result=m;break;}return result;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "constantaliases")
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const m = n", "const b = a + a", "var b = a", "var m = n"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import("math";"cmp")
const Base=254;const Small=Base+1;const Huge=1e400;const HugeAlias=Huge;const Offset=2.;const Index=Offset;const Rounded float32=16777217.;const Saved=Rounded
func Narrowed()byte{const n=Small;const m=n;return m}
func Precise()float64{const n=HugeAlias;const m=n+n;return m/n}
func NarrowFloat()float32{const n=math.Pi;return n}
func Rounding()float32{const n=Saved;const m=n;return m-16777216.}
func Complex()complex64{const z=1+2i;const a=z;const b=a+a;return b}
func Indices(xs []int)[]int{const n=Index;const m=n+1.;return []int{xs[n],xs[m],xs[Index],len(xs[n:m])}}
func Sizes()[]int{const n=Index;const m=n+1.;xs:=make([]int,n,m);channel:=make(chan int,n);return []int{len(xs),cap(xs),cap(channel),cmp.Compare[byte](Small,255)}}
func Integers()byte{const a=3;const b=a<<2;const c=^b;const d=-c;return d}
type small int8
func Conversions()int{const a=120;const b=int8(a);const c=b;const d=small(a);const e=d;return int(c)+int(e)}
func Shadow()int{const n=2.;const m=n;{const n=7.;return int(m+n)}}
func Runtime()int{count:=0;get:=func()int{count++;return 4};a:=get();b:=a;x:=3;y:=x;x=9;_=x;return b+count*100+y}
func Loops()float64{result:=0.;for n:=2.;result<1.;{m:=n;result=m;break};return result}
`
	comparison := `package constantaliases_test
import("testing";"reflect";g "constant-aliases.test";r "constant-aliases.test/reference")
func TestConstants(t *testing.T){if g.Narrowed()!=r.Narrowed()||g.Precise()!=r.Precise()||g.NarrowFloat()!=r.NarrowFloat()||g.Rounding()!=r.Rounding()||g.Complex()!=r.Complex()||g.Integers()!=r.Integers()||g.Conversions()!=r.Conversions()||g.Shadow()!=r.Shadow()||g.Runtime()!=r.Runtime()||g.Loops()!=r.Loops(){t.Fatal("constants/storage")};if !reflect.DeepEqual(g.Sizes(),r.Sizes()){t.Fatal("sizes")};for _,xs:=range [][]int{{1,2,3,4},{9,8,7,6}}{if !reflect.DeepEqual(g.Indices(xs),r.Indices(xs)){t.Fatal("indices")}}}
`
	runGeneratedGoDifferentialTest(t, root, "constant-aliases.test", generated, reference, comparison)
}
