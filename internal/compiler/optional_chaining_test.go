package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOptionalChainingMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"user.km": `export class User {
 constructor(public name:string,public n:int){}
 public next:User|null=null;
 public function read(n:int):int{return this.n+n;}
 public function self():User{return this;}
 public function touch(visit:(n:int)=>void):void{visit(this.n);}
}
`,
		"entry.km": `import {User} from "./user";
import go flag from "flag";
import go __kinmokuseiOptionalReceiver from "strconv";
export alias Callback=(n:int)=>int;
export function Name(u:User|null):string{return u?.name??"anonymous";}
export function Chain(u:User|null):int{return u?.next?.n??-1;}
export function Tail(u:User|null):int{return u?.self().n??-1;}
export function Reference(u:User|null):User|null{return u?.self();}
export function Read(u:User|null,visit:(n:int)=>void):int{
 const root=():User|null=>{visit(1);return u;};
 const argument=():int=>{visit(2);return 3;};
 const fallback=():int=>{visit(4);return -1;};
 return root()?.read(argument())??fallback();
}
export function Index(xs:int[]|null,visit:(n:int)=>void):int{
 const index=():int=>{visit(1);return 0;};return xs?.[index()]??9;
}
export function Slice(xs:int[]|null):int[]|null{return xs?.[1:2];}
export function Call(fn:Callback|null,visit:(n:int)=>void):int{
 const argument=():int=>{visit(1);return 3;};return fn?.(argument())??9;
}
export function Touch(u:User|null,visit:(n:int)=>void):void{u?.touch(visit);}
export function Split(u:User|null):int{const[n,present]=u?.n;if(!present){return -1;}return n;}
export function Force(u:User|null):string{return u?.name!;}
export function AssertGroup(u:User|null):int{return (u?.self())!.n;}
alias Producer<T>=()=>T;export function Generic<T>(fn:Producer<T>|null,fallback:T):T{return fn?.()??fallback;}
export function Raw():bstring{return flag.Lookup("kinmokusei-absent-optional")?.Name??b"missing";}
export function Hygiene(u:User|null):int{return u?.read(__kinmokuseiOptionalReceiver.IntSize)??0;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, ds, err := EmitGo(paths, "optional")
	if err != nil || len(ds) != 0 {
		t.Fatalf("emit: %v %v", err, ds)
	}
	mapped, ds, err := EmitGoWithSourceMap(paths, "optional")
	if err != nil || len(ds) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("map: %v %v", err, ds)
	}
	reference := `package reference
type User struct{Name string;N int;Next *User}
func Name(u *User)string{if u==nil{return "anonymous"};return u.Name}
func Chain(u *User)int{if u==nil||u.Next==nil{return -1};return u.Next.N}
func Tail(u *User)int{if u==nil{return -1};return u.N}
func Read(u *User,visit func(int))int{visit(1);if u==nil{visit(4);return -1};visit(2);return u.N+3}
func Index(xs []int,visit func(int))int{if xs==nil{return 9};visit(1);return xs[0]}
func Call(fn func(int)int,visit func(int))int{if fn==nil{return 9};visit(1);return fn(3)}
func Split(u *User)int{if u==nil{return -1};return u.N}
func Generic[T any](fn func()T,fallback T)T{if fn==nil{return fallback};return fn()}
`
	comparison := `package optional_test
import("fmt";"reflect";"strings";"strconv";"testing";g "optional-chaining.test";r "optional-chaining.test/reference")
func panicValue(call func())(result any){defer func(){result=recover()}();call();return nil}
func TestOptional(t *testing.T){
 for mode:=0;mode<3;mode++{
 var gu *g.User;var ru *r.User
 if mode>0{gu=g.NewUser("Alice",0);ru=&r.User{Name:"Alice"};if mode==2{gu.Next=g.NewUser("Bob",7);ru.Next=&r.User{Name:"Bob",N:7}}}
 if g.Name(gu)!=r.Name(ru)||g.Chain(gu)!=r.Chain(ru)||g.Tail(gu)!=r.Tail(ru)||g.Split(gu)!=r.Split(ru){t.Fatal(mode,"values")}
 if g.Reference(gu)!=gu{t.Fatal("reference identity")}
 var gs,rs []int;got:=g.Read(gu,func(n int){gs=append(gs,n)});want:=r.Read(ru,func(n int){rs=append(rs,n)});if got!=want||!reflect.DeepEqual(gs,rs){t.Fatal(got,want,gs,rs)}
 var touched []int;g.Touch(gu,func(n int){touched=append(touched,n)});if (mode==0&&len(touched)!=0)||(mode>0&&!reflect.DeepEqual(touched,[]int{0})){t.Fatal(touched)}
 if g.Hygiene(gu)!=map[bool]int{true:0,false:strconv.IntSize}[mode==0]{t.Fatal("capture hygiene")}
 if mode>0{if g.Force(gu)!="Alice"||g.AssertGroup(gu)!=0{t.Fatal("assertion")}}
 }
 for _,xs:=range [][]int{nil,{0,7}}{var gs,rs []int;got:=g.Index(xs,func(n int){gs=append(gs,n)});want:=r.Index(xs,func(n int){rs=append(rs,n)});if got!=want||!reflect.DeepEqual(gs,rs){t.Fatal(got,want,gs,rs)};slice:=g.Slice(xs);if xs==nil{if slice!=nil{t.Fatal(slice)}}else if !reflect.DeepEqual(slice,xs[1:2]){t.Fatal(slice)}}
 for _,fn:=range []func(int)int{nil,func(n int)int{return n*2}}{var gs,rs []int;got:=g.Call(fn,func(n int){gs=append(gs,n)});want:=r.Call(fn,func(n int){rs=append(rs,n)});if got!=want||!reflect.DeepEqual(gs,rs){t.Fatal(got,want,gs,rs)}}
 for _,fn:=range []func()int{nil,func()int{return 0}}{if got,want:=g.Generic(fn,9),r.Generic(fn,9);got!=want{t.Fatal(got,want)}}
 for _,call:=range []func(){func(){g.Force(nil)},func(){g.AssertGroup(nil)}}{p:=panicValue(call);if !strings.Contains(fmt.Sprint(p),"non-null assertion failed at entry.km:"){t.Fatal(p)}}
 if p:=panicValue(func(){g.Index([]int{},func(int){})});p==nil{t.Fatal("empty non-nil slice lost bounds check")}
 if g.Raw()!="missing"{t.Fatal(g.Raw())}
}
`
	runGeneratedGoDifferentialTest(t, root, "optional-chaining.test", generated, reference, comparison)
}
