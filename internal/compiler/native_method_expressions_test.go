package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeMethodExpressionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `import go errors from "errors";
export struct Counter {
 public value:int;
 public function copy(n:int):int { this.value+=n; return this.value; }
 public pointer function set(n:int):void { this.value=n; }
 public function read():int { return this.value; }
 public pointer function isNil():boolean { return this===nil; }
 public function sum(...xs:int[]):int { let n=this.value; for(const x of xs){n+=x;}return n; }
 public function pair():(int,bstring) { return this.value,b"ok"; }
 public function checked():Result<int> { if(this.value<0){return fail(errors.New("negative"));}return ok(this.value); }
}
export alias Alias=Counter;
export struct Box<T>{public item:T;public function read():T{return this.item;}}
export alias IntBox=Box<int>;
export type Numbers<T>=distinct T[];
public function first<T>(this:Numbers<T>):T{return this[0];}
export alias IntNumbers=Numbers<int>;
export interface Echo<T>{function echo(value:T):T;}
export alias IntEcho=Echo<int>;
export class EchoImpl implements Echo<int>{public function echo(value:int):int{return value;}}
export type Number=distinct int;
public function add(this:Number,n:int):Number{return Number(int(this)+n);}
public function set(this:*Number,n:int):void{*this=Number(n);}
export interface Reader{function read():int;}
export interface DerivedReader extends Reader{}
export class Dynamic implements DerivedReader{constructor(public value:int){}public function read():int{return this.value*2;}}
export function callback():(v:Counter,n:int)=>int{return Counter.copy;}
`,
		"entry.km": `import {Counter,Alias,IntBox,IntNumbers,IntEcho,EchoImpl,Number,Reader,DerivedReader,Dynamic,callback} from "./models";
export function Run(n:int):int[]{
 let v=Counter{value:n};const copy=Counter.copy;const pointerRead=(*Counter).read;
 const old=copy(v,3);(*Alias).set(&v,n+5);const aliasRead=Alias.read;
 const sum=Counter.sum;const xs=[1,2];const [pairValue,label]=Counter.pair(v);
 let defined=Number(n);const add=Number.add;(*Number).set(&defined,n+7);
 const box=IntBox{item:n+8};const get=IntBox.read;
 const sequence=IntNumbers([n+9]);const first=IntNumbers.first;
 const echo:IntEcho=new EchoImpl();const echoCall=IntEcho.echo;
 const reader:DerivedReader=new Dynamic(n);const read=Reader.read;const inherited=DerivedReader.read;
 return [old,v.value,pointerRead(&v),aliasRead(v),sum(v,xs...),pairValue,len(label),int(add(defined,2)),int(defined),get(box),first(sequence),echoCall(echo,n+10),read(reader),inherited(reader),callback()(v,4)];
}
export function Checked(n:int):Result<int>{const read=Counter.checked;return read(Counter{value:n});}
export function NilPointer():boolean{return (*Counter).isNil(nil);}
export function NilValue():int{return (*Counter).read(nil);}
export function NilInterface():int{const v:Reader=nil;return Reader.read(v);}
export function Evaluation():int[]{
 let order=0;let v=Counter{value:1};const receiver=():*Counter=>{order=order*10+1;return &v;};const argument=():int=>{order=order*10+2;return 8;};
 (*Counter).set(receiver(),argument());defer (*Counter).set(&v,100);
 const task=go Counter.read(v);return [order,v.value,await task];
}
export function Shadow():int{const Counter=Counter{value:9};return Counter.read();}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "methods")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{filepath.Join(root, "entry.km")}, "methods")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(mapped.GoSource, generated) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source-mapped emission: err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "errors"
type counter struct{value int}
func(v counter)copy(n int)int{v.value+=n;return v.value}
func(v *counter)set(n int){v.value=n}
func(v counter)read()int{return v.value}
func(v *counter)isNil()bool{return v==nil}
func(v counter)sum(xs ...int)int{n:=v.value;for _,x:=range xs{n+=x};return n}
func(v counter)pair()(int,string){return v.value,"ok"}
func(v counter)checked()(int,error){if v.value<0{return 0,errors.New("negative")};return v.value,nil}
type alias=counter
type box[T any]struct{item T}
func(v box[T])read()T{return v.item}
type intBox=box[int]
type numbers[T any][]T
func(v numbers[T])first()T{return v[0]}
type intNumbers=numbers[int]
type echo[T any]interface{echo(T)T}
type intEcho=echo[int]
type echoImpl struct{}
func(*echoImpl)echo(n int)int{return n}
type number int
func(v number)add(n int)number{return number(int(v)+n)}
func(v *number)set(n int){*v=number(n)}
type reader interface{read()int}
type derivedReader interface{reader}
type dynamic struct{value int}
func(v *dynamic)read()int{return v.value*2}
func callback()func(counter,int)int{return counter.copy}
func Run(n int)[]int{
 v:=counter{n};copy:=counter.copy;pointerRead:=(*counter).read;old:=copy(v,3);(*alias).set(&v,n+5);aliasRead:=alias.read
 sum:=counter.sum;xs:=[]int{1,2};num,label:=counter.pair(v)
 defined:=number(n);add:=number.add;(*number).set(&defined,n+7);b:=intBox{n+8};get:=intBox.read
 sequence:=intNumbers{n+9};first:=intNumbers.first;var echo intEcho=&echoImpl{};echoCall:=intEcho.echo
 var value derivedReader=&dynamic{n};read:=reader.read;inherited:=derivedReader.read
 return []int{old,v.value,pointerRead(&v),aliasRead(v),sum(v,xs...),num,len(label),int(add(defined,2)),int(defined),get(b),first(sequence),echoCall(echo,n+10),read(value),inherited(value),callback()(v,4)}
}
func Checked(n int)(int,error){read:=counter.checked;return read(counter{n})}
func NilPointer()bool{return (*counter).isNil(nil)}
func NilValue()int{return (*counter).read(nil)}
func NilInterface()int{var v reader;return reader.read(v)}
func Evaluation()[]int{order:=0;v:=counter{1};receiver:=func()*counter{order=order*10+1;return &v};argument:=func()int{order=order*10+2;return 8};(*counter).set(receiver(),argument());defer (*counter).set(&v,100);result:=make(chan int,1);snapshot:=v;go func(){result<-counter.read(snapshot)}();return []int{order,v.value,<-result}}
func Shadow()int{counter:=counter{9};return counter.read()}
`
	comparison := `package methods_test
import("reflect";"testing";g "native-method-expressions.test";r "native-method-expressions.test/reference")
func panics(f func()int)(did bool){defer func(){did=recover()!=nil}();f();return}
func TestMethods(t *testing.T){
 for _,n:=range []int{-10,0,17}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal("methods",n,a,b)};a,ae:=g.Checked(n);b,be:=r.Checked(n);if a!=b||(ae==nil)!=(be==nil){t.Fatal("result",a,ae,b,be)};if ae!=nil&&ae.Error()!=be.Error(){t.Fatal("error",ae,be)}}
 if !g.NilPointer()||g.NilPointer()!=r.NilPointer(){t.Fatal("nil pointer")}
 for _,pair:=range [][2]func()int{{g.NilValue,r.NilValue},{g.NilInterface,r.NilInterface}}{if !panics(pair[0])||!panics(pair[1]){t.Fatal("nil panic")}}
 if a,b:=g.Evaluation(),r.Evaluation();!reflect.DeepEqual(a,b){t.Fatal("evaluation",a,b)}
 if g.Shadow()!=r.Shadow(){t.Fatal("lexical shadowing")}
}
`
	runGeneratedGoDifferentialTest(t, root, "native-method-expressions.test", generated, reference, comparison)
}

func TestNativeMethodExpressionVisibilityAcrossModules(t *testing.T) {
	t.Parallel()
	for _, pointer := range []bool{false, true} {
		root := t.TempDir()
		declaration := `export struct S{private function hidden():int{return 1;}}`
		receiver := "S"
		if pointer {
			receiver = "(*S)"
		}
		for name, contents := range map[string]string{"models.km": declaration, "entry.km": `import {S} from "./models";const read=` + receiver + `.hidden;`} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "methods")
		if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "is private") {
			t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
		}
	}
}
