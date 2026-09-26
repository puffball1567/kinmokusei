package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestUnsafeMultipleResultsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":          "module unsafe-results.test\n\ngo 1.23\n",
		"kinmokusei.toml": "[project]\nname=\"unsafe-results\"\nversion=\"0.1.0\"\ngo-module=\"unsafe-results.test\"\ngo-version=\"1.23\"\n[go.interop]\nunsafe=\"allow\"\n",
		"inputs.km":       `export function pair<T>(p:*T,n:int):(*T,int){return p,n;}`,
		"bridge.km":       `export {pair as inputs} from "./inputs";`,
		"entry.km": `import {inputs} from "./bridge";
import go u from "unsafe";
import go {Slice as view,String as text,Add as advance} from "unsafe";
type Bytes=distinct byte[];
alias P=*byte;
class Item{constructor(public value:int){}}
alias Maybe=Item|null;
class Producer<T>{constructor(public values:T[]){}public function get():(*T,int){return u.SliceData(this.values),len(this.values);}}
function generic<T>(values:T[]):T[]{return view(inputs(u.SliceData(values),len(values)));}
export function Run():int[]{
 let calls=0;const raw:byte[]=[65,66,67];const values=Bytes(raw);
 const produce=():(*byte,int)=>{calls++;return u.SliceData(values),len(values);};
 const xs=view(produce());xs[1]=68;const s=text(produce());
 const shift=():(u.Pointer,int)=>{calls++;return u.Pointer(u.SliceData(values)),1;};
 const p=P(advance(shift()));const middle=view(inputs(p,1));
 const narrow=generic(values);_=view(produce());
 return [calls,int(values[1]),int(s[1]),int(middle[0]),len(narrow)];
}
export function Objects():int[]{const xs:Item[]=[new Item(2)];const p=new Producer<Item>(xs);const ys=view(p.get());ys[0]=new Item(7);const optional:Maybe[]=[null];const zs=generic(optional);let empty=0;if(zs[0]===null){empty=1;}return [xs[0].value,empty];}
export function Nil(n:int):int[]{let p:*byte=nil;const xs=view(inputs(p,n));const s=text(inputs(p,n));let empty=0;if(xs===nil){empty=1;}return [empty,len(xs),len(s)];}
export function Evaluation():int[]{let calls=0;const values:byte[]=[1,2];const pair=():(*byte,int)=>{calls++;return u.SliceData(values),2;};const ignored=u.Sizeof(view(pair()));const measured=len(view(pair()));return [calls,int(ignored),measured];}
export function Panics(n:int):int{let p:*byte=nil;return len(view(inputs(p,n)));}
export function NegativeText(n:int):int{let p:*byte=nil;return len(text(inputs(p,n)));}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "unsaferesults")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "unsafe"
type bytes []byte
type item struct{value int}
func Run()[]int{calls:=0;values:=bytes{65,66,67};pair:=func()(*byte,int){calls++;return unsafe.SliceData(values),len(values)};p,n:=pair();xs:=unsafe.Slice(p,n);xs[1]=68;p,n=pair();s:=unsafe.String(p,n);shift:=func()(unsafe.Pointer,int){calls++;return unsafe.Pointer(unsafe.SliceData(values)),1};q,offset:=shift();middle:=unsafe.Slice((*byte)(unsafe.Add(q,offset)),1);narrow:=unsafe.Slice(unsafe.SliceData(values),len(values));p,n=pair();_=unsafe.Slice(p,n);return []int{calls,int(values[1]),int(s[1]),int(middle[0]),len(narrow)}}
func Objects()[]int{xs:=[]*item{{2}};ys:=unsafe.Slice(unsafe.SliceData(xs),len(xs));ys[0]=&item{7};optional:=[]*item{nil};zs:=unsafe.Slice(unsafe.SliceData(optional),len(optional));empty:=0;if zs[0]==nil{empty=1};return []int{xs[0].value,empty}}
func Nil(n int)[]int{var p *byte;xs:=unsafe.Slice(p,n);s:=unsafe.String(p,n);empty:=0;if xs==nil{empty=1};return []int{empty,len(xs),len(s)}}
func Evaluation()[]int{calls:=0;values:=[]byte{1,2};pair:=func()(*byte,int){calls++;return unsafe.SliceData(values),2};const ignored=unsafe.Sizeof(unsafe.Slice(pair()));p,n:=pair();measured:=len(unsafe.Slice(p,n));return []int{calls,int(ignored),measured}}
func Panics(n int)int{var p *byte;return len(unsafe.Slice(p,n))}
func NegativeText(n int)int{var p *byte;return len(unsafe.String(p,n))}
`
	comparison := `package unsaferesults_test
import("testing";"reflect";"os";g "unsafe-results.test";r "unsafe-results.test/reference")
func TestResults(t *testing.T){if got,want:=g.Run(),r.Run();!reflect.DeepEqual(got,want){t.Fatal("values",got,want)};if got,want:=g.Objects(),r.Objects();!reflect.DeepEqual(got,want){t.Fatal("objects",got,want)};if got,want:=g.Nil(0),r.Nil(0);!reflect.DeepEqual(got,want){t.Fatal("nil",got,want)};if got,want:=g.Evaluation(),r.Evaluation();!reflect.DeepEqual(got,want){t.Fatal("evaluation",got,want)}}
func panics(f func(int)int,n int)(out bool){defer func(){out=recover()!=nil}();f(n);return}
func TestPanics(t *testing.T){if os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE")=="1"{t.Skip("unsafe runtime failures are fatal under checkptr")};for _,pair:=range [][2]func(int)int{{g.Panics,r.Panics},{g.NegativeText,r.NegativeText}}{for _,n:=range []int{-1,1}{if !panics(pair[0],n)||!panics(pair[1],n){t.Fatal("missing panic",n)}}}}
`
	runGeneratedGoDifferentialTest(t, root, "unsafe-results.test", generated, reference, comparison)
}
