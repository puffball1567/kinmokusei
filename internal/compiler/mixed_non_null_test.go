package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMixedNonNullBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "mixed.km")
	input := `import go value from "reflect";
export function Reflected():int{return value.TypeOf(1).NumMethod();}
export alias Callback=()=>int;
export class Item{public n:int=7;}
constraint Mixed=Callback|int;
constraint Ref=Callback|*int;
constraint ClassOrNumber=Item|int;
let state=0;
export function Reset():void{state=0;}
export function State():int{return state;}
export function Entry<T extends Mixed>(value:T):int{state++;return 7;}
export function ClassEntry<T extends ClassOrNumber>(value:T):int{state++;return 7;}
export function Rest<T extends Mixed>(...values:T[]):int{state++;return len(values);}
export function Arrow<T extends Mixed>():(value:T)=>int{return(value)=>{state++;return 7;};}
export function Local<T extends Mixed>(read:()=>T):T{const value:T=read();state++;return value;}
export function Publish<T extends Mixed>(read:()=>T):T{return read();}
export function Convert<T extends Mixed>(read:()=>T):T{const value=T(read());state++;return value;}
export function Pair<T extends Mixed>(read:()=>(T,int)):(T,int){return read();}
export function Split<T extends Mixed>(read:()=>(T,int)):T{const[value,_]=read();state++;return value;}
export function Lookup<T extends Mixed>(m:Map<int,T>,fallback:T):T{const[value,present]=m[0];if(!present){return fallback;}state++;return value;}
export function Receive<T extends Mixed>(ch:GoChannel<T>,fallback:T):T{const[value,present]=<-ch;if(!present){return fallback;}state++;return value;}
export function Selected<T extends Mixed>(ch:GoChannel<T>,fallback:T):T{select{case const[value,present]=<-ch{if(!present){return fallback;}state++;return value;}}}
export function ResultForward<T extends Mixed>(read:()=>Result<T>):Result<T>{return read();}
export function Propagate<T extends Mixed>(read:()=>Result<T>):Result<T>{const value=read()?;state++;return ok(value);}
export function ResultSplit<T extends Mixed>(read:()=>Result<T>,fallback:T):T{const[value,err]=read();if(err!=nil){return fallback;}state++;return value;}
export function Coalesce<T extends Ref>(read:()=>T,fallback:T):T{const value:T|null=T(read());return value??fallback;}
export function Assert<T extends Ref>(read:()=>interface{},fallback:T):T{const[value,present]=read() as? T;if(!present){return fallback;}return value;}
export function Forced<T extends Ref>(read:()=>interface{}):T{return read() as! T;}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, ds, err := EmitGo([]string{path}, "mixed")
	if err != nil || len(ds) != 0 {
		t.Fatalf("emit: %v %v", err, ds)
	}
	mapped, ds, err := EmitGoWithSourceMap([]string{path}, "mixed")
	if err != nil || len(ds) != 0 || !bytes.Equal(generated, mapped.GoSource) {
		t.Fatalf("map: %v %v", err, ds)
	}
	reference := `package reference
func Accept(valid bool)int{if !valid{panic("non-null reference")};return 7}
func Optional(valid,present bool)int{if !present{return 9};return Accept(valid)}
`
	comparison := `package mixed_test
import("errors";"fmt";"strings";"testing";g "mixed-non-null.test";r "mixed-non-null.test/reference")
func capture(call func())(result any){defer func(){result=recover()}();call();return nil}
func TestMixed(t *testing.T){
 if g.Reflected()!=0{t.Fatal("reflect alias")}
 nilRead:=func()func()int{return nil};nilPair:=func()(func()int,int){return nil,0};nilResult:=func()(func()int,error){return nil,nil}
 good:=func()int{return 9}
 for name,call:=range map[string]func(){
 "entry":func(){g.Entry[func()int](nil)},"class":func(){g.ClassEntry[*g.Item](nil)},"rest":func(){g.Rest[func()int](good,nil)},"arrow":func(){g.Arrow[func()int]()(nil)},
 "local":func(){g.Local(nilRead)},"return":func(){g.Publish(nilRead)},"conversion":func(){g.Convert(nilRead)},"forward tuple":func(){g.Pair(nilPair)},"split":func(){g.Split(nilPair)},
 "lookup":func(){g.Lookup(map[int]func()int{0:nil},good)},"receive":func(){ch:=make(chan func()int,1);ch<-nil;g.Receive(ch,good)},"select":func(){ch:=make(chan func()int,1);ch<-nil;g.Selected(ch,good)},
 "result forward":func(){g.ResultForward(nilResult)},"propagate":func(){g.Propagate(nilResult)},"result split":func(){g.ResultSplit(nilResult,good)},
 "forced assertion":func(){g.Forced[func()int](func()any{return (func()int)(nil)})},
 }{t.Run(name,func(t *testing.T){g.Reset();got:=capture(call);want:=capture(func(){r.Accept(false)});if (got==nil)!=(want==nil)||!strings.Contains(fmt.Sprint(got),"non-null")||g.State()!=0{t.Fatal(got,want,g.State())}})}
 if g.Entry(0)!=r.Accept(true)||g.ClassEntry(0)!=7||g.Rest[int](0,1)!=2||g.Arrow[int]()(0)!=7{t.Fatal("scalar")}
 if g.Local(func()int{return 0})!=0||g.Publish(func()int{return 0})!=0||g.Convert(func()int{return 0})!=0{t.Fatal("scalar transfers")}
 for _,p:=range []bool{false,true}{m:=map[int]int{};if p{m[0]=0};want:=9;if p{want=0};if g.Lookup(m,9)!=want{t.Fatal("lookup scalar")}}
 if got:=g.Lookup(map[int]func()int{},good);got()!=9{t.Fatal("missing lookup")}
 for _,call:=range []func(chan func()int,func()int)func()int{g.Receive[func()int],g.Selected[func()int]}{ch:=make(chan func()int);close(ch);if call(ch,good)()!=9{t.Fatal("closed receive")}}
 failure:=errors.New("failed");failed:=func()(func()int,error){return nil,failure}
 for _,call:=range []func(func()(func()int,error))(func()int,error){g.ResultForward[func()int],g.Propagate[func()int]}{value,err:=call(failed);if value!=nil||err!=failure{t.Fatal(value!=nil,err)}}
 if g.ResultSplit(failed,good)()!=9{t.Fatal("failure fallback")}
 n:=7;pointer:=&n;if g.Coalesce(func()*int{return nil},pointer)!=pointer{t.Fatal("raw nil lost nullable state")}
 if g.Assert[*int](func()any{return (*int)(nil)},pointer)!=nil{t.Fatal("raw checked assertion")}
 if g.Forced[*int](func()any{return (*int)(nil)})!=nil{t.Fatal("raw forced assertion")}
 if g.Assert[func()int](func()any{return (func()int)(nil)},good)()!=9{t.Fatal("native checked assertion")}
}
`
	runGeneratedGoDifferentialTest(t, root, "mixed-non-null.test", generated, reference, comparison)
}
