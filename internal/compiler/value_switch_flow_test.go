package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValueSwitchFlowMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `export function Trace(tag:int):int[]{
 let trace=0;let result=0;
 const mark=(digit:int):int=>{trace=trace*10+digit;return digit;};
 switch(tag){
  default{result=9;fallthrough;}
  case mark(1),mark(2){result=result*10+1;fallthrough;}
  case mark(3){result=result*10+2;}
  case mark(4){result=3;}
 }
 return [trace,result];
}
export function ColonTrace(tag:int):int[]{
 let trace=0
 let result=0
 const mark=(digit:int):int=>{trace=trace*10+digit;return digit;}
 switch(tag){
  case mark(1):
  case mark(2):
   result=1
  default:
   result=result*10+9
  case mark(3):
   if(tag==1){break}
   for(;;){ {goto ready; ready: result=result*10+3;} break; }
  case mark(4):
   const n=4
   result=result*10+n
 }
 return [trace,result]
}
export function ColonReturn(tag:int):int{
 switch(tag){case 0: case 1: return 1; default: return 2;}
}
function work():int{return 1;}
export function AwaitCase(tag:int):int{
 const task=go work();
 switch(tag){default{return 8;}case await task{return 1;}}
}
export function AwaitFallthrough(tag:int):int{
 const task=go work();let result=0;
 switch(tag){case 0{const n=await task;result=n;fallthrough;}case await task{result+=2;}default{result=3;}}
 return result;
}
export function AwaitUnmatched(tag:int):int{
 const task=go work();let result=0;
 switch(tag){case await task{result=1;}}
 return result;
}
export function ColonAwait(tag:int):int{
 const task=go work();let result=0;
 switch(tag){case 0: result=await task; case await task: result+=2;break;default:result=3;}
 return result;
}`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "switchflow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Trace(tag int)[]int{
 trace,result:=0,0;mark:=func(digit int)int{trace=trace*10+digit;return digit}
 switch tag{default:result=9;fallthrough;case mark(1),mark(2):result=result*10+1;fallthrough;case mark(3):result=result*10+2;case mark(4):result=3}
 return []int{trace,result}
}
func ColonTrace(tag int)[]int{
 trace,result:=0,0
 mark:=func(digit int)int{trace=trace*10+digit;return digit}
 switch tag {
 case mark(1):
  fallthrough
 case mark(2):
  result=1
  fallthrough
 default:
  result=result*10+9
  fallthrough
 case mark(3):
  if tag==1 {break}
  result=result*10+3
  fallthrough
 case mark(4):
  result=result*10+4
 }
 return []int{trace,result}
}
func ColonReturn(tag int)int{switch tag{case 0,1:return 1;default:return 2}}
func start()<-chan int{task:=make(chan int,1);go func(){task<-1}();return task}
func AwaitCase(tag int)int{task:=start();switch tag{default:return 8;case <-task:return 1}}
func AwaitFallthrough(tag int)int{task:=start();result:=0;switch tag{case 0:result=<-task;fallthrough;case <-task:result+=2;default:result=3};return result}
func AwaitUnmatched(tag int)int{task:=start();result:=0;switch tag{case <-task:result=1};return result}
func ColonAwait(tag int)int{task:=start();result:=0;switch tag{case 0:result=<-task;fallthrough;case <-task:result+=2;default:result=3};return result}
`
	tests := `package switchflow_test
import("reflect";"testing";g "switch-flow.test";r "switch-flow.test/reference")
func TestSwitch(t *testing.T){for _,tag:=range []int{-1,0,1,2,3,4,5}{
 if got,want:=g.Trace(tag),r.Trace(tag);!reflect.DeepEqual(got,want){t.Fatal(tag,got,want)}
 if got,want:=g.ColonTrace(tag),r.ColonTrace(tag);!reflect.DeepEqual(got,want){t.Fatal("colon",tag,got,want)}
 if got,want:=g.ColonReturn(tag),r.ColonReturn(tag);got!=want{t.Fatal("return",tag,got,want)}
 if got,want:=g.AwaitCase(tag),r.AwaitCase(tag);got!=want{t.Fatal(tag,got,want)}
 if got,want:=g.AwaitFallthrough(tag),r.AwaitFallthrough(tag);got!=want{t.Fatal(tag,got,want)}
 if got,want:=g.AwaitUnmatched(tag),r.AwaitUnmatched(tag);got!=want{t.Fatal(tag,got,want)}
 if got,want:=g.ColonAwait(tag),r.ColonAwait(tag);got!=want{t.Fatal("await",tag,got,want)}
}}`
	runGeneratedGoDifferentialTest(t, root, "switch-flow.test", generated, reference, tests)
}

func TestColonSwitchStopsAtUnboundedLoop(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "entry.km")
	input := `function run(tag: int): void {
  switch (tag) {
    case 0:
      for (;;) {}
    default:
      return
  }
}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "switchflow")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if strings.Contains(string(generated), "fallthrough") {
		t.Fatalf("emitted unreachable fallthrough after an unbounded loop:\n%s", generated)
	}
}
