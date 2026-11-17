package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestChannelAbsenceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `export interface Reader{function read():int;}
export class Item implements Reader{constructor(public n:int){}public function read():int{return this.n;}}
export class Child extends Item{constructor(n:int){super(n);}}
export struct Row{public item:Item;}
`,
		"entry.km": `import {Reader,Item,Child,Row} from "./models";
function input(p:boolean):GoChannel<Item>{const ch=goChannel<Item>(1);if(p){ch<-new Item(7);}closeGoChannel(ch);return ch;}
export function Receive(p:boolean):int{const v=<-input(p);if(v===null){return 0;}return v.n;}
export function Fallback(p:boolean):int{return ((<-input(p))??new Item(9)).n;}
export function Assert(p:boolean):int{return (<-input(p))!.n;}
export function Checked(p:boolean):int{select{case const [v,open]=<-input(p){if(!open){return 0;}return v.n;}}}
export function Assigned(p:boolean):int{let v:Item|null=null;let open=false;select{case [v,open]=<-input(p){if(!open){return 0;}return v.n;}}}
export function Interface(p:boolean):int{let value:Reader|null=null;select{case value=<-input(p){}}if(value===null){return 0;}return value.read();}
class Holder{private stored:Reader|null=null;public set value(v:Reader|null){this.stored=v;}public get value():Reader|null{return this.stored;}}
export function Property(p:boolean):int{const holder=new Holder();select{case holder.value=<-input(p){}}const value=holder.value;if(value===null){return 0;}return value.read();}
export function Parent(p:boolean):int{const ch=goChannel<Child>(1);if(p){ch<-new Child(11);}closeGoChannel(ch);let value:Item|null=null;select{case value=<-ch{}}if(value===null){return 0;}return value.n;}
export function Generic<T>(ch:GoChannel<T>,fallback:T):T{const [v,open]=<-ch;if(!open){return fallback;}return v;}
export function GenericSelect<T>(ch:GoChannel<T>,fallback:T):T{select{case const [v,open]=<-ch{if(!open){return fallback;}return v;}}}
export function Discard():int{const ch=goChannel<Row>();closeGoChannel(ch);<-ch;const _=<-ch;_=<-ch;select{case <-ch{}}select{case const _=<-ch{}}select{case _=<-ch{}}return 6;}
export function NilDefault():int{let ch:GoChannel<Item>=nil;select{case const v=<-ch{if(v===null){return 0;}return v.n;}default{return 9;}}}
function digit(value:boolean):int{if(value){return 1;}return 0;}
export function Targets(ready:boolean):int[]{
 let i=0;let calls=0;const values=[0,0];const flags=[false,false];const ch=goChannel<int>(1);if(ready){ch<-17;}
 const index=():int=>{calls++;i=1;return 0;};
 select{case [values[i],flags[index()]]=<-ch{}default{}}
 return [values[0],values[1],digit(flags[0]),i,calls];
}
class Setter{public stored:int=0;constructor(private change:()=>void){}public set value(v:int){this.change();this.stored=v;}}
export function SetterTargets(ready:boolean):int[]{
 let i=0;let receivers=0;const flags=[false,false];const ch=goChannel<int>(1);if(ready){ch<-13;}
 const setter=new Setter(()=>{i=1;});const target=():Setter=>{receivers++;return setter;};
 select{case [target().value,flags[i]]=<-ch{}default{}}
 return [setter.stored,digit(flags[0]),digit(flags[1]),i,receivers];
}
export function SetterBounds():int{let calls=0;const setter=new Setter(()=>{calls++;});const flags:boolean[]=[];const ch=goChannel<int>(1);ch<-3;try{select{case [setter.value,flags[0]]=<-ch{}}}finally{return calls;}}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "channelabsence")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "channelabsence")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
type Reader interface{Read()int}
type Item struct{N int}
func(v *Item)Read()int{return v.N}
type Child struct{Item}
func input(p bool)chan *Item{ch:=make(chan *Item,1);if p{ch<-&Item{7}};close(ch);return ch}
func Receive(p bool)int{v:=<-input(p);if v==nil{return 0};return v.N}
func Fallback(p bool)int{v:=<-input(p);if v==nil{v=&Item{9}};return v.N}
func Assert(p bool)int{v:=<-input(p);if v==nil{panic("absent")};return v.N}
func Checked(p bool)int{select{case v,open:=<-input(p):if !open{return 0};return v.N}}
func Assigned(p bool)int{var v *Item;var open bool;select{case v,open=<-input(p):if !open{return 0};return v.N}}
func Interface(p bool)int{var value Reader;select{case v:=<-input(p):if v!=nil{value=v}};if value==nil{return 0};return value.Read()}
func Property(p bool)int{var stored Reader;set:=func(v Reader){stored=v};select{case v:=<-input(p):var converted Reader;if v!=nil{converted=v};set(converted)};if stored==nil{return 0};return stored.Read()}
func Parent(p bool)int{ch:=make(chan *Child,1);if p{ch<-&Child{Item{11}}};close(ch);var value *Item;select{case v:=<-ch:if v!=nil{value=&v.Item}};if value==nil{return 0};return value.N}
func Generic[T any](ch chan T,fallback T)T{v,open:=<-ch;if !open{return fallback};return v}
func GenericSelect[T any](ch chan T,fallback T)T{select{case v,open:=<-ch:if !open{return fallback};return v}}
func Discard()int{ch:=make(chan struct{item *Item});close(ch);<-ch;_=<-ch;_=<-ch;select{case <-ch:};select{case <-ch:};select{case <-ch:};return 6}
func NilDefault()int{var ch chan *Item;select{case v:=<-ch:if v==nil{return 0};return v.N;default:return 9}}
func digit(v bool)int{if v{return 1};return 0}
func Targets(ready bool)[]int{i,calls:=0,0;values:=[]int{0,0};flags:=[]bool{false,false};ch:=make(chan int,1);if ready{ch<-17};index:=func()int{calls++;i=1;return 0};select{case v,open:=<-ch:first:=&values[i];second:=&flags[index()];*first=v;*second=open;default:};return []int{values[0],values[1],digit(flags[0]),i,calls}}
func SetterTargets(ready bool)[]int{i,receivers,stored:=0,0,0;flags:=[]bool{false,false};ch:=make(chan int,1);if ready{ch<-13};target:=func()func(int){receivers++;return func(v int){i=1;stored=v}};select{case v,open:=<-ch:set:=target();flag:=&flags[i];set(v);*flag=open;default:};return []int{stored,digit(flags[0]),digit(flags[1]),i,receivers}}
func SetterBounds()(calls int){defer func(){recover()}();flags:=[]bool{};ch:=make(chan int,1);ch<-3;select{case _,open:=<-ch:flag:=&flags[0];calls++;*flag=open};return calls}
`
	comparison := `package channelabsence_test
import("fmt";"reflect";"strings";"testing";g "channel-absence.test";r "channel-absence.test/reference")
func capture(f func())(failure any){defer func(){failure=recover()}();f();return nil}
func TestReceive(t *testing.T){for _,p:=range []bool{false,true}{
 for _,pair:=range [][2]func(bool)int{{g.Receive,r.Receive},{g.Fallback,r.Fallback},{g.Checked,r.Checked},{g.Assigned,r.Assigned},{g.Interface,r.Interface},{g.Property,r.Property},{g.Parent,r.Parent}}{if got,want:=pair[0](p),pair[1](p);got!=want{t.Fatal(p,got,want)}}
 gp,rp:=capture(func(){g.Assert(p)}),capture(func(){r.Assert(p)});if(gp==nil)!=(rp==nil){t.Fatal(p,gp,rp)};if gp!=nil&&!strings.Contains(fmt.Sprint(gp),"entry.km:"){t.Fatal(gp)}
 for _,pair:=range [][2]func(bool)[]int{{g.Targets,r.Targets},{g.SetterTargets,r.SetterTargets}}{if got,want:=pair[0](p),pair[1](p);!reflect.DeepEqual(got,want){t.Fatal(p,got,want)}}
 for _,pair:=range [][2]func(chan int,int)int{{g.Generic[int],r.Generic[int]},{g.GenericSelect[int],r.GenericSelect[int]}}{gc,rc:=make(chan int,1),make(chan int,1);if p{gc<-7;rc<-7};close(gc);close(rc);if got,want:=pair[0](gc,9),pair[1](rc,9);got!=want{t.Fatal(p,got,want)}}
 }
 if got,want:=g.Discard(),r.Discard();got!=want{t.Fatal(got,want)};if got,want:=g.NilDefault(),r.NilDefault();got!=want{t.Fatal(got,want)};if got,want:=g.SetterBounds(),r.SetterBounds();got!=want{t.Fatal(got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "channel-absence.test", generated, reference, comparison)
}
