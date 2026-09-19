package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestUnsafeCollectionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":          "module unsafe-collections.test\n\ngo 1.23\n",
		"kinmokusei.toml": "[project]\nname = \"unsafe-collections\"\nversion = \"0.1.0\"\ngo-module = \"unsafe-collections.test\"\ngo-version = \"1.23\"\n[go.interop]\nunsafe = \"allow\"\n",
		"views.km": `import go {Slice,SliceData} from "unsafe";
export constraint SliceOf<E>=~E[];
export constraint PointerTo<E>=~*E;
export function data<E,S extends SliceOf<E>>(value:S):*E{return SliceData(value);}
export function view<E,P extends PointerTo<E>>(value:P,n:int):E[]{return Slice(value,n);}
`,
		"bridge.km": `export {SliceOf,PointerTo,data,view} from "./views";`,
		"entry.km": `import {SliceOf,PointerTo,data,view} from "./bridge";
import go u from "unsafe";
type Bytes=distinct byte[];
alias BytePointer=*byte;
class Views<E>{public function data<S extends SliceOf<E>>(value:S):*E{return u.SliceData(value);}}
class Item{constructor(public value:int){}}
alias Maybe=Item|null;
export function BytesCase():int[]{const raw:byte[]=[10,20,30];const values=Bytes(raw);const p=data<byte,Bytes>(values);const xs=view<byte,*byte>(p,3);xs[1]=25;const q=u.Add(u.Pointer(p),1.0);const middle=u.Slice(BytePointer(q),1.0);const back=u.Add(q,-1.0);return [int(values[1]),int(middle[0]),int(u.Slice(BytePointer(back),1)[0]),len(xs),cap(xs)];}
export function Objects():int[]{const values:Item[]=[new Item(4)];const p=new Views<Item>().data<Item[]>(values);const xs=u.Slice(p,1.0);xs[0]=new Item(9);return [values[0].value,(*p).value];}
export function Nullable():int{const values:Maybe[]=[null];const xs=u.Slice(u.SliceData(values),1);const v=xs[0];if(v===null){return 1;}return v.value;}
export function Sizes(n:uint):int[]{const values:byte[]=[65,66,67,68];const p=u.SliceData(values);const xs=u.Slice(p,1.0<<n);const text=u.String(p,1.0<<n);return [len(xs),len(text),int(text[0])];}
export function Counted():int[]{let count=0;const values:byte[]=[1,2];const source=():byte[]=>{count=count*10+1;return values;};const size=():int=>{count=count*10+2;return 2;};const xs=u.Slice(u.SliceData(source()),size());return [count,len(xs)];}
export function NilSlice(n:int):int{let p:*byte=nil;const xs=u.Slice(p,n);if(xs===nil){return -1;}return len(xs);}
export function NilString(n:int):int{return len(u.String(nil,n));}
export function Empty():int[]{let zero:byte[]=nil;const allocated=make[byte[]](0);let a=0;let b=0;if(data<byte,byte[]>(zero)===nil){a=1;}if(data<byte,byte[]>(allocated)!==nil){b=1;}return [a,b];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "unsafecollections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "unsafe"
type bytes []byte
func data[E any,S ~[]E](value S)*E{return unsafe.SliceData(value)}
func view[E any,P ~*E](value P,n int)[]E{return unsafe.Slice(value,n)}
func BytesCase()[]int{values:=bytes{10,20,30};p:=data[byte](values);xs:=view[byte](p,3);xs[1]=25;q:=unsafe.Add(unsafe.Pointer(p),1.0);middle:=unsafe.Slice((*byte)(q),1.0);back:=unsafe.Add(q,-1.0);return []int{int(values[1]),int(middle[0]),int(unsafe.Slice((*byte)(back),1)[0]),len(xs),cap(xs)}}
type item struct{value int}
func Objects()[]int{values:=[]*item{{4}};p:=data[*item](values);xs:=unsafe.Slice(p,1.0);xs[0]=&item{9};return []int{values[0].value,(*p).value}}
func Nullable()int{values:=[]*item{nil};xs:=unsafe.Slice(unsafe.SliceData(values),1);v:=xs[0];if v==nil{return 1};return v.value}
func Sizes(n uint)[]int{values:=[]byte{65,66,67,68};p:=unsafe.SliceData(values);xs:=unsafe.Slice(p,1.0<<n);text:=unsafe.String(p,1.0<<n);return []int{len(xs),len(text),int(text[0])}}
func Counted()[]int{count:=0;values:=[]byte{1,2};source:=func()[]byte{count=count*10+1;return values};size:=func()int{count=count*10+2;return 2};xs:=unsafe.Slice(unsafe.SliceData(source()),size());return []int{count,len(xs)}}
func NilSlice(n int)int{var p *byte;xs:=unsafe.Slice(p,n);if xs==nil{return -1};return len(xs)}
func NilString(n int)int{return len(unsafe.String(nil,n))}
func Empty()[]int{var zero []byte;allocated:=make([]byte,0);a,b:=0,0;if data[byte](zero)==nil{a=1};if data[byte](allocated)!=nil{b=1};return []int{a,b}}
`
	comparison := `package unsafecollections_test
import("testing";"reflect";"os";g "unsafe-collections.test";r "unsafe-collections.test/reference")
func capture(f func()int)(value int,panicked bool){defer func(){panicked=recover()!=nil}();value=f();return}
func TestCollections(t *testing.T){for _,pair:=range [][2][]int{{g.BytesCase(),r.BytesCase()},{g.Objects(),r.Objects()},{g.Counted(),r.Counted()},{g.Empty(),r.Empty()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatal("collections",pair)}};if g.Nullable()!=r.Nullable(){t.Fatal("nullable")};for _,n:=range []uint{0,1,2}{if !reflect.DeepEqual(g.Sizes(n),r.Sizes(n)){t.Fatal("sizes",n)}}}
func TestNil(t *testing.T){for _,n:=range []int{-1,0,1}{if n!=0&&os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE")=="1"{continue};for _,pair:=range [][2]func(int)int{{g.NilSlice,r.NilSlice},{g.NilString,r.NilString}}{gv,gp:=capture(func()int{return pair[0](n)});rv,rp:=capture(func()int{return pair[1](n)});if gv!=rv||gp!=rp{t.Fatal("nil",n,gv,gp,rv,rp)}}}}
`
	runGeneratedGoDifferentialTest(t, root, "unsafe-collections.test", generated, reference, comparison)
}
