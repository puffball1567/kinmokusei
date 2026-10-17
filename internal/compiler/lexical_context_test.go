package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClosureLexicalBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `class Item{constructor(public value:int){}}
export function Capture(n:int):()=>int{
 const outer=():int=>{const first=()=>second();const second=():int=>{n++;return n;};return first();};
 return outer;
}
export function Flow(present:boolean,rounds:int):int[]{
 let initial=7;let value:*int|null=null;if(present){value=&initial;}
 let total=0;let calls=0;
 for(let i=0;i<rounds;i++){
  const nested=(stop:boolean):int=>{let sum=0;for(let j=0;j<4;j++){if(j===1){continue;}if(stop && j===3){break;}sum+=j;}return sum;};
  if(value===null){break;}total+=*value+nested(i===1);calls++;if(i===1){continue;}total++;
 }
 switch(rounds){case 0 {const nested=():int=>{switch(present){case true {return 10;}default{return 20;}}};total+=nested();break;}default{total+=30;}}
 return [total,calls];
}
export function Unreachable(n:int):int{
 let value:Item|null=new Item(n);
 const harmless=():void=>{return;const skipped=():void=>{value=null;};};
 harmless();return value.value;
}
export function Tasks(flag:boolean):int{
 const nested=():int=>{const value=():int=>7;const task=go value();if(flag){return await task;}detach task;return 3;};
 const outer=go nested();return await outer;
}
`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "lexical")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type item struct{value int}
func Capture(n int)func()int{outer:=func()int{second:=func()int{n++;return n};first:=func()int{return second()};return first()};return outer}
func Flow(present bool,rounds int)[]int{
 number:=7;var value *int;if present{value=&number};total,calls:=0,0
 for i:=0;i<rounds;i++{nested:=func(stop bool)int{sum:=0;for j:=0;j<4;j++{if j==1{continue};if stop && j==3{break};sum+=j};return sum};if value==nil{break};total+=*value+nested(i==1);calls++;if i==1{continue};total++}
 switch rounds{case 0:nested:=func()int{switch present{case true:return 10;default:return 20}};total+=nested();default:total+=30}
 return []int{total,calls}
}
func Unreachable(n int)int{value:=&item{n};harmless:=func(){return;skipped:=func(){value=nil};_ = skipped};harmless();return value.value}
func Tasks(flag bool)int{outer:=make(chan int,1);go func(){inner:=make(chan int,1);go func(){inner<-7}();if flag{outer<- <-inner}else{go func(){<-inner}();outer<-3}}();return <-outer}
`
	comparison := `package lexical_test
import("reflect";"testing";g "lexical-context.test";r "lexical-context.test/reference")
func TestBoundaries(t *testing.T){
 for _,n:=range []int{-10,0,8}{got,want:=g.Capture(n),r.Capture(n);for i:=0;i<4;i++{if a,b:=got(),want();a!=b{t.Fatal("capture",a,b)}};if a,b:=g.Unreachable(n),r.Unreachable(n);a!=b{t.Fatal("unreachable",a,b)}}
 for _,present:=range []bool{false,true}{for _,rounds:=range []int{0,1,2,5}{if a,b:=g.Flow(present,rounds),r.Flow(present,rounds);!reflect.DeepEqual(a,b){t.Fatal("flow",present,rounds,a,b)}}}
 for i:=0;i<10;i++{for _,flag:=range []bool{false,true}{if a,b:=g.Tasks(flag),r.Tasks(flag);a!=b{t.Fatal("tasks",a,b)}}}
}
`
	runGeneratedGoDifferentialTest(t, root, "lexical-context.test", generated, reference, comparison)
}

func TestClosureLexicalBoundariesRejectUnsafeBodies(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`class Item{public value:int=1;}class Box{constructor(public item:Item|null){}}function clear(box:Box):void{box.item=null;}function outer():void{return;const callback=():int=>{const box=new Box(new Item());if(box.item!==null){clear(box);return box.item.value;}return 0;};}`, "must be checked against null"},
		{`function value():int{return 1;}function outer():void{return;const callback=(flag:boolean):void=>{const task=go value();if(flag){return;}detach task;};}`, `Task "task" must be consumed`},
	} {
		path := filepath.Join(t.TempDir(), "invalid.km")
		if err := os.WriteFile(path, []byte(test.input), 0o644); err != nil {
			t.Fatal(err)
		}
		generated, diagnostics, err := EmitGo([]string{path}, "invalid")
		if err != nil || len(generated) != 0 || !strings.Contains(diagnosticsText(diagnostics), test.want) {
			t.Fatalf("err=%v diagnostics=%v generated=%d", err, diagnostics, len(generated))
		}
	}
}
