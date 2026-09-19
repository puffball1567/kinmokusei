package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNullableArrayConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":    "module nullable-array-constants.test\n\ngo 1.23\n",
		"width.km":  `export const pointer:*[3]int|null=null;export const width=len(pointer);`,
		"bridge.km": `export {width as Width} from "./width";`,
		"entry.km": `import {Width} from "./bridge";
import go crc32 from "hash/crc32";
type Array=distinct [3]int;
type Pointer=distinct *[3]int;
alias MaybePointer=*[3]int|null;
const globalSlice:int[]|null=null;
class Box<T>{constructor(public pointer:*[3]T|null){}public function size():int{const n=len(this.pointer);const m=cap(this.pointer);return n+m;}}
function generic<T>(pointer:*[3]T|null):int{const n=len(pointer);return n;}
export function Sizes(pointer:*[3]int|null):int[]{const n=len(pointer);const m=cap(pointer);const alias=n;let stored=alias;stored++;const box=new Box<int>(pointer);return [n,m,stored,Width,box.size(),generic<int>(pointer)];}
export function Named():int[]{const array:*Array|null=null;const pointer:Pointer|null=null;const table:*crc32.Table|null=null;const zero:*[0]int|null=null;const a=len(array);const p=cap(pointer);const imported=len(table);const empty=cap(zero);return [a,p,imported,empty];}
export function Ignored(index:int):int{const pointers:MaybePointer[]=[];const n=len(pointers[index]);const m=cap(pointers[index]);return n+m;}
export function Runtime():int[]{let calls=0;const next=():*[3]int|null=>{calls++;return null;};const n=len(next());const m=cap(next());const address=&n;return [*address,m,calls];}
export function Receive():int[]{const channel=goChannel<*[3]int|null>(2);channel<-null;channel<-null;const n=len(<-channel);const m=cap(<-channel);const address=&m;return [n,*address,len(channel)];}
export function RuntimeIndex(index:int):int{const pointers:MaybePointer[]=[];const next=():int=>index;return len(pointers[next()]);}
export function RuntimeSlice():int[]{const slice:int[]|null=null;const n=len(slice);const m=cap(slice);const address=&n;return [*address,m,len(globalSlice)];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "nullablearrayconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const n = len(pointer)", "const m = cap(pointer)", "const imported = len(table)", "const empty = cap(zero)", "const n = len(pointers[index])", "var n = len(next())", "var m = cap(<-channel)"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import "hash/crc32"
type array [3]int
type pointer *[3]int
type box[T any] struct{pointer *[3]T}
func (b *box[T]) size()int{const n=len(b.pointer);const m=cap(b.pointer);return n+m}
func generic[T any](p *[3]T)int{const n=len(p);return n}
var exportedPointer *[3]int
const width=len(exportedPointer)
func Sizes(p *[3]int)[]int{const n=len(p);const m=cap(p);const alias=n;stored:=alias;stored++;b:=&box[int]{p};return []int{n,m,stored,width,b.size(),generic(p)}}
func Named()[]int{var a *array;var p pointer;var table *crc32.Table;var zero *[0]int;const n=len(a);const m=cap(p);const imported=len(table);const empty=cap(zero);return []int{n,m,imported,empty}}
func Ignored(index int)int{pointers:=[]*[3]int{};const n=len(pointers[index]);const m=cap(pointers[index]);return n+m}
func Runtime()[]int{calls:=0;next:=func()*[3]int{calls++;return nil};n:=len(next());m:=cap(next());address:=&n;return []int{*address,m,calls}}
func Receive()[]int{channel:=make(chan *[3]int,2);channel<-nil;channel<-nil;n:=len(<-channel);m:=cap(<-channel);address:=&m;return []int{n,*address,len(channel)}}
func RuntimeIndex(index int)int{pointers:=[]*[3]int{};next:=func()int{return index};return len(pointers[next()])}
var globalSlice []int
func RuntimeSlice()[]int{var slice []int;n:=len(slice);m:=cap(slice);address:=&n;return []int{*address,m,len(globalSlice)}}
`
	comparison := `package nullablearrayconstants_test
import("testing";"reflect";g "nullable-array-constants.test";r "nullable-array-constants.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestNullableLengths(t *testing.T){
for _,p:=range []*[3]int{nil,{7,8,9}}{if !reflect.DeepEqual(g.Sizes(p),r.Sizes(p)){t.Fatal("sizes")}}
for name,pair:=range map[string][2][]int{"named":{g.Named(),r.Named()},"calls":{g.Runtime(),r.Runtime()},"receive":{g.Receive(),r.Receive()},"slice":{g.RuntimeSlice(),r.RuntimeSlice()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatalf("%s: %v != %v",name,pair[0],pair[1])}}
for _,index:=range []int{-1,0,999}{if g.Ignored(index)!=r.Ignored(index){t.Fatal("unevaluated index")};if panics(func(){g.RuntimeIndex(index)})!=panics(func(){r.RuntimeIndex(index)}){t.Fatal("runtime index panic")}}
}
`
	runGeneratedGoDifferentialTest(t, root, "nullable-array-constants.test", generated, reference, comparison)
}
