package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectOperandFlowMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `export function Run(ready:boolean):int[]{
 let trace=0;let targetCalls=0;let chosen=0;
 const input=goChannel[int](1);if(ready){input<-7;}
 let output:GoChannel<int>=nil;
 const channel=(ch:GoChannel<int>,digit:int):GoChannel<int>=>{trace=trace*10+digit;return ch;};
 const value=():int=>{trace=trace*10+3;return 9;};
 const index=():int=>{targetCalls++;return 0;};
 let values=[0];
 select{
  default{chosen=1;}
  case values[index()]=<-channel(input,1){chosen=2;}
  case channel(output,2)<-value(){chosen=3;}
 }
 return [trace,targetCalls,chosen,values[0]];
}
export function SendTask(ready:boolean):int{
 const output=goChannel[int](1);if(!ready){output<-8;}
 const work=():int=>{return 7;};const task=go work();
 select{case output<-await task{} default{}}
 return <-output;
}
export function ReceiveTask(ready:boolean):int{
 const input=goChannel[int](1);if(ready){input<-7;}
 const get=():GoChannel<int>=>{return input;};const task=go get();
 select{default{return 8;} case const n=<-await task{return n;}}
}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "selectflow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Run(ready bool)[]int{
 trace,targetCalls,chosen:=0,0,0
 input:=make(chan int,1);if ready{input<-7};var output chan int
 channel:=func(ch chan int,digit int)chan int{trace=trace*10+digit;return ch}
 value:=func()int{trace=trace*10+3;return 9}
 index:=func()int{targetCalls++;return 0};values:=[]int{0}
 select{default:chosen=1;case values[index()]=<-channel(input,1):chosen=2;case channel(output,2)<-value():chosen=3}
 return []int{trace,targetCalls,chosen,values[0]}
}
func SendTask(ready bool)int{
 output:=make(chan int,1);if !ready{output<-8}
 task:=make(chan int,1);go func(){task<-7}()
 select{case output<- <-task:default:};return <-output
}
func ReceiveTask(ready bool)int{
 input:=make(chan int,1);if ready{input<-7}
 task:=make(chan chan int,1);go func(){task<-input}()
 select{default:return 8;case n:=<- (<-task):return n}
}`
	tests := `package selectflow_test
import("reflect";"testing";g "select-flow.test";r "select-flow.test/reference")
func TestSelect(t *testing.T){for _,ready:=range []bool{false,true}{
 if got,want:=g.Run(ready),r.Run(ready);!reflect.DeepEqual(got,want){t.Fatal(ready,got,want)}
 if got,want:=g.SendTask(ready),r.SendTask(ready);got!=want{t.Fatal(ready,got,want)}
 if got,want:=g.ReceiveTask(ready),r.ReceiveTask(ready);got!=want{t.Fatal(ready,got,want)}
}}`
	runGeneratedGoDifferentialTest(t, root, "select-flow.test", generated, reference, tests)
}
