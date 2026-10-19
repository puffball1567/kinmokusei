package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNullableClassInterfaceUpcastsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, source := range map[string]string{
		"models.km": `import go fmt from "fmt";
export interface Speaker{function speak():string;}
export class User implements Speaker{constructor(public name:string){}public function speak():string{return this.name;}}
export interface Reader<T>{function read():T;}
export class Box<T> implements Reader<T>{constructor(public value:T){}public function read():T{return this.value;}}
export class Raw implements fmt.Stringer{public function string():bstring{return b"raw";}}
export class Failure extends Exception{constructor(){super("failure");}}
export alias MaybeSpeaker=Speaker|null;
`,
		"entry.km": `import {Speaker,User,Reader,Box,Raw,Failure,MaybeSpeaker} from "./models";
import go fmt from "fmt";
let calls:int=0;
function find(p:boolean):User|null{calls++;if(p){return new User("金木犀");}return null;}
function pass(v:Speaker|null):boolean{return v===null;}
function returned(p:boolean):Speaker|null{return find(p);}
function pair(p:boolean):(User|null,int){return find(p),7;}
function checked(p:boolean):Result<Speaker|null>{return ok(find(p));}
struct Holder{public value:Speaker|null;}
class Owner{constructor(public value:Speaker|null){}}
class Property{
 private stored:Speaker|null=null;
 public set value(v:Speaker|null){this.stored=v;}
 public get value():Speaker|null{return this.stored;}
}
function errorValue(p:boolean):Failure|null{if(p){return new Failure();}return null;}
function goValue(p:boolean):Raw|null{if(p){return new Raw();}return null;}
function box<T>(v:Box<T>|null):Reader<T>|null{return v;}
export function Run(p:boolean):(boolean[],int){
 calls=0;
 const direct:Speaker|null=find(p);
 const alias:MaybeSpeaker=find(p);
 let assigned:Speaker|null=new User("old");assigned=find(p);
 const field=Holder{value:find(p)};
 const owner=new Owner(find(p));
 const object:{value:Speaker|null}={value:find(p)};
 const array:MaybeSpeaker[]=[find(p)];
 const fixed:[1]MaybeSpeaker=[find(p)];
 const map=makeMap<int,Speaker|null>();map[0]=find(p);
 const channel=goChannel<Speaker|null>(1);channel<-find(p);const received=<-channel;
 const arrow:()=>Speaker|null=()=>find(p);
 const [result,_]=checked(p);
 let multiple:Speaker|null=null;let tag:int=0;[multiple,tag]=pair(p);
 const property=new Property();property.value=find(p);
 const anonymous:interface{speak():string;}|null=find(p);
 const raw:fmt.Stringer|null=goValue(p);
 const err:error|null=errorValue(p);
 let generic:Box<int>|null=null;if(p){generic=new Box<int>(42);}
 const reader=box<int>(generic);
 const value=():User|null=>find(p);const shadow:Speaker|null=value();
 const answer=[direct===null,alias===null,assigned===null,field.value===null,
 owner.value===null,object.value===null,array[0]===null,fixed[0]===null,
 map[0]===null,received===null,arrow()===null,result===null,multiple===null,
 property.value===null,anonymous===null,raw===null,err===null,reader===null,
 pass(find(p)),returned(p)===null,tag===7,shadow===null];
 if(direct!==null){if(direct.speak()!=="金木犀"){throw new Exception("bad dispatch");}}
 if(reader!==null){if(reader.read()!==42){throw new Exception("bad generic dispatch");}}
 return answer,calls;
}
export function RawErrorAbsent(v:error):boolean{const snapshot:error|null=v;return snapshot===null;}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "nullableinterfaces")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{filepath.Join(root, "entry.km")}, "nullableinterfaces")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import "fmt"
type speaker interface{Speak()string}
type user struct{name string}
func(u *user)Speak()string{return u.name}
type reader[T any]interface{Read()T}
type box[T any]struct{value T}
func(v *box[T])Read()T{return v.value}
type raw struct{}
func(*raw)String()string{return "raw"}
type failure struct{}
func(*failure)Error()string{return "failure"}
var calls int
func find(p bool)*user{calls++;if p{return &user{"金木犀"}};return nil}
func normalized(v *user)speaker{if v==nil{return nil};return v}
func pass(v speaker)bool{return v==nil}
func returned(p bool)speaker{return normalized(find(p))}
func pair(p bool)(*user,int){return find(p),7}
func checked(p bool)(speaker,error){return normalized(find(p)),nil}
type holder struct{value speaker}
type owner struct{value speaker}
type property struct{value speaker}
func errorValue(p bool)error{var v *failure;if p{v=&failure{}};if v==nil{return nil};return v}
func goValue(p bool)fmt.Stringer{var v *raw;if p{v=&raw{}};if v==nil{return nil};return v}
func genericBox[T any](v *box[T])reader[T]{if v==nil{return nil};return v}
func Run(p bool)([]bool,int){
calls=0
direct:=normalized(find(p));alias:=normalized(find(p))
var assigned speaker=&user{"old"};assigned=normalized(find(p))
field:=holder{normalized(find(p))};owner:=owner{normalized(find(p))}
object:=struct{value speaker}{normalized(find(p))}
array:=[]speaker{normalized(find(p))};fixed:=[1]speaker{normalized(find(p))}
table:=make(map[int]speaker);table[0]=normalized(find(p))
channel:=make(chan speaker,1);channel<-normalized(find(p));received:=<-channel
arrow:=func()speaker{return normalized(find(p))};result,_:=checked(p)
var multiple speaker;var tag int;value,n:=pair(p);multiple,tag=normalized(value),n
property:=property{};property.value=normalized(find(p))
var anonymous interface{Speak()string}=normalized(find(p))
raw:=goValue(p);err:=errorValue(p)
var generic *box[int];if p{generic=&box[int]{42}};reader:=genericBox(generic)
valueFn:=func()*user{return find(p)};shadow:=normalized(valueFn())
answer:=[]bool{direct==nil,alias==nil,assigned==nil,field.value==nil,
owner.value==nil,object.value==nil,array[0]==nil,fixed[0]==nil,
table[0]==nil,received==nil,arrow()==nil,result==nil,multiple==nil,
property.value==nil,anonymous==nil,raw==nil,err==nil,reader==nil,
pass(normalized(find(p))),returned(p)==nil,tag==7,shadow==nil}
if direct!=nil&&direct.Speak()!="金木犀"{panic("bad dispatch")}
if reader!=nil&&reader.Read()!=42{panic("bad generic dispatch")}
return answer,calls
}
func RawErrorAbsent(v error)bool{return v==nil}
`
	tests := `package nullableinterfaces_test
import("errors";"reflect";"testing";g "nullable-interfaces.test";r "nullable-interfaces.test/reference")
type typedError struct{}
func(*typedError)Error()string{return "typed nil"}
func TestNullableInterfaces(t *testing.T){
for _,present:=range []bool{false,true}{a,n:=g.Run(present);b,m:=r.Run(present);if !reflect.DeepEqual(a,b)||n!=m{t.Fatalf("present=%v got %v/%d want %v/%d",present,a,n,b,m)}}
var typedNil *typedError
for _,v:=range []error{nil,errors.New("failure"),typedNil}{if a,b:=g.RawErrorAbsent(v),r.RawErrorAbsent(v);a!=b{t.Fatalf("raw Go interface %T: got %v want %v",v,a,b)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "nullable-interfaces.test", generated, reference, tests)
}
