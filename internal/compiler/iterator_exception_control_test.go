package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIteratorExceptionControlMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	library := `import go errors from "errors";
export function digit(log:*int,value:int):void{*log=*log*10+value;}
export function sequence(log:*int,mode:int):(yield:(value:int)=>boolean)=>void{
 return (yield:(value:int)=>boolean):void=>{
  try{if(!yield(1)){return;}if(!yield(2)){return;}}
  catch(_:error){digit(log,9);}
  finally{digit(log,2);if(mode===1){return;}if(mode===2){throw errors.New("iterator");}}
 };
}
export function nested(log:*int):(yield:(value:int)=>boolean)=>void{
 return (yield:(value:int)=>boolean):void=>{
  try{for(const value of sequence(log,0)){if(!yield(value)){return;}}}
  catch(_:error){digit(log,8);}
  finally{digit(log,5);}
 };
}
type Yield=distinct (value:int)=>boolean;
type Named=distinct (yield:Yield)=>void;
export function named(log:*int):Named{
 return (yield:Yield):void=>{try{if(!yield(1)){return;}}finally{digit(log,2);}};
}
export function zero(log:*int):(yield:()=>boolean)=>void{
 return (yield:()=>boolean):void=>{try{if(!yield()){return;}}finally{digit(log,2);}};
}
export function pair(log:*int):(yield:(key:int,value:int)=>boolean)=>void{
 return (yield:(key:int,value:int)=>boolean):void=>{try{if(!yield(4,6)){return;}}finally{digit(log,2);}};
}
`
	input := `import {digit,sequence,nested,named,zero,pair} from "./iterators";
import go slices from "slices";
function find(log:*int,mode:int):int{
 try{for(const value of sequence(log,mode)){try{return value*100;}finally{digit(log,1);}}return -1;}
 catch(_:error){digit(log,4);return 300;}
 finally{digit(log,3);}
}
export function ReturnCase(mode:int):int[]{let log=0;const result=find(&log,mode);return [result,log];}
export function ThrowCase(log:*int,err:error):error{
 try{for(const value of sequence(log,0)){try{throw err;}finally{digit(log,1);}}}
 catch(caught:error){digit(log,4);return caught;}
 finally{digit(log,3);}
 return nil;
}
function resultValue(err:error):Result<int>{if(err!==nil){return fail(err);}return ok(7);}
export function ResultCase(log:*int,err:error):Result<int>{
 try{for(const value of sequence(log,0)){try{const result=resultValue(err)?;return ok(result+value);}finally{digit(log,1);}}return ok(-1);}
 finally{digit(log,3);}
}
export function MultipleCase(log:*int):(int,*int|null){
 try{for(const value of nested(log)){try{return value*100,null;}finally{digit(log,1);}}return -1,null;}
 finally{digit(log,3);}
}
export function ZeroCase(log:*int):int{try{for(const _ of zero(log)){try{return 42;}finally{digit(log,1);}}return -1;}finally{digit(log,3);}}
export function PairCase(log:*int):int{try{for(const [key,value] of pair(log)){try{return key*10+value;}finally{digit(log,1);}}return -1;}finally{digit(log,3);}}
export function NamedCase(log:*int):int{try{for(const value of named(log)){try{return value*100;}finally{digit(log,1);}}return -1;}finally{digit(log,3);}}
export function RuntimeCase(log:*int,callback:()=>void):void{try{for(const value of sequence(log,0)){callback();}}catch(_:error){digit(log,4);}finally{digit(log,3);}}
export function DirectCase(log:*int,err:error):void{sequence(log,0)((value:int):boolean=>{throw err;});}
function plain(yield:(value:int)=>boolean):void{if(!yield(1)){return;}}
export function PlainRuntime(callback:()=>void):void{for(const value of plain){callback();}}
function bad(yield:(value:int)=>boolean):void{yield(1);yield(2);}
export function ProtocolCase():void{for(const value of bad){break;}}
export function StandardCase(log:*int):int{try{for(const value of slices.Values([1,2])){try{return value*100;}finally{digit(log,1);}}return -1;}finally{digit(log,3);}}
constraint SourceIterator=(yield:(value:int)=>boolean)=>void;
function consume<Source extends SourceIterator>(source:Source,log:*int):int{try{for(const value of source){try{return value*100;}finally{digit(log,1);}}return -1;}finally{digit(log,3);}}
export function GenericCase(log:*int):int{return consume(sequence(log,0),log);}
`
	for name, contents := range map[string]string{"iterators.km": library, "entry.km": input} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "iteratorcontrol")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "iteratorcontrol")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("errors";"slices")
func digit(log *int,value int){*log=*log*10+value}
func sequence(log *int,mode int)func(func(int)bool){return func(yield func(int)bool){defer func(){digit(log,2);if mode==2{panic(errors.New("iterator"))}}();if !yield(1){return};if !yield(2){return}}}
func nested(log *int)func(func(int)bool){return func(yield func(int)bool){defer digit(log,5);for value:=range sequence(log,0){if !yield(value){return}}}}
func find(log *int,mode int)(result int){defer digit(log,3);defer func(){if p:=recover();p!=nil{digit(log,4);result=300}}();for value:=range sequence(log,mode){digit(log,1);return value*100};return -1}
func ReturnCase(mode int)[]int{log:=0;result:=find(&log,mode);return []int{result,log}}
func ThrowCase(log *int,err error)(result error){defer digit(log,3);defer func(){if p:=recover();p!=nil{digit(log,4);result=p.(error)}}();for range sequence(log,0){func(){defer digit(log,1);panic(err)}()};return nil}
func ResultCase(log *int,err error)(int,error){defer digit(log,3);for value:=range sequence(log,0){digit(log,1);if err!=nil{return 0,err};return 7+value,nil};return -1,nil}
func MultipleCase(log *int)(int,*int){defer digit(log,3);for value:=range nested(log){digit(log,1);return value*100,nil};return -1,nil}
func ZeroCase(log *int)int{defer digit(log,3);ticks:=func(yield func()bool){defer digit(log,2);if !yield(){return}};for range ticks{digit(log,1);return 42};return -1}
func PairCase(log *int)int{defer digit(log,3);pairs:=func(yield func(int,int)bool){defer digit(log,2);if !yield(4,6){return}};for key,value:=range pairs{digit(log,1);return key*10+value};return -1}
type yieldFunc func(int)bool
type namedIterator func(yieldFunc)
func NamedCase(log *int)int{defer digit(log,3);var seq namedIterator=func(yield yieldFunc){defer digit(log,2);if !yield(1){return}};for value:=range seq{digit(log,1);return value*100};return -1}
func RuntimeCase(log *int,callback func()){defer digit(log,3);for range sequence(log,0){callback()}}
func DirectCase(log *int,err error){defer digit(log,2);defer func(){if recover()!=nil{digit(log,9)}}();panic(err)}
func PlainRuntime(callback func()){plain:=func(yield func(int)bool){if !yield(1){return}};for range plain{callback()}}
func ProtocolCase(){bad:=func(yield func(int)bool){yield(1);yield(2)};for range bad{break}}
func StandardCase(log *int)int{defer digit(log,3);for value:=range slices.Values([]int{1,2}){digit(log,1);return value*100};return -1}
func consume[Source ~func(func(int)bool)](source Source,log *int)int{defer digit(log,3);for value:=range source{digit(log,1);return value*100};return -1}
func GenericCase(log *int)int{return consume(sequence(log,0),log)}
`
	comparison := `package iteratorcontrol_test
import("errors";"reflect";"runtime";"testing";g "iterator-exception-control.test";r "iterator-exception-control.test/reference")
func capture(call func())(result any){defer func(){result=recover()}();call();return nil}
func captureCompleted(call func())(completed bool,value any){defer func(){value=recover()}();call();return true,nil}
func TestControl(t *testing.T){
 for _,mode:=range []int{0,1,2}{if got,want:=g.ReturnCase(mode),r.ReturnCase(mode);!reflect.DeepEqual(got,want){t.Fatalf("return mode %d: %v != %v",mode,got,want)}}
 err:=errors.New("body")
 gl,rl:=0,0;if got,want:=g.ThrowCase(&gl,err),r.ThrowCase(&rl,err);got!=err||want!=err||gl!=rl{t.Fatal("throw",got,want,gl,rl)}
 for _,failure:=range []error{nil,err}{gl,rl=0,0;gv,ge:=g.ResultCase(&gl,failure);rv,re:=r.ResultCase(&rl,failure);if gv!=rv||ge!=re||gl!=rl{t.Fatal("Result",gv,ge,rv,re,gl,rl)}}
 gl,rl=0,0;gv,gp:=g.MultipleCase(&gl);rv,rp:=r.MultipleCase(&rl);if gv!=rv||gp!=nil||rp!=nil||gl!=rl{t.Fatal("nested multiple",gv,gp,rv,rp,gl,rl)}
 for name,pair:=range map[string][2]func(*int)int{"zero":{g.ZeroCase,r.ZeroCase},"pair":{g.PairCase,r.PairCase},"named":{g.NamedCase,r.NamedCase},"standard":{g.StandardCase,r.StandardCase},"generic":{g.GenericCase,r.GenericCase}}{gl,rl=0,0;got,want:=pair[0](&gl),pair[1](&rl);if got!=want||gl!=rl{t.Fatal(name,got,want,gl,rl)}}
 token:=&struct{value int}{7};gl,rl=0,0;gpv:=capture(func(){g.RuntimeCase(&gl,func(){panic(token)})});rpv:=capture(func(){r.RuntimeCase(&rl,func(){panic(token)})});if gpv!=token||rpv!=token||gl!=rl{t.Fatal("ordinary panic",gpv,rpv,gl,rl)}
 for _,call:=range []func(*int,func()){g.RuntimeCase,r.RuntimeCase}{done:=make(chan struct{});go func(){defer close(done);trace:=0;call(&trace,runtime.Goexit);t.Error("Goexit returned")}();<-done}
 gl,rl=0,0;g.DirectCase(&gl,err);r.DirectCase(&rl,err);if gl!=rl{t.Fatal("direct callback catch",gl,rl)}
 gc,gpanic:=captureCompleted(func(){g.PlainRuntime(func(){panic(nil)})});rc,rpanic:=captureCompleted(func(){r.PlainRuntime(func(){panic(nil)})});if gc||rc||reflect.TypeOf(gpanic)!=reflect.TypeOf(rpanic){t.Fatal("nil panic",gc,gpanic,rc,rpanic)}
 if gp,rp:=capture(g.ProtocolCase),capture(r.ProtocolCase);gp==nil||rp==nil||reflect.TypeOf(gp)!=reflect.TypeOf(rp){t.Fatal("yield protocol",gp,rp)}
}
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module iterator-exception-control.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"0", "1"} {
		t.Run("panicnil="+policy, func(t *testing.T) {
			runGeneratedGoDifferentialTestInExistingModule(t, root, "iterator-exception-control.test", generated, reference, comparison, nil, []string{"GODEBUG=panicnil=" + policy})
		})
	}
}

func TestPlainIteratorRetainsDirectGoRange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "plain.km")
	if err := os.WriteFile(path, []byte(`function seq(yield:(value:int)=>boolean):void{if(!yield(7)){return;}}export function First():int{for(const value of seq){return value;}return 0;}`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "plain")
	if err != nil || len(diagnostics) != 0 || !bytes.Contains(generated, []byte("range seq")) || bytes.Contains(generated, []byte("__kinmokuseiIterator")) {
		t.Fatalf("plain iterator lowering err=%v diagnostics=%v", err, diagnostics)
	}
}
