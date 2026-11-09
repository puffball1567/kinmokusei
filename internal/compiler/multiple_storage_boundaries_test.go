package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMultipleStorageBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module multiple-storage.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
import "time"
type hidden func()int
type Raw func()int
var calls,hits int
func Reset(){calls=0;hits=0}
func Counts()(int,int){return calls,hits}
func Hit(){hits++}
func Pair(a,b bool)(hidden,Raw,int){calls++;var x hidden;var y Raw;if a{x=func()int{return 7}};if b{y=func()int{return 8}};return x,y,9}
func PublicPair(a,b bool)(Raw,Raw,int){x,y,n:=Pair(a,b);return Raw(x),y,n}
func Plain(p bool)(func()int,int){calls++;var f func()int;if p{f=func()int{return 7}};return f,9}
func Flag(p bool)(Raw,bool){calls++;var f Raw;if p{f=func()int{return 7}};return f,false}
type Failure struct{}
func(*Failure)Error()string{return "failure"}
func Pointer()(*Failure,int){calls++;return nil,9}
func Interface(v error)(error,int){calls++;return v,9}
type privateTimed func(time.Duration)int
func Timed(p bool)(privateTimed,int){calls++;var f privateTimed;if p{f=func(v time.Duration)int{return int(v)}};return f,9}
`,
		"models.km": `import go clock from "time";
export alias Callback=()=>int;
export alias View=interface{Error():bstring;};
export alias Timed=(value:clock.Duration)=>int;
`,
		"entry.km": `import go native from "example.com/native";
import {Callback,View,Timed} from "./models";
import go clock from "strings";
export function Reset():void{native.Reset();First=()=>1;Second=()=>2;Number=3;}
export function Counts():int[]{const [calls,hits]=native.Counts();return [calls,hits];}
export function Forward(a:boolean,b:boolean):(Callback,Callback,int){return native.Pair(a,b);}
export const Arrow=(a:boolean,b:boolean):(Callback,Callback,int)=>native.Pair(a,b);
export const BlockArrow=(a:boolean,b:boolean):(Callback,Callback,int)=>{return native.Pair(a,b);};
export function Finally(a:boolean,b:boolean):(Callback,Callback,int){try{return native.Pair(a,b);}finally{native.Hit();}}
constraint Functions=()=>int;
export function Generic<T extends Functions>(p:boolean):(T,int){return native.Plain(p);}
export class Host{public function pair(a:boolean,b:boolean):(Callback,Callback,int){return native.Pair(a,b);}}
export let First:Callback=()=>1;
export let Second:Callback=()=>2;
export let Number:int=3;
export function Assign(a:boolean,b:boolean):void{[First,Second,Number]=native.Pair(a,b);}
export function Duplicate(a:boolean,b:boolean):void{[First,First,Number]=native.Pair(a,b);}
export function Post(a:boolean,b:boolean):void{for(let i=0;i<1;[First,Second,Number]=native.Pair(a,b)){i++;}}
export function IgnoreFirst(a:boolean,b:boolean):int{let value:Callback=()=>1;let n=3;[_,value,n]=native.Pair(a,b);return value()+n;}
export function Local(a:boolean,b:boolean,observe:(x:int,y:int,n:int)=>void):void{
 let __assignmentValue0:Callback=()=>1;let __assignmentValue1:Callback=()=>2;let __assignmentValue2=3;
 const report=():void=>observe(__assignmentValue0(),__assignmentValue1(),__assignmentValue2);defer report();
 [__assignmentValue0,__assignmentValue1,__assignmentValue2]=native.Pair(a,b);
}
export function Flag(p:boolean):(Callback,boolean){return native.Flag(p);}
export function Optional(p:boolean):(Callback|null,int){return native.Plain(p);}
export function Raw(a:boolean,b:boolean):(native.Raw,native.Raw,int){return native.PublicPair(a,b);}
export function TypedNil():(View,int){return native.Pointer();}
export function Interface(v:error):(View,int){return native.Interface(v);}
export function Qualified(p:boolean):(Timed,int){const unused=clock.ToUpper(b"x");return native.Timed(p);}
export let Timer:Timed=(value)=>int(value);
export let Reader:View=initialView();
function initialView():View{const [value,n]=native.Pointer();return value;}
export function AssignTimed(p:boolean):void{[Timer,Number]=native.Timed(p);}
export function AssignView(v:error):void{[Reader,Number]=native.Interface(v);}
export function AssignGeneric<__assignmentValue0 extends Functions>(p:boolean,fallback:__assignmentValue0):__assignmentValue0{let value:__assignmentValue0=fallback;let n=0;[value,n]=native.Plain(p);return value;}

`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "multiplestorage")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "multiplestorage")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) || bytes.Contains(generated, []byte("native.privateTimed")) {
		t.Fatalf("private Go result type leaked:\n%s", generated)
	}
	reference := `package reference
import "time"
var calls,hits int
var First=func()int{return 1}
var Second=func()int{return 2}
var Number=3
func Reset(){calls=0;hits=0;First=func()int{return 1};Second=func()int{return 2};Number=3}
func Counts()[]int{return []int{calls,hits}}
func Pair(a,b bool)(func()int,func()int,int){calls++;var x,y func()int;if a{x=func()int{return 7}};if b{y=func()int{return 8}};if x==nil{panic("nil first")};if y==nil{panic("nil second")};return x,y,9}
func Finally(a,b bool)(func()int,func()int,int){defer func(){hits++}();return Pair(a,b)}
func Plain(p bool)(func()int,int){calls++;var f func()int;if p{f=func()int{return 7}};if f==nil{panic("nil result")};return f,9}
func Assign(a,b bool){First,Second,Number=Pair(a,b)}
func Duplicate(a,b bool){First,First,Number=Pair(a,b)}
func Post(a,b bool){for i:=0;i<1;First,Second,Number=Pair(a,b){i++}}
func IgnoreFirst(a,b bool)int{_,value,n:=Raw(a,b);if value==nil{panic("nil storage")};return value()+n}
func Local(a,b bool,observe func(int,int,int)){x:=func()int{return 1};y:=func()int{return 2};n:=3;defer func(){observe(x(),y(),n)}();x,y,n=Pair(a,b)}
func Optional(p bool)(func()int,int){calls++;var f func()int;if p{f=func()int{return 7}};return f,9}
func Raw(a,b bool)(func()int,func()int,int){calls++;var x,y func()int;if a{x=func()int{return 7}};if b{y=func()int{return 8}};return x,y,9}
type Failure struct{}
func(*Failure)Error()string{return "failure"}
func TypedNil()(interface{Error()string},int){calls++;var p *Failure;return p,9}
func Interface(v error)(interface{Error()string},int){calls++;if v==nil{panic("nil interface")};return v,9}
func Timed(p bool)(func(time.Duration)int,int){calls++;var f func(time.Duration)int;if p{f=func(v time.Duration)int{return int(v)}};if f==nil{panic("nil timed")};return f,9}
func Flag(p bool)(func()int,bool){f,_:=Plain(p);return f,false}
var Timer=func(value time.Duration)int{return int(value)}
var Reader interface{Error()string}=(*Failure)(nil)
func AssignTimed(p bool){Timer,Number=Timed(p)}
func AssignView(v error){Reader,Number=Interface(v)}
func AssignGeneric(p bool,fallback func()int)func()int{value:=fallback;value,_=Plain(p);return value}
`
	comparison := `package multiplestorage_test
import("errors";"fmt";"reflect";"strings";"testing";"time";g "multiple-storage.test";r "multiple-storage.test/reference")
func capture(f func())(failure any){defer func(){failure=recover()}();f();return nil}
func TestForward(t *testing.T){host:=g.NewHost();for _,a:=range []bool{false,true}{for _,b:=range []bool{false,true}{for name,pair:=range map[string][2]func(bool,bool)(func()int,func()int,int){"function":{g.Forward,r.Pair},"arrow":{g.Arrow,r.Pair},"block arrow":{g.BlockArrow,r.Pair},"method":{host.Pair,r.Pair},"finally":{g.Finally,r.Finally}}{t.Run(fmt.Sprint(name,a,b),func(t *testing.T){g.Reset();r.Reset();var gx,gy,rx,ry func()int;var gn,rn int;gp:=capture(func(){gx,gy,gn=pair[0](a,b)});rp:=capture(func(){rx,ry,rn=pair[1](a,b)});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.Counts(),r.Counts())};if gp==nil{if gx()!=rx()||gy()!=ry()||gn!=rn{t.Fatal("values changed")}}else{slot:=1;if a{slot=2};if !strings.Contains(fmt.Sprint(gp),fmt.Sprintf("non-null result %d failed at entry.km:",slot)){t.Fatal(gp)}}})}}}}
func TestAssignments(t *testing.T){for _,a:=range []bool{false,true}{for _,b:=range []bool{false,true}{for name,pair:=range map[string][2]func(bool,bool){"global":{g.Assign,r.Assign},"duplicate":{g.Duplicate,r.Duplicate},"for post":{g.Post,r.Post}}{t.Run(fmt.Sprint(name,a,b),func(t *testing.T){g.Reset();r.Reset();gp:=capture(func(){pair[0](a,b)});rp:=capture(func(){pair[1](a,b)});if(gp==nil)!=(rp==nil)||g.First()!=r.First()||g.Second()!=r.Second()||g.Number!=r.Number||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.First(),r.First(),g.Second(),r.Second(),g.Number,r.Number,g.Counts(),r.Counts())}})};g.Reset();r.Reset();var got,want []int;gp:=capture(func(){g.Local(a,b,func(x,y,n int){got=[]int{x,y,n}})});rp:=capture(func(){r.Local(a,b,func(x,y,n int){want=[]int{x,y,n}})});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(got,want)||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,got,want,g.Counts(),r.Counts())}}}}
func TestOtherSlots(t *testing.T){for _,p:=range []bool{false,true}{g.Reset();r.Reset();var gf,rf func()int;var gn,rn int;gp:=capture(func(){gf,gn=g.Generic[func()int](p)});rp:=capture(func(){rf,rn=r.Plain(p)});if(gp==nil)!=(rp==nil)||gn!=rn||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,gn,rn)};if gp==nil&&gf()!=rf(){t.Fatal("generic result")};var gt,rt func(time.Duration)int;g.Reset();r.Reset();gp=capture(func(){gt,gn=g.Qualified(p)});rp=capture(func(){rt,rn=r.Timed(p)});if(gp==nil)!=(rp==nil)||gn!=rn||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,gn,rn)};if gp==nil&&gt(4)!=rt(4){t.Fatal("qualified result")};gp=capture(func(){g.Flag(p)});rp=capture(func(){r.Flag(p)});if(gp==nil)!=(rp==nil){t.Fatal("ordinary false flag does not permit nil",gp,rp)}}}
func TestQualifiedAndGenericAssignments(t *testing.T){for _,p:=range []bool{false,true}{g.Reset();r.Reset();gp:=capture(func(){g.AssignTimed(p)});rp:=capture(func(){r.AssignTimed(p)});if(gp==nil)!=(rp==nil)||g.Timer(4)!=r.Timer(4)||g.Number!=r.Number||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.Counts(),r.Counts())};g.Reset();r.Reset();gp=capture(func(){g.AssignGeneric[func()int](p,func()int{return 2})});rp=capture(func(){r.AssignGeneric(p,func()int{return 2})});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.Counts(),r.Counts())}};for _,v:=range []error{nil,errors.New("failure")}{g.Reset();r.Reset();gp:=capture(func(){g.AssignView(v)});rp:=capture(func(){r.AssignView(v)});if(gp==nil)!=(rp==nil)||g.Reader.Error()!=r.Reader.Error()||g.Number!=r.Number||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.Number,r.Number,g.Counts(),r.Counts())}}}
func TestDiscardedSlot(t *testing.T){for _,a:=range []bool{false,true}{for _,b:=range []bool{false,true}{g.Reset();r.Reset();var got,want int;gp:=capture(func(){got=g.IgnoreFirst(a,b)});rp:=capture(func(){want=r.IgnoreFirst(a,b)});if(gp==nil)!=(rp==nil)||got!=want||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,got,want,g.Counts(),r.Counts())}}}}
func TestNullableAndRaw(t *testing.T){for _,p:=range []bool{false,true}{gf,gn:=g.Optional(p);rf,rn:=r.Optional(p);if(gf==nil)!=(rf==nil)||gn!=rn{t.Fatal("nullable result")};gx,gy,gn:=g.Raw(p,p);rx,ry,rn:=r.Raw(p,p);if(gx==nil)!=(rx==nil)||(gy==nil)!=(ry==nil)||gn!=rn{t.Fatal("raw Go result")}}}
func TestInterfaces(t *testing.T){gv,gn:=g.TypedNil();rv,rn:=r.TypedNil();if gv==nil||rv==nil||gv.Error()!=rv.Error()||gn!=rn{t.Fatal("typed nil boxed incorrectly")};for _,v:=range []error{nil,errors.New("failure")}{g.Reset();r.Reset();var gx,rx interface{Error()string};gp:=capture(func(){gx,gn=g.Interface(v)});rp:=capture(func(){rx,rn=r.Interface(v)});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp)};if gp==nil&&gx.Error()!=rx.Error(){t.Fatal(gx,rx)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "multiple-storage.test", generated, reference, comparison, nil, nil)
}
