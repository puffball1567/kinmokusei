package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMakeCollectionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module collection-make.test\n\ngo 1.23\n",
		"native/native.go": `package native
type Slice[E any] interface{~[]E}
type Numbers []int
type Lookup map[string]int
type Channel chan int
`,
		"allocate.km": `import go native from "collection-make.test/native";
export constraint Slice<E>=native.Slice<E>;
export constraint Mapping<V>=~Map<string,V>;
export constraint Sending<E>=~GoChannel<E>|~GoSendChannel<E>;
export constraint Receiving<E>=~GoChannel<E>|~GoReceiveChannel<E>;
export function allocate<E,T extends Slice<E>>(n:int,c:int):T{return make[T](n,c);}
export function mapping<V,T extends Mapping<V>>(n:int):T{return make[T](n);}
export function sending<E,T extends Sending<E>>(n:int):T{return make[T](n);}
export function receiving<E,T extends Receiving<E>>(n:int):T{return make[T](n);}
`,
		"exports.km": `export {Slice,allocate,mapping,sending,receiving} from "./allocate";`,
		"entry.km": `import {Slice,allocate,mapping,sending,receiving} from "./exports";
import go native from "collection-make.test/native";
type Numbers=distinct int[];
type Lookup=distinct Map<string,int>;
class Allocator<E>{public function make<T extends Slice<E>>(n:int):T{return make[T](n);}}
class Item{constructor(public value:int){}}
alias Maybe=Item|null;
export function Slices():int[]{const a=allocate<int,Numbers>(2,4);a[0]=7;const b=make[native.Numbers](2.0,4.0);b[1]=8;const zero=make[Numbers](0);let present=0;if(zero!==nil){present=1;}return [a[0],len(a),cap(a),b[1],present];}
export function Maps():int[]{const a=mapping<int,Lookup>(2);a["x"]=9;const b=make[native.Lookup]();b["y"]=8;return [a["x"],b["y"],len(a),len(b)];}
export function Channels():int[]{const ch=make[native.Channel](2);ch<-7;const value=<-ch;const send=sending<int,GoSendChannel<int>>(2);send<-1;closeGoChannel(send);const recv=receiving<int,GoReceiveChannel<int>>(3);const empty=make[GoChannel<int>]();closeGoChannel(empty);const [zero,open]=<-empty;let flag=0;if(open){flag=1;}return [value,len(send),cap(send),cap(recv),zero,flag];}
export function Objects():int[]{const values=new Allocator<Maybe>().make<Maybe[]>(2);let empty=0;if(values[0]===null){empty=1;}values[1]=new Item(4);const value=values[1];if(value!==null){return [empty,value.value];}return [empty,0];}
export function Ordered():int[]{let order=0;const size=(digit:int,n:int):int=>{order=order*10+digit;return n;};const values=make[Numbers](size(1,2),size(2,4));return [order,len(values),cap(values)];}
export function SliceLength(n:int,c:int):int{return len(make[Numbers](n,c));}
export function Channel(n:int):int{return cap(make[GoChannel<int>](n));}
export function MapHint(n:int):int{return len(make[Lookup](n));}
export function Wide(n:uint64):int{return len(make[int[]](n,n));}
export function Shadow():int{const make=(n:int):int=>n+1;return make(4);}
export function Names(kinmokuseiMakeLength:int,kinmokuseiMakeCapacity:int):int[]{const a=make[int[]](1,kinmokuseiMakeLength);const b=makeSlice[int](1,kinmokuseiMakeLength);const c=make[int[]](kinmokuseiMakeLength,kinmokuseiMakeCapacity);return [len(a),cap(a),len(b),cap(b),len(c),cap(c)];}
`,
	}
	for name, input := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "collections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type numbers []int
type lookup map[string]int
func allocate[E any,T ~[]E](n,c int)T{return make(T,n,c)}
func mapping[V any,T ~map[string]V](n int)T{return make(T,n)}
func sending[E any,T interface{~chan E|~chan<- E}](n int)T{return make(T,n)}
func receiving[E any,T interface{~chan E|~<-chan E}](n int)T{return make(T,n)}
func Slices()[]int{a:=allocate[int,numbers](2,4);a[0]=7;b:=make([]int,2.0,4.0);b[1]=8;zero:=make(numbers,0);present:=0;if zero!=nil{present=1};return []int{a[0],len(a),cap(a),b[1],present}}
func Maps()[]int{a:=mapping[int,lookup](2);a["x"]=9;b:=make(map[string]int);b["y"]=8;return []int{a["x"],b["y"],len(a),len(b)}}
func Channels()[]int{ch:=make(chan int,2);ch<-7;value:=<-ch;send:=sending[int,chan<- int](2);send<-1;close(send);recv:=receiving[int,<-chan int](3);empty:=make(chan int);close(empty);zero,open:=<-empty;flag:=0;if open{flag=1};return []int{value,len(send),cap(send),cap(recv),zero,flag}}
type item struct{value int}
func Objects()[]int{values:=allocate[*item,[]*item](2,2);empty:=0;if values[0]==nil{empty=1};values[1]=&item{4};value:=values[1];if value!=nil{return []int{empty,value.value}};return []int{empty,0}}
func Ordered()[]int{order:=0;size:=func(digit,n int)int{order=order*10+digit;return n};n:=size(1,2);c:=size(2,4);values:=make(numbers,n,c);return []int{order,len(values),cap(values)}}
func SliceLength(n,c int)int{return len(make(numbers,n,c))}
func Channel(n int)int{return cap(make(chan int,n))}
func MapHint(n int)int{return len(make(lookup,n))}
func Wide(n uint64)int{return len(make([]int,n,n))}
func Shadow()int{make:=func(n int)int{return n+1};return make(4)}
func Names(n,c int)[]int{a:=make([]int,1,n);b:=make([]int,1,n);d:=make([]int,n,c);return []int{len(a),cap(a),len(b),cap(b),len(d),cap(d)}}
`
	comparison := `package collections_test
import("testing";"reflect";g "collection-make.test";r "collection-make.test/reference")
func capture(f func()int)(value int,panicked bool){defer func(){panicked=recover()!=nil}();value=f();return}
func TestMake(t *testing.T){for _,pair:=range [][2][]int{{g.Slices(),r.Slices()},{g.Maps(),r.Maps()},{g.Channels(),r.Channels()},{g.Objects(),r.Objects()},{g.Ordered(),r.Ordered()},{g.Names(3,4),r.Names(3,4)}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatalf("got %v want %v",pair[0],pair[1])}};if g.Shadow()!=r.Shadow(){t.Fatal("shadow")}}
func TestSizes(t *testing.T){for _,sizes:=range [][2]int{{0,0},{1,3},{-1,2},{2,1},{0,-1}}{n,c:=sizes[0],sizes[1];gv,gp:=capture(func()int{return g.SliceLength(n,c)});rv,rp:=capture(func()int{return r.SliceLength(n,c)});if gv!=rv||gp!=rp{t.Fatal("slice",sizes,gv,gp,rv,rp)}};for _,n:=range []int{-1,0,2}{for _,pair:=range [][2]func(int)int{{g.Channel,r.Channel},{g.MapHint,r.MapHint}}{gv,gp:=capture(func()int{return pair[0](n)});rv,rp:=capture(func()int{return pair[1](n)});if gv!=rv||gp!=rp{t.Fatal("size",n,gv,gp,rv,rp)}}};for _,n:=range []uint64{0,2,^uint64(0)}{gv,gp:=capture(func()int{return g.Wide(n)});rv,rp:=capture(func()int{return r.Wide(n)});if gv!=rv||gp!=rp{t.Fatal("wide",n,gv,gp,rv,rp)}}}
`
	runGeneratedGoDifferentialTest(t, root, "collection-make.test", generated, reference, comparison)
}
