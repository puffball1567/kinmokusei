package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageBoundariesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module storage-boundaries.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
import "os"
var loads,hits int
var present bool
type Raw func()int
type hidden func()int
func Reset(){loads=0;hits=0}
func Counts()(int,int){return loads,hits}
func Touch()int{hits++;return hits}
func Set(p bool){present=p}
func Load(p bool)Raw{loads++;if p{return func()int{return 7}};return nil}
func Hidden(p bool)hidden{loads++;if p{return func()int{return 8}};return nil}
func Current()Raw{return Load(present)}
func Default()Raw{return Load(os.Getenv("KINMOKUSEI_STORAGE_NULL")!="1")}
func Plain(p bool)func()int{return Load(p)}
`,
		"models.km": `export alias Callback=()=>int;
export type Named=distinct ()=>int;
export alias View=interface{Error():bstring;};
export class Holder{constructor(public value:Callback){}}
export struct Record{public value:Callback;}
`,
		"entry.km": `import go native from "example.com/native";
import {Callback,Named,View,Holder,Record} from "./models";
export function Reset():void{native.Reset();}
export function Counts():int[]{const [loads,hits]=native.Counts();return [loads,hits];}
export function Bind(p:boolean):int{const value:Callback=native.Load(p);native.Touch();return value();}
export function Return(p:boolean):Callback{return native.Load(p);}
export function Hidden(p:boolean):Callback{return native.Hidden(p);}
export function Arrow(p:boolean):()=>Callback{return ():Callback=>native.Load(p);}
export function Defined(p:boolean):Named{const value:Named=native.Plain(p);native.Touch();return value;}
export function Assign(holder:Holder,p:boolean):void{holder.value=native.Load(p);native.Touch();}
export function MapAssign(values:Map<int,Callback>,p:boolean):void{values[0]=native.Load(p);native.Touch();}
export function Success(p:boolean):Result<Callback>{return ok(native.Load(p));}
export function Pair(p:boolean):(Callback,int){return native.Load(p),native.Touch();}
export function Literal(p:boolean):Callback[]{const values:Callback[]=[native.Load(p)];native.Touch();return values;}
export function FixedLen(p:boolean):int{const raw=native.Load(p);const values:[1]Callback=[raw];return len(values);}
export function RecordLiteral(p:boolean):Record{const record=Record{value:native.Load(p)};native.Touch();return record;}
export function ObjectLiteral(p:boolean):{value:Callback}{const record:{value:Callback}={value:native.Load(p)};native.Touch();return record;}
export class Initialized{public value:Callback=native.Current();constructor(){native.Touch();}}
export function Field(p:boolean):Initialized{native.Set(p);return new Initialized();}
export function Append(p:boolean):Callback[]{const values:Callback[]=[];const result=append(values,native.Load(p));native.Touch();return result;}
export function Send(ch:GoChannel<Callback>,p:boolean):void{ch<-native.Load(p);native.Touch();}
export function Raw(p:boolean):boolean{const value=native.Load(p);native.Touch();return value===nil;}
export function Nullable(p:boolean):boolean{const value:Callback|null=native.Load(p);native.Touch();return value===null;}
export function Interface(raw:error):View{return raw;}
export function Finally(p:boolean):Callback{try{return native.Load(p);}finally{native.Touch();}}
export function Loop(p:boolean):int{let value:Callback=()=>9;let count=0;while(count<2){value=native.Load(p);count++;}native.Touch();return value();}
export function Global():int{return global();}
const global:Callback=native.Default();
export struct ArrayRecord{public values:[1]Callback;}
export function DirectLen(p:boolean):int{const raw=native.Load(p);const n=len(ArrayRecord{values:[raw]}.values);const pointer=&n;return *pointer;}
export function DirectCap(p:boolean):int{const raw=native.Load(p);const n=cap(ArrayRecord{values:[raw]}.values);const pointer=&n;return *pointer;}
export function Unused(p:boolean):void{const raw=native.Load(p);const unused:Callback=raw;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "storageboundaries")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "storageboundaries")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hidden")) {
		t.Fatalf("private Go function type leaked:\n%s", generated)
	}
	reference := `package reference
var loads,hits int
func Reset(){loads=0;hits=0}
func Counts()[]int{return []int{loads,hits}}
func touch()int{hits++;return hits}
func load(p bool)func()int{loads++;if p{return func()int{return 7}};return nil}
func check(v func()int)func()int{if v==nil{panic("nil storage")};return v}
func Bind(p bool)int{v:=check(load(p));touch();return v()}
func Return(p bool)func()int{return check(load(p))}
func Hidden(p bool)func()int{loads++;var v func()int;if p{v=func()int{return 8}};return check(v)}
func Defined(p bool)func()int{v:=check(load(p));touch();return v}
func Arrow(p bool)func()func()int{return func()func()int{return check(load(p))}}
type Holder struct{Value func()int}
func Assign(h *Holder,p bool){h.Value=check(load(p));touch()}
func MapAssign(m map[int]func()int,p bool){m[0]=check(load(p));touch()}
func Success(p bool)(func()int,error){return check(load(p)),nil}
func Pair(p bool)(func()int,int){v:=check(load(p));return v,touch()}
func Literal(p bool)[]func()int{v:=[]func()int{check(load(p))};touch();return v}
func FixedLen(p bool)int{raw:=load(p);v:=[1]func()int{check(raw)};n:=len(v);pointer:=&n;return *pointer}
type Record struct{Value func()int}
func RecordLiteral(p bool)Record{v:=Record{check(load(p))};touch();return v}
func ObjectLiteral(p bool)struct{Value func()int}{v:=struct{Value func()int}{check(load(p))};touch();return v}
func Field(p bool)*Holder{v:=check(load(p));touch();return &Holder{v}}
func Append(p bool)[]func()int{v:=append([]func()int{},check(load(p)));touch();return v}
func Send(ch chan func()int,p bool){ch<-check(load(p));touch()}
func Raw(p bool)bool{v:=load(p);touch();return v==nil}
func Nullable(p bool)bool{return Raw(p)}
func Interface(v error)interface{Error()string}{if v==nil{panic("nil storage")};return v}
func Finally(p bool)func()int{defer touch();return check(load(p))}
var global=check(load(true))
func Global()int{return global()}
func DirectLen(p bool)int{raw:=load(p);n:=len(struct{Values [1]func()int}{[1]func()int{check(raw)}}.Values);pointer:=&n;return *pointer}
func DirectCap(p bool)int{raw:=load(p);n:=cap(struct{Values [1]func()int}{[1]func()int{check(raw)}}.Values);pointer:=&n;return *pointer}
func Unused(p bool){raw:=load(p);_ =check(raw)}
func Loop(p bool)int{v:=func()int{return 9};for n:=0;n<2;n++{v=check(load(p))};touch();return v()}
`
	comparison := `package storageboundaries_test
import("errors";"fmt";"os";"os/exec";"reflect";"strings";"testing";g "storage-boundaries.test";r "storage-boundaries.test/reference")
func capture(f func())(failure any){defer func(){failure=recover()}();f();return nil}
type failure struct{}
func(*failure)Error()string{return "failure"}
func TestStorage(t *testing.T){for _,p:=range []bool{false,true}{for name,pair:=range map[string][2]func(){
 "binding":{func(){g.Bind(p)},func(){r.Bind(p)}},"unused":{func(){g.Unused(p)},func(){r.Unused(p)}},"defined":{func(){g.Defined(p)},func(){r.Defined(p)}},"direct len":{func(){g.DirectLen(p)},func(){r.DirectLen(p)}},"direct cap":{func(){g.DirectCap(p)},func(){r.DirectCap(p)}},"return":{func(){g.Return(p)},func(){r.Return(p)}},"private":{func(){g.Hidden(p)},func(){r.Hidden(p)}},"arrow":{func(){g.Arrow(p)()},func(){r.Arrow(p)()}},
 "result":{func(){g.Success(p)},func(){r.Success(p)}},"pair":{func(){g.Pair(p)},func(){r.Pair(p)}},"literal":{func(){g.Literal(p)},func(){r.Literal(p)}},"array len":{func(){g.FixedLen(p)},func(){r.FixedLen(p)}},"struct":{func(){g.RecordLiteral(p)},func(){r.RecordLiteral(p)}},"object":{func(){g.ObjectLiteral(p)},func(){r.ObjectLiteral(p)}},"field":{func(){g.Field(p)},func(){r.Field(p)}},"append":{func(){g.Append(p)},func(){r.Append(p)}},"finally":{func(){g.Finally(p)},func(){r.Finally(p)}},"loop":{func(){g.Loop(p)},func(){r.Loop(p)}},
 }{t.Run(fmt.Sprint(name,p),func(t *testing.T){g.Reset();r.Reset();gp,rp:=capture(pair[0]),capture(pair[1]);if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.Counts(),r.Counts())};if !p&&!strings.Contains(fmt.Sprint(gp),"non-null storage boundary failed at entry.km:"){t.Fatal(gp)}})}}
 if got:=fmt.Sprint(capture(func(){g.Bind(false)}));got!="non-null storage boundary failed at entry.km:5:58"{t.Fatal(got)}
}
func TestGlobalInitialization(t *testing.T){command:=exec.Command(os.Args[0],"-test.run=^$");command.Env=append(os.Environ(),"KINMOKUSEI_STORAGE_NULL=1");output,err:=command.CombinedOutput();if err==nil||!strings.Contains(string(output),"non-null storage boundary failed at entry.km:"){t.Fatalf("global nil publication: %v\n%s",err,output)}}
func TestValues(t *testing.T){if got,want:=g.Bind(true),r.Bind(true);got!=want{t.Fatal(got,want)};if got,want:=g.Hidden(true)(),r.Hidden(true)();got!=want{t.Fatal(got,want)};if g.Return(true)()!=r.Return(true)()||g.Global()!=r.Global()||g.Loop(true)!=r.Loop(true){t.Fatal("value changed")}}
func TestPublication(t *testing.T){for _,p:=range []bool{false,true}{gh:=g.NewHolder(func()int{return 9});rh:=&r.Holder{Value:func()int{return 9}};g.Reset();r.Reset();gp:=capture(func(){g.Assign(gh,p)});rp:=capture(func(){r.Assign(rh,p)});if(gp==nil)!=(rp==nil)||gh.Value()!=rh.Value()||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,gh.Value(),rh.Value(),g.Counts(),r.Counts())}
 gm,rm:=map[int]func()int{0:func()int{return 9}},map[int]func()int{0:func()int{return 9}};g.Reset();r.Reset();gp=capture(func(){g.MapAssign(gm,p)});rp=capture(func(){r.MapAssign(rm,p)});if(gp==nil)!=(rp==nil)||gm[0]()!=rm[0]()||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.Counts(),r.Counts())}
 gc,rc:=make(chan func()int,1),make(chan func()int,1);g.Reset();r.Reset();gp=capture(func(){g.Send(gc,p)});rp=capture(func(){r.Send(rc,p)});if(gp==nil)!=(rp==nil)||len(gc)!=len(rc)||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(gp,rp,g.Counts(),r.Counts())};if len(gc)!=0{if got,want:=(<-gc)(),(<-rc)();got!=want{t.Fatal(got,want)}}
}}
func TestRawAndNullable(t *testing.T){for _,p:=range []bool{false,true}{g.Reset();r.Reset();if got,want:=g.Raw(p),r.Raw(p);got!=want||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(got,want,g.Counts(),r.Counts())};g.Reset();r.Reset();if got,want:=g.Nullable(p),r.Nullable(p);got!=want||!reflect.DeepEqual(g.Counts(),r.Counts()){t.Fatal(got,want,g.Counts(),r.Counts())}}}
func TestInterfaces(t *testing.T){for _,v:=range []error{nil,(*failure)(nil),errors.New("failure")}{var got,want any;gp:=capture(func(){got=g.Interface(v)});rp:=capture(func(){want=r.Interface(v)});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(got,want){t.Fatal(got,want,gp,rp)}}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "storage-boundaries.test", generated, reference, comparison, nil, nil)
}
