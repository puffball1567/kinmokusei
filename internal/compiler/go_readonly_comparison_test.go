package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGoReadOnlyComparisonMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, source := range map[string]string{
		"compare.km": `import go reflection from "reflect";
export function same<T>(a:T,b:T):boolean{return reflection.DeepEqual(a,b);}
`,
		"entry.km": `import {same} from "./compare";
import go {DeepEqual as equal} from "reflect";
import go r from "reflect";
import go math from "math";
struct Row{public text:string;public parts:Map<string,string[]>;}
struct Node{public text:string;public next:*Node;}
let callbacks:int=0;
let evaluations:int=0;
class C{constructor(public text:string){}public function string():bstring{callbacks++;return b"called";}public function equal(other:C):boolean{callbacks++;return true;}}
function mark<T>(v:T):T{evaluations++;return v;}
function pair(v:string[]):(string[],string[]){evaluations++;return v,v;}
function deferred(v:string[]):void{defer r.DeepEqual(v,v);}
export function Run():(boolean[],int,int){
 callbacks=0;evaluations=0;
 const a=["雪","\u0000"];const b=["雪","\u0000"];const other=["雨","\u0000"];
 const empty:string[]=[];const nilSlice:string[]=nil;
 const fixed:[2]string=["a","b"];
 const ma=makeMap<string,string[]>();ma["x"]=a;
 const mb=makeMap<string,string[]>();mb["x"]=b;
 const mc=makeMap<string,string[]>();mc["x"]=other;
 const row=Row{text:"row",parts:ma};const copy=Row{text:"row",parts:mb};
 let first=Node{text:"cycle",next:nil};first.next=&first;
 let second=Node{text:"cycle",next:nil};second.next=&second;
 let third=Node{text:"other",next:nil};third.next=&third;
 const pointer:*Node=nil;
 const c=new C("x");const d=new C("x");
 const callback=(v:string):void=>{callbacks++;};const missing:(v:string)=>void=nil;
 const task=go r.DeepEqual(a,b);const concurrent=await task;
 deferred(a);
 const nan=math.NaN();
 const values=[same(a,b),same(a,other),equal(ma,mb),equal(ma,mc),same(row,copy),
 r.DeepEqual(&first,&second),r.DeepEqual(&first,&third),r.DeepEqual(pointer,pointer),r.DeepEqual(pointer,nil),
 same(empty,nilSlice),same(nilSlice,nilSlice),same(fixed,fixed),same(c,d),same<C|null>(null,null),
 same(callback,callback),same(missing,missing),r.DeepEqual(nil,nil),same(nan,nan),
 same(mark(a),mark(b)),equal(pair(a)),concurrent];
 return values,callbacks,evaluations;
}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := filepath.Join(root, "entry.km")
	generated, diagnostics, err := EmitGo([]string{entry}, "readonlycomparison")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{entry}, "readonlycomparison")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import("math";"reflect")
func same[T any](a,b T)bool{return reflect.DeepEqual(a,b)}
type row struct{text string;parts map[string][]string}
type node struct{text string;next *node}
var callbacks,evaluations int
type c struct{text string}
func(*c)String()string{callbacks++;return "called"}
func(*c)Equal(*c)bool{callbacks++;return true}
func mark[T any](v T)T{evaluations++;return v}
func pair(v []string)([]string,[]string){evaluations++;return v,v}
func deferred(v []string){defer reflect.DeepEqual(v,v)}
func Run()([]bool,int,int){
callbacks=0;evaluations=0
a,b,other:=[]string{"雪","\x00"},[]string{"雪","\x00"},[]string{"雨","\x00"}
empty:=[]string{};var nilSlice []string;fixed:=[2]string{"a","b"}
ma,mb,mc:=map[string][]string{"x":a},map[string][]string{"x":b},map[string][]string{"x":other}
r,s:=row{"row",ma},row{"row",mb}
first,second,third:=node{text:"cycle"},node{text:"cycle"},node{text:"other"}
first.next=&first;second.next=&second;third.next=&third;var pointer *node
x,y:=&c{"x"},&c{"x"};callback:=func(string){callbacks++};var missing func(string)
done:=make(chan bool,1);go func(){done<-reflect.DeepEqual(a,b)}();concurrent:=<-done
deferred(a);nan:=math.NaN()
values:=[]bool{same(a,b),same(a,other),reflect.DeepEqual(ma,mb),reflect.DeepEqual(ma,mc),same(r,s),
reflect.DeepEqual(&first,&second),reflect.DeepEqual(&first,&third),reflect.DeepEqual(pointer,pointer),reflect.DeepEqual(pointer,nil),
same(empty,nilSlice),same(nilSlice,nilSlice),same(fixed,fixed),same(x,y),same[*c](nil,nil),
same(callback,callback),same(missing,missing),reflect.DeepEqual(nil,nil),same(nan,nan),
same(mark(a),mark(b)),reflect.DeepEqual(pair(a)),concurrent}
return values,callbacks,evaluations
}`
	tests := `package readonlycomparison_test
import("reflect";"testing";g "readonly-comparison.test";r "readonly-comparison.test/reference")
func TestComparison(t *testing.T){a,n,e:=g.Run();b,m,f:=r.Run();if !reflect.DeepEqual(a,b)||n!=m||e!=f{t.Fatalf("got %v/%d/%d want %v/%d/%d",a,n,e,b,m,f)};if n!=0||e!=3{t.Fatalf("callbacks=%d evaluations=%d",n,e)}}
`
	runGeneratedGoDifferentialTest(t, root, "readonly-comparison.test", generated, reference, tests)
}
