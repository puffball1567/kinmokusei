package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDeferredExceptionUnwindingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `import go slices from "slices";
export function RuntimeCase(event:(value:int)=>void,body:()=>void,oldest:()=>void,newest:()=>void,count:int):int {
 defer oldest();
 try {
  for(let i=0;i<count;i++){defer event(i);}
  defer newest();body();return 17;
 } finally {event(-1);}
}
export function Captures(first:(value:int)=>void,last:(value:int)=>void,arg:()=>int):void {
 let fn=first;
 try {defer fn(arg());fn=last;defer fn(arg());}
 finally {first(-1);}
 first(-2);
}
function generic<T>(event:(value:T)=>void,value:T):void{event(value);}
function pair():(int,int){return 4,5;}
export function Arguments(event:(value:int)=>void,variadic:(...values:int[])=>void,tuple:(a:int,b:int)=>void,narrow:(value:byte)=>void,n:uint):void {
 const values=[1,2];
 try{
  defer generic(event,3);defer generic<byte>(narrow,1<<n);
  defer tuple(pair());defer variadic(values...);defer narrow(1<<n);
  values[0]=9;
 }finally{event(-1);}
 event(-2);
}
export function Builtins(m:Map<int,int>,ch:GoChannel<int>):void{
 try {defer delete(m,1);defer closeGoChannel(ch);}finally{m[2]=2;}
 m[3]=3;
}
export function Many(event:(value:int)=>void,count:int):void{
 try{for(let i=0;i<count;i++){defer event(i);}}finally{event(-1);}
}
export class Recorder {
 constructor(event:(value:int)=>void){try{defer event(1);event(2);}finally{event(3);}event(4);}
 public function run(event:(value:int)=>void):(int,int){try{defer event(5);return 6,7;}finally{event(8);}}
}
export function Returns(event:(value:int)=>void,failure:error):Result<int>{
 try{defer event(1);if(failure!==nil){return fail(failure);}return ok(42);}finally{event(2);}
}
export function Override(event:(value:int)=>void):int{
 try{defer event(1);return 2;}finally{defer event(3);return 4;}
}
export function Iterator(event:(value:int)=>void):int{
 try{for(const value of slices.Values([1,2,3])){defer event(value);if(value===2){return value;}}}finally{event(4);}
 return -1;
}
`
	path := filepath.Join(root, "unwind.km")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "defers")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "defers")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "slices"
func RuntimeCase(event func(int),body,oldest,newest func(),count int)int{
 defer oldest();for i:=0;i<count;i++{defer event(i)};defer newest()
 func(){defer event(-1);body()}();return 17
}
func Captures(first,last func(int),arg func()int){fn:=first;defer fn(arg());fn=last;defer fn(arg());first(-1);first(-2)}
func generic[T any](event func(T),value T){event(value)}
func pair()(int,int){return 4,5}
func Arguments(event func(int),variadic func(...int),tuple func(int,int),narrow func(byte),n uint){
 values:=[]int{1,2};defer generic(event,3);defer generic[byte](narrow,1<<n);defer tuple(pair());defer variadic(values...);defer narrow(1<<n);values[0]=9;event(-1);event(-2)
}
func Builtins(m map[int]int,ch chan int){defer delete(m,1);defer close(ch);m[2]=2;m[3]=3}
func Many(event func(int),count int){for i:=0;i<count;i++{defer event(i)};event(-1)}
type Recorder struct{}
func NewRecorder(event func(int))*Recorder{defer event(1);event(2);event(3);event(4);return &Recorder{}}
func(this *Recorder)Run(event func(int))(int,int){defer event(5);event(8);return 6,7}
func Returns(event func(int),failure error)(int,error){defer event(1);event(2);if failure!=nil{return 0,failure};return 42,nil}
func Override(event func(int))int{defer event(1);defer event(3);return 4}
func Iterator(event func(int))int{for value:=range slices.Values([]int{1,2,3}){defer event(value);if value==2{event(4);return value}};event(4);return -1}
`
	comparison := `package defers_test
import("errors";"fmt";"reflect";"runtime";"testing";g "defer-unwind.test";r "defer-unwind.test/reference")
func run(call func(func(int),func(),func(),func(),int)int,mode,count int)(trace []string,result int,completed bool,escaped any){
 defer func(){escaped=recover()}()
 emit:=func(i int){trace=append(trace,fmt.Sprint(i))}
 token:=errors.New("body")
 body:=func(){if mode==1||mode==2||mode==4{panic(token)};if mode==3{panic(nil)}}
 oldest:=func(){p:=recover();trace=append(trace,fmt.Sprintf("recover:%T:%v:%t",p,p,p==token))}
 newest:=func(){trace=append(trace,"newest");if mode==2||mode==6{panic("replacement")};if mode==4{p:=recover();trace=append(trace,fmt.Sprintf("inner:%T:%v:%t",p,p,p==token))}}
 if mode==5{newest=nil}
 result=call(emit,body,oldest,newest,count);completed=true;return
}
func TestUnwind(t *testing.T){for _,n:=range []int{0,1,62,63,64,65,128,257}{for mode:=0;mode<7;mode++{gt,gv,gc,gp:=run(g.RuntimeCase,mode,n);rt,rv,rc,rp:=run(r.RuntimeCase,mode,n);if !reflect.DeepEqual(gt,rt)||gv!=rv||gc!=rc||fmt.Sprint(gp)!=fmt.Sprint(rp){t.Fatalf("mode=%d n=%d generated=%v,%v,%v,%v reference=%v,%v,%v,%v",mode,n,gt,gv,gc,gp,rt,rv,rc,rp)}}}}
func TestGoexit(t *testing.T){for _,inDefer:=range []bool{false,true}{traces:=[][]string{};for _,call:=range []func(func(int),func(),func(),func(),int)int{g.RuntimeCase,r.RuntimeCase}{trace:=[]string{};done:=make(chan struct{});go func(){defer close(done);defer func(){if p:=recover();p!=nil{trace=append(trace,fmt.Sprint(p))}}();event:=func(i int){trace=append(trace,fmt.Sprint(i))};body:=func(){if !inDefer{runtime.Goexit()}};oldest:=func(){p:=recover();trace=append(trace,fmt.Sprintf("recover:%T:%v",p,p))};newest:=func(){if inDefer{runtime.Goexit()}};call(event,body,oldest,newest,257);trace=append(trace,"returned")}();<-done;traces=append(traces,trace)};if !reflect.DeepEqual(traces[0],traces[1]){t.Fatalf("Goexit inDefer=%v generated=%v reference=%v",inDefer,traces[0],traces[1])}}}
func TestArguments(t *testing.T){for _,n:=range []uint{0,1,7,8,64}{traces:=[][]int{};for _,call:=range []func(func(int),func(...int),func(int,int),func(byte),uint){g.Arguments,r.Arguments}{trace:=[]int{};event:=func(i int){trace=append(trace,i)};call(event,func(v ...int){trace=append(trace,v...)},func(a,b int){event(a*10+b)},func(v byte){event(int(v))},n);traces=append(traces,trace)};if !reflect.DeepEqual(traces[0],traces[1]){t.Fatalf("arguments n=%d: %v != %v",n,traces[0],traces[1])}}}
func TestCaptures(t *testing.T){traces:=[][]int{};for _,call:=range []func(func(int),func(int),func()int){g.Captures,r.Captures}{trace:=[]int{};i:=0;call(func(v int){trace=append(trace,v)},func(v int){trace=append(trace,v*10)},func()int{i++;trace=append(trace,100+i);return i});traces=append(traces,trace)};if !reflect.DeepEqual(traces[0],traces[1]){t.Fatal(traces)}}
func TestMany(t *testing.T){traces:=[][]int{};for _,call:=range []func(func(int),int){g.Many,r.Many}{trace:=[]int{};call(func(i int){trace=append(trace,i);if i==9999{var pcs [1024]uintptr;if runtime.Callers(0,pcs[:])==len(pcs){t.Fatal("defer stack depth grows with registrations")}}},10000);traces=append(traces,trace)};if !reflect.DeepEqual(traces[0],traces[1]){t.Fatal("many defer order mismatch")}}
func TestUnrecoveredIdentity(t *testing.T){token:=&struct{n int}{42};for _,n:=range []int{0,65,257}{for _,call:=range []func(func(int),func(),func(),func(),int)int{g.RuntimeCase,r.RuntimeCase}{var recovered any;completed:=false;func(){defer func(){recovered=recover()}();call(func(int){},func(){panic(token)},func(){},func(){},n);completed=true}();if recovered!=token||completed{t.Fatal("panic identity",n,recovered,completed)}}}}
func TestBuiltins(t *testing.T){gm,rm:=map[int]int{1:1},map[int]int{1:1};gc,rc:=make(chan int),make(chan int);g.Builtins(gm,gc);r.Builtins(rm,rc);_,gok:=<-gc;_,rok:=<-rc;if !reflect.DeepEqual(gm,rm)||gok!=rok||gok{t.Fatal("builtins",gm,rm,gok,rok)}}
func TestReturns(t *testing.T){gt,rt:=[]int{},[]int{};ge:=func(i int){gt=append(gt,i)};re:=func(i int){rt=append(rt,i)};gv,gerr:=g.Returns(ge,nil);rv,rerr:=r.Returns(re,nil);if gv!=rv||gerr!=rerr||!reflect.DeepEqual(gt,rt){t.Fatal("Result",gv,gerr,rv,rerr,gt,rt)};failure:=errors.New("failed");gv,gerr=g.Returns(ge,failure);rv,rerr=r.Returns(re,failure);if gv!=rv||gerr!=rerr||!reflect.DeepEqual(gt,rt){t.Fatal("failed Result")};if g.Override(ge)!=r.Override(re)||g.Iterator(ge)!=r.Iterator(re)||!reflect.DeepEqual(gt,rt){t.Fatal("return override/iterator",gt,rt)};go1,go2:=g.NewRecorder(ge).Run(ge);ro1,ro2:=r.NewRecorder(re).Run(re);if go1!=ro1||go2!=ro2||!reflect.DeepEqual(gt,rt){t.Fatal("constructor/method",gt,rt)}}
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module defer-unwind.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"0", "1"} {
		t.Run("panicnil="+policy, func(t *testing.T) {
			runGeneratedGoDifferentialTestInExistingModule(t, root, "defer-unwind.test", generated, reference, comparison, nil, []string{"GODEBUG=panicnil=" + policy})
		})
	}
}

func TestPlainDeferRetainsDirectGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "plain.km")
	if err := os.WriteFile(path, []byte(`export function Run(callback:()=>void):void{defer callback();}`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "plain")
	if err != nil || len(diagnostics) != 0 || !bytes.Contains(generated, []byte("defer callback()")) || bytes.Contains(generated, []byte("__kinmokuseiDeferFrame")) || bytes.Contains(generated, []byte("__kinmokuseiSourceCallable")) {
		t.Fatalf("err=%v diagnostics=%v generated=%s", err, diagnostics, generated)
	}
}
