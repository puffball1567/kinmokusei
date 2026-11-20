package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNamedResultsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"base.km": `export const value=99;
export function Pair(n:int):(value:int,err:error){
 defer (():void=>{value+=2;})();
 if(n<0){return n,nil;}
 value=n*2;
 return;
}
export function Try(n:int):(value:int,err:error){
 try {
  defer (():void=>{value+=5;})();
  if(n<0){const value=n;return value,nil;}
  if(n===0){throw new Exception("empty");}
  value=n;
  return;
 } catch(e:error) {
  err=e;
  return;
 } finally {
  value+=3;
 }
}
`,
		"entry.km": `import {Pair,Try,value} from "./base";
class Item{constructor(public n:int){}}
export type Total=distinct int;
class Box {public function next(n:int):(value:int){value=n+1;return;}}
struct Record { public function next(n:int):(value:int){value=n+2;return;} }
constraint Number=~int|~int16;
function save<T extends Number>(n:T):(value:T){value=n;return;}
function blank():( _:int, value:int){ value=9; return; }
function arrow(n:int):int{
 const work=(n:int):(value:int)=>{value=n+3;return;};
 return work(n);
}
export function Named(n:int):int[]{
 const [a,e]=Pair(n);
 const [b,f]=Try(n);
 const [empty,x]=blank();
 let failed=0;
 if(f!=nil){failed=1;}
 return [a,b,failed,empty,x,arrow(n),new Box().next(n),Record{}.next(n),int(save(int16(n))),value];
}
function optional(n:int):(item:Item|null){
 if(n>0){item=new Item(n);}
 return;
}
export function Optional(n:int):int{
 const item=optional(n);
 if(item===null){return -1;}
 return item.n;
}
export function Recover(handle:(slot:*int)=>void, stop:()=>void, n:int):(value:int){
 try {
  defer handle(&value);
  value=n;
  stop();
  return;
 } finally {
  value+=3;
 }
}
export function ShadowType():(Total:Total){
 try {
  defer (():void=>{Total+=1;})();
  return 7;
 } finally {
  Total+=2;
 }
}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "namedresults")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import "errors"
const value=99
func pair(n int)(value int,err error){
 defer func(){value+=2}()
 if n<0{return n,nil}
 value=n*2
 return
}
func try(n int)(value int,err error){
 defer func(){value+=5}()
 defer func(){value+=3}()
 switch {
 case n<0:
  return n,nil
 case n==0:
  return 0,errors.New("empty")
 default:
  value=n
  return
 }
}
type box struct{}
func(box)next(n int)(value int){value=n+1;return}
type record struct{}
func(record)next(n int)(value int){value=n+2;return}
func save[T ~int|~int16](n T)(value T){value=n;return}
func blank()(_ int,value int){value=9;return}
func Named(n int)[]int{
 a,_:=pair(n)
 b,err:=try(n)
 empty,x:=blank()
 failed:=0
 if err!=nil{failed=1}
 arrow:=func(n int)(value int){value=n+3;return}
 return []int{a,b,failed,empty,x,arrow(n),box{}.next(n),record{}.next(n),int(save(int16(n))),value}
}
type item struct{n int}
func optional(n int)(result *item){if n>0{result=&item{n}};return}
func Optional(n int)int{result:=optional(n);if result==nil{return -1};return result.n}
func Recover(handle func(*int), stop func(), n int)(value int){
 defer handle(&value)
 defer func(){value+=3}()
 value=n
 stop()
 return
}
type Total int
func ShadowType()(Total Total){
 defer func(){Total++}()
 defer func(){Total+=2}()
 return 7
}
`
	comparison := `package namedresults_test
import("reflect";"testing";g "named-results.test";r "named-results.test/reference")
func TestResults(t *testing.T){
 for _,n:=range []int{-10,0,7,255}{
  if got,want:=g.Named(n),r.Named(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)}
  if got,want:=g.Optional(n),r.Optional(n);got!=want{t.Fatal(n,got,want)}
 }
 handle:=func(slot *int){if recover()!=nil{*slot+=11}}
 for _,stop:=range []func(){func(){},func(){panic("stop")}}{
  if got,want:=g.Recover(handle,stop,7),r.Recover(handle,stop,7);got!=want{t.Fatal(got,want)}
 }
 if got,want:=int(g.ShadowType()),int(r.ShadowType());got!=want{t.Fatal(got,want)}
}`
	runGeneratedGoDifferentialTest(t, root, "named-results.test", generated, reference, comparison)
}
