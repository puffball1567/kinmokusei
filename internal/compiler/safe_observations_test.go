package compiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeObservationPipeline(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function f<T>(v:T):bstring{return inspectTypeName(v)+inspectKind(v)+inspectString(v);}`,
		`function f<A,B>(a:A,b:B):boolean{return inspectSameType(a,b)&&inspectEqual(a,b);}`,
		`function f<T>(v:T):boolean{return inspectNil(v)||inspectComparable(v);}`,
		`function f<T>(v:T):float{return inspectFloat(v);}`,
		`function f<T>(v:T):Result<byte[]>{return jsonSnapshot(v);}`,
		`function f<T>(v:T):Result<byte[]>{return jsonSnapshot(v,"","  ");}`,
		`const answer=inspectKind(["safe"]);const f=()=>inspectNil(null);function main():void{const g=():Result<byte[]>=>{return jsonSnapshot(["safe"]);};const _=g();}`,
		`function main():void{const f=()=>inspectKind(42);const g=():Result<byte[]>=>{return jsonSnapshot(42);};f();const _=g();}`,
		`import go r from "reflect";import go j from "encoding/json";function main():void{const _=inspectTypeName(r.TypeFor<int>());const _=jsonSnapshot(j.Valid(nil));}`,
		`const label=inspectKind(new C("safe"));class C{constructor(public text:string){}}function main():void{const _=label;}`,
		`function inspectKind(v:int):int{return v;}function jsonSnapshot(v:int):int{return v;}function main():void{const a:int=inspectKind(1)+jsonSnapshot(2);}`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "input.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestSafeObservationRetainsBoundaryChecks(t *testing.T) {
	t.Parallel()
	for i, test := range []struct{ input, want string }{
		{`function main():void{inspectKind();}`, "expects 1 arguments"},
		{`function main():void{inspectEqual(1);}`, "expects 2 arguments"},
		{`function main():void{inspectKind<int>(1);}`, "does not accept explicit"},
		{`function main():void{inspectKind([1]...);}`, "does not accept spread"},
		{`function main():void{jsonSnapshot(1,"");}`, "expects 3 arguments"},
		{`function main():void{jsonSnapshot(1,1,2);}`, "cannot use"},
		{`function f():void{}function main():void{inspectKind(f());}`, "cannot inspect void"},
		{`function f():Result<int>{return ok(1);}function main():void{inspectKind(f());}`, "Result"},
		{`function f():int{return 1;}function main():void{const task=go f();inspectKind(task);}`, "Task"},
		{`function f():int{return 1;}function main():void{const task=go f();jsonSnapshot([task]);}`, "Task"},
		{`function main():void{const task=go inspectKind(1);const _=await task;}`, "does not support compiler built-ins"},
		{`function main():void{const task=go jsonSnapshot(1);const _=await task;}`, "does not support compiler built-ins"},
		{`function main():void{inspectKind(999999999999999999999999999999999999);}`, "overflows"},
		{`import go r from "reflect";function f<T>(v:T):bstring{return inspectString(r.ValueOf(v));}`, "cannot use T as any"},
		{`import go r from "reflect";function f(v:string[]):void{r.ValueOf(v);}`, "cannot use string[] as any"},
		{`import go r from "reflect";function f(v:string[]):void{r.TypeOf(v);}`, "cannot use string[] as any"},
		{`import go j from "encoding/json";function f<T>(v:T):void{j.Marshal(v);}`, "cannot use T as any"},
		{`import go j from "encoding/json";function f(v:*string):void{j.Unmarshal(nil,v);}`, "cannot use *string as any"},
		{`import go e from "errors";function f<T>(source:error,target:*T):boolean{return e.As(source,target);}`, "cannot use *T as any"},
		{`import go r from "reflect";function f<T>():*T{return r.New(r.TypeFor<T>()).Interface() as! *T;}`, "cannot establish a UTF-8"},
		{`import go f from "fmt";function display<T>(v:T):bstring{const unknown=f.Sprintf;return unknown("%#v",v);}`, "cannot use T as any"},
		{`import go f from "fmt";function display<T>(v:T):error{return f.Errorf("%w",v);}`, "cannot use T as any"},
		{`import go fmt from "fmt";class Item{public n:int=1;}class Holder{constructor(public value:Item|null){}}class C implements fmt.GoStringer{constructor(private holder:Holder){}public function GoString():bstring{this.holder.value=null;return b"";}}function f(h:Holder):int{if(h.value===null){return 0;}const _=fmt.Sprintf("%#v",new C(h));return h.value.n;}`, "nullable"},
		{`class Item{public n:int=1;}class Holder{constructor(public value:Item|null){}}class C{constructor(public holder:Holder){}public function MarshalJSON():Result<byte[]>{this.holder.value=null;return ok([byte(110),byte(117),byte(108),byte(108)]);}}function f(h:Holder):int{if(h.value===null){return 0;}const _=jsonSnapshot(new C(h));return h.value.n;}`, "nullable"},
	} {
		t.Run(fmt.Sprintf("case-%02d", i), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.km")
			checked, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: test.input})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range checked.Diagnostics {
				if strings.Contains(d.Message, test.want) {
					return
				}
			}
			t.Fatalf("diagnostics=%v want=%s", checked.Diagnostics, test.want)
		})
	}
}

func TestScalarObservationsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "observations.km")
	input := `export function Metadata<T>(v:T):(bstring,bstring,boolean,boolean){return inspectTypeName(v),inspectKind(v),inspectNil(v),inspectComparable(v);}
export function SameType<A,B>(a:A,b:B):boolean{return inspectSameType(a,b);}
export function Equal<A,B>(a:A,b:B):boolean{return inspectEqual(a,b);}
export function Float<T>(v:T):float{return inspectFloat(v);}
export function Text<T>(v:T):bstring{return inspectString(v);}
function kind<T>(v:T):bstring{return inspectKind(v);}
export function Async<T>(v:T):bstring{const task=go kind(v);return await task;}
let evaluations=0;
function mark():string[]{evaluations++;return ["safe"];}
export function Count():int{evaluations=0;const _=inspectKind(mark());const _=inspectSameType(mark(),mark());return evaluations;}
class Item{constructor(public text:string){}}
export function Native():boolean{const a=new Item("safe");const b=new Item("safe");const missing:Item|null=null;return inspectEqual(a,a)&&!inspectEqual(a,b)&&inspectNil(missing)&&inspectString("雪")==b"雪"&&!inspectComparable(["safe"]);}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "observations")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "observations")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "scalar-observations.test", generated,
		`package reference
import "reflect"
func Metadata[T any](v T)(string,string,bool,bool){r:=reflect.ValueOf(v);name:="<nil>";if t:=reflect.TypeOf(v);t!=nil{name=t.String()};isNil:=!r.IsValid();if r.IsValid(){switch r.Kind(){case reflect.Chan,reflect.Func,reflect.Interface,reflect.Map,reflect.Pointer,reflect.Slice,reflect.UnsafePointer:isNil=r.IsNil()}};return name,r.Kind().String(),isNil,r.Comparable()}
func SameType[A,B any](a A,b B)bool{return reflect.TypeOf(a)==reflect.TypeOf(b)}
func Equal[A,B any](a A,b B)bool{return reflect.ValueOf(a).Equal(reflect.ValueOf(b))}
func Float[T any](v T)float64{return reflect.ValueOf(v).Float()}
func Text[T any](v T)string{return reflect.ValueOf(v).String()}`,
		`package observations_test
import("math";"testing";g "scalar-observations.test";r "scalar-observations.test/reference")
var callbacks int
type named int
type printable struct{n int}
func(*printable)String()string{callbacks++;return "callback"}
func(*printable)Error()string{callbacks++;return "callback"}
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestBehavior(t *testing.T){
 var ptr *printable;var fn func();var ch chan int;var m map[string]int;var slice []int
 p:=&printable{1};q:=&printable{1};nan:=math.NaN();negativeZero:=math.Copysign(0,-1)
 values:=[]any{nil,ptr,fn,ch,m,slice,p,q,42,named(42),"雪","\xff",[]string{"safe"},struct{V any}{[]int{1}},nan,negativeZero,float32(-2.5),true}
 for _,v:=range values{a,b,c,d:=g.Metadata(v);e,f,h,i:=r.Metadata(v);if a!=e||b!=f||c!=h||d!=i{t.Fatalf("metadata %T: %v/%v/%v/%v vs %v/%v/%v/%v",v,a,b,c,d,e,f,h,i)};if g.Text(v)!=r.Text(v){t.Fatalf("String %T",v)}}
 for _,a:=range values{for _,b:=range values{if g.SameType(a,b)!=r.SameType(a,b){t.Fatal("dynamic type identity")};var got,want bool;gp:=panics(func(){got=g.Equal(a,b)});rp:=panics(func(){want=r.Equal(a,b)});if gp!=rp||!gp&&got!=want{t.Fatalf("equal %T/%T",a,b)}}}
 for _,v:=range []any{nan,negativeZero,float32(-2.5),2.5,42,nil}{var a,b float64;ap:=panics(func(){a=g.Float(v)});bp:=panics(func(){b=r.Float(v)});if ap!=bp||!ap&&math.Float64bits(a)!=math.Float64bits(b){t.Fatal("float observation")}}
 if callbacks!=0{t.Fatal("observation invoked methods",callbacks)};if g.Count()!=3||!g.Native()||g.Async([]string{"safe"})!="slice"{t.Fatal("native storage/evaluation/task")}
}`)
}
