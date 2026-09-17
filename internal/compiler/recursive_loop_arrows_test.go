package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecursiveLoopArrowsPipeline(t *testing.T) {
	t.Parallel()
	for index, input := range []string{
		`function run():void{for(const f=():int=>f();false;){}}`,
		`function run():int{for(const f=(n:int):int=>{if(n<1){return 1;}return n*f(n-1);};f(0)>0;){return f(5);}return 0;}`,
		`function run():int{for(const f:(n:int)=>int=(n)=>{if(n<1){return 1;}return f(n-1);}; ;){return f(2);}return 0;}`,
		`alias Callback=(n:int)=>int;function run():int{for(const f:Callback=(n)=>{if(n<1){return 1;}return f(n-1);}; ;){return f(2);}return 0;}`,
		`type Callback=distinct (n:int)=>int;function run():int{for(const f:Callback=(n)=>{if(n<1){return 1;}return f(n-1);}; ;){return f(2);}return 0;}`,
		`function run<T>(value:T):T{for(const f=(n:int):T=>{if(n<1){return value;}return f(n-1);}; ;){return f(2);}return value;}`,
		`class C{constructor(public value:int){}public function run():int{for(const f=(n:int):int=>{if(n<1){return this.value;}return f(n-1);}; ;){return f(2);}return 0;}}`,
		`import go http from "net/http";function run():void{for(const f:http.HandlerFunc=(w,r)=>{if(r.Method=="again"){f(w,r);}};false;){}}`,
		`function run():void{let i=0;outer:for(let f=():int=>f();i<2;i++){for(const f=():int=>f();false;){break;}continue outer;}}`,
		`function run():int{for(let f=():int=>{f=():int=>7;return f();}; ;){const p=&f;return (*p)();}return 0;}`,
		`function run():int{try{for(const f=(n:int):int=>{if(n<1){return 1;}return f(n-1);}; ;){return f(2);}}finally{}return 0;}`,
		`function run():int{for(const f=(n:int):int=>{if(n<1){return 1;}return f(n-1);};f(0)>0;){const f="shadow";return len(f);}return 0;}`,
	} {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			t.Log(input)
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "loop.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestRecursiveLoopArrowDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`function run():void{for(const f=()=>f();false;){}}`, "needs an explicit return type"},
		{`function run():void{for(const f=():int=>f();false;f=():int=>1){}}`, "cannot assign to const"},
		{`function run():void{for(const f=(n:int):int=>f("bad");false;){}}`, "cannot use string"},
		{`function run():void{for(const f=():int=>f();false;){}f();}`, "undefined"},
		{`function run():void{for(const f=():int=>f();false;){const f=1;f();}}`, "not callable"},
	} {
		path := filepath.Join(t.TempDir(), "invalid.km")
		result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: test.input})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.Diagnostics {
			found = found || strings.Contains(d.Message, test.want)
		}
		if !found {
			t.Fatalf("%s: %v, want %s", test.input, result.Diagnostics, test.want)
		}
	}
}

func TestRecursiveLoopArrowLabelCollision(t *testing.T) {
	t.Parallel()
	input := `function run():void{goto loop;loop:for(const f=():int=>f();false;){break loop;}`
	name := fmt.Sprintf("__kinmokusei_loop_label_%d", strings.Index(input, "loop:"))
	input += fmt.Sprintf("goto %s;%s:{} }", name, name)
	if _, accepted, err := compilePipelineProperty(input); err != nil || !accepted {
		t.Fatalf("accepted=%v err=%v source=%s", accepted, err, input)
	}
}

func TestRecursiveLoopArrowsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"library.km": `alias Read=()=>int;alias Callback=(n:int)=>int;
export function Factorial(n:int):int{
  for(const f=(v:int):int=>{if(v<=1){return 1;}return v*f(v-1);}; ;){return f(n);}return 0;
}
export function Fresh():int[]{
  let callbacks:Read[]=[];
  let originals:Callback[]=[];
  let i=0;
  for(let f=(n:int):int=>{if(n<=0){return 1;}return f(n-1)+1;};i<3;i++){
    originals=append(originals,f);
    callbacks=append(callbacks,()=>f(1));
    const value=(i+1)*10;
    f=(n:int):int=>value;
  }
  return [callbacks[0](),callbacks[1](),callbacks[2](),originals[0](1),originals[1](1)];
}
function next(previous:(n:int)=>int):(n:int)=>int{return (n)=>previous(n)+10;}
export function Post():int[]{
  let callbacks:Read[]=[];
  let i=0;
  for(let f=(n:int):int=>{if(n<=0){return 1;}return f(n-1)+1;};i<3;f=next(f)){
    callbacks=append(callbacks,()=>f(1));i++;continue;
  }
  return [callbacks[0](),callbacks[1](),callbacks[2]()];
}
export function Condition(limit:int):int{
  let calls=0;let posts=0;
  for(const f=(n:int):int=>{calls++;if(n==0){return calls;}return f(n-1);};f(1)<limit;posts++){continue;}
  return calls*100+posts;
}
export function Labels():int{
  let i=0;let total=0;
  outer:for(const f=(n:int):int=>{if(n==0){return 1;}return f(n-1)+1;};i<5;i++){
    for(const inner=():int=>inner(); ;){if(i==1){continue outer;}if(i==3){break outer;}break;}
    total+=f(2);
  }
  return total*10+i;
}
export function Restart():int{
  let i=0;let saved:Callback[]=[];
  again:for(let f=(n:int):int=>{if(n==0){return 1;}return f(n-1)+1;}; ;){
    saved=append(saved,f);const value=10+i;f=(n:int):int=>value;i++;
    if(i<2){goto again;}break;
  }
  return saved[0](1)*100+saved[1](1);
}
export function MixedLabels():int{
  let i=0;let trace=0;goto again;
  again:for(let f=(n:int):int=>{if(n==0){return 1;}return f(n-1)+1;};i<5;trace+=100){
    const nested=():int=>{again:for(let f=():int=>f(); ;){break again;}return 7;};
    trace+=f(1)+nested();i++;
    if(i==1){goto again;}
    if(i==2){continue again;}
    break again;
  }
  return trace+i;
}
class Hidden{constructor(public value:int){}}
export function Generic<T>(value:T):T{for(const f=(n:int):T=>{if(n==0){return value;}return f(n-1);}; ;){return f(2);}return value;}
export function Object():Hidden{return Generic(new Hidden(42));}
export function Finally():int{let result=0;try{for(let f=():int=>{f=():int=>7;return f();}; ;){result=f();break;}}finally{result+=10;}return result;}`,
		"main.km": `import { Factorial, Fresh, Post, Condition, Labels, Restart, MixedLabels, Object, Finally } from "./library";
export function Run(n:int):int{return Factorial(n);}
export function Values():int[]{return append(Fresh(),Post()...);}
export function Control(n:int):int{return Condition(n)+Labels()+Restart()+MixedLabels()+Object().value+Finally();}`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "main.km")}, "looparrows")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("%v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "recursive-loop-arrows.test", generated,
		`package reference
func Run(n int)int{if n<=1{return 1};return n*Run(n-1)}
// Model iteration storage explicitly, independently of the emitted for loop.
func Values()[]int{
  var first func(int)int
  first=func(n int)int{if n<=0{return 1};return first(n-1)+1}
  current:=&first
  var callbacks []func()int
  var originals []func(int)int
  for i:=0;i<3;i++{
    slot:=current;originals=append(originals,*slot);callbacks=append(callbacks,func()int{return (*slot)(1)})
    value:=(i+1)*10;*slot=func(int)int{return value}
    next:=new(func(int)int);*next=*slot;current=next
  }
  result:=[]int{callbacks[0](),callbacks[1](),callbacks[2](),originals[0](1),originals[1](1)}
  var recursive func(int)int
  recursive=func(n int)int{if n<=0{return 1};return recursive(n-1)+1}
  for f,i:=recursive,0;i<3;f=addTen(f){value:=f;callbacks=append(callbacks,func()int{return value(1)});i++}
  for _,callback:=range callbacks[3:]{result=append(result,callback())}
  return result
}
func addTen(f func(int)int)func(int)int{return func(n int)int{return f(n)+10}}
func Control(limit int)int{calls,posts:=0,0;for{calls+=2;if calls>=limit{break};posts++};return calls*100+posts+63+1112+130+42+17}`,
		`package looparrows
import("testing";"reflect";reference "recursive-loop-arrows.test/reference")
func TestBehavior(t *testing.T){
  for _,n:=range []int{-1,0,1,2,5,8}{if got,want:=Run(n),reference.Run(n);got!=want{t.Errorf("Run(%d)=%d want %d",n,got,want)}}
  if got,want:=Values(),reference.Values();!reflect.DeepEqual(got,want){t.Errorf("Values=%v want %v",got,want)}
  for _,n:=range []int{-1,0,2,3,6,9}{if got,want:=Control(n),reference.Control(n);got!=want{t.Errorf("Control(%d)=%d want %d",n,got,want)}}
}`)
}
