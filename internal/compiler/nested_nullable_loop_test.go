package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNestedNullableLoopsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `class Item { public value:int=3; }
export function Run(n:int):int {
 let p:Item|null=new Item();let total=0;
 for(let i=0;i<n;i++) {
   p=new Item();
   for(let j=0;j<2;j++) {
     if(p!==null){const current=p;total+=current.value;}
     p=null;
   }
 }
 if(p!==null){return total+p.value;}
 return total;
}
export function Checked(n:int):int {
 let value=7;let p:*int|null=&value;let total=0;
 for(let i=0;i<n;i++) {
   p=&value;
   for(let j=0;j<2;j++) {
     if(p!==null){total+=*p;}
     p=null;
   }
 }
 if(p!==null){return total+*p;}
 return total;
}
function work():int{return 7;}
export function Effects(n:int,throwing:boolean):int[]{
 let p:Item|null=new Item();let events=0;
 const run=()=>{
  try {
   for(let i=0;i<n;i++){
    p=new Item();
    for(let j=0;j<2;j++){
     p=null;if(throwing){throw new Exception("expected");}
     if(i===1){return 13;}
    }
   }
   return 5;
  }catch(_:Exception){return 9;}
  finally{events+=7;}
 };
 const value=run();let flag=0;const snapshot=p;if(snapshot!==null){flag=snapshot.value;}
 return [value,events,flag];
}
export function TaskEffects(n:int,log:*int):int{
 const task=go work();let p:Item|null=new Item();
 try{for(let i=0;i<n;i++){p=new Item();for(let j=0;j<2;j++){p=null;if(i===1){return 13;}}}}
 finally{*log+=await task;}
 return 5;
}`
	path := filepath.Join(root, "loops.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "loops")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type Item struct{value int}
func Run(n int)int{
 p:=&Item{3};total:=0
 for i:=0;i<n;i++{p=&Item{3};for j:=0;j<2;j++{if p!=nil{current:=p;total+=current.value};p=nil}}
 if p!=nil{return total+p.value};return total
}
func Checked(n int)int{
 value:=7;p:=&value;total:=0
 for i:=0;i<n;i++{p=&value;for j:=0;j<2;j++{if p!=nil{total+=*p};p=nil}}
 if p!=nil{return total+*p};return total
}
func Effects(n int,throwing bool)[]int{
 p:=&Item{3};events:=0
 run:=func()(result int){
  defer func(){events+=7}()
  defer func(){if recover()!=nil{result=9}}()
  for i:=0;i<n;i++{p=&Item{3};for j:=0;j<2;j++{p=nil;if throwing{panic("expected")};if i==1{return 13}}}
  return 5
 }
 value:=run();flag:=0;if snapshot:=p;snapshot!=nil{flag=snapshot.value}
 return []int{value,events,flag}
}
func TaskEffects(n int,log *int)int{
 task:=make(chan int,1);go func(){task<-7}();defer func(){*log+=<-task}()
 for i:=0;i<n;i++{for j:=0;j<2;j++{if i==1{return 13}}};return 5
}`
	testSource := `package loops_test
import("testing";"reflect";g "nested-loops.test";r "nested-loops.test/reference")
func TestLoops(t *testing.T){for _,n:=range []int{-1,0,1,2,8}{
 if got,want:=g.Run(n),r.Run(n);got!=want{t.Fatalf("class n=%d: %d != %d",n,got,want)}
 if got,want:=g.Checked(n),r.Checked(n);got!=want{t.Fatalf("pointer n=%d: %d != %d",n,got,want)}
 for _,throwing:=range []bool{false,true}{if got,want:=g.Effects(n,throwing),r.Effects(n,throwing);!reflect.DeepEqual(got,want){t.Fatalf("effects n=%d throw=%v: %v != %v",n,throwing,got,want)}}
 gl,rl:=0,0;if got,want:=g.TaskEffects(n,&gl),r.TaskEffects(n,&rl);got!=want||gl!=rl||gl!=7{t.Fatalf("Task n=%d: %d,%d != %d,%d",n,got,gl,want,rl)}
}}`
	runGeneratedGoDifferentialTest(t, root, "nested-loops.test", generated, reference, testSource)
}
