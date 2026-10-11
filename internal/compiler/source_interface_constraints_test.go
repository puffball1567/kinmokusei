package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceInterfaceConstraintsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `export interface Reader<E>{function read():E;}
export interface TextReader extends Reader<string>{}
export interface Checked{function read():Result<int>;}
export interface Many{function sum(...values:int[]):int;function pair():(int,bstring);}
export interface Counter{get value():int;set value(v:int):void;}
export struct Number{public n:int;public function read():int{return this.n;}}
export struct Text{public text:string;public function read():string{return this.text;}}
export struct Pointer{public n:int;public pointer function read():int{return this.n;}}
export class Count{constructor(private n:int){}public get value():int{return this.n;}public set value(v:int):void{this.n=v;}}
export class Maybe{constructor(public n:int){}public function read():Maybe|null{if(this.n<0){return null;}return this;}}
export struct Worker{public n:int;public function read():Result<int>{return ok(this.n);}public function sum(...values:int[]):int{let n=this.n;for(const v of values){n+=v;}return n;}public function pair():(int,bstring){return this.n,b"ok";}}
export class Box<T extends Reader<int>>{constructor(private v:T){}public function read():int{return this.v.read();}}
export struct Holder<T extends Reader<int>>{public v:T;public function read():int{return this.v.read();}}
`,
		"entry.km": `import {Reader as Readable,TextReader,Checked,Many,Counter,Number,Text,Pointer,Count,Maybe,Worker,Box,Holder} from "./models";
function read<E,T extends Readable<E>>(v:T):E{return v.read();}
function forward<E,T extends Readable<E>>(v:T):E{return read(v);}
function text<T extends TextReader>(v:T):string{const f=v.read;return f();}
function checked<T extends Checked>(v:T):Result<int>{return v.read();}
function many<T extends Many>(v:T):int{const pair=v.pair;const [n,s]=pair();const sum=v.sum;return sum(n,len(s));}
function counter<T extends Counter>(v:T):int{v.value+=2;v.value++;return v.value;}
class Extract<E>{public function get<T extends Readable<E>>(v:T):E{return v.read();}}
export function Run(n:int):int[]{const v=Number{n:n};let p=Pointer{n:n+1};const f=read<int,Number>;return [read(v),forward(v),f(v),read(&p),new Box<Number>(v).read(),Holder<Number>{v:v}.read(),counter(new Count(n)),many(Worker{n:n}),new Extract<int>().get(v)];}
export function TextValue(s:string):string{return text(Text{text:s});}
export function SpecializedText(s:string):string{return new Extract<string>().get(Text{text:s});}
export function Optional(n:int):boolean{return read(new Maybe(n))===null;}
export function CheckedValue(n:int):Result<int>{return checked(Worker{n:n});}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(entry, "sourcebounds")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(entry, "sourcebounds")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(mapped.GoSource, generated) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source map err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type reader[E any]interface{Read()E}
type textReader interface{reader[string]}
type checkedReader interface{Read()(int,error)}
type manyReader interface{Sum(...int)int;Pair()(int,string)}
type counterReader interface{GetValue()int;SetValue(int)}
type number struct{n int};func(v number)Read()int{return v.n}
type text struct{s string};func(v text)Read()string{return v.s}
type pointer struct{n int};func(v *pointer)Read()int{return v.n}
type count struct{n int};func(v *count)GetValue()int{return v.n};func(v *count)SetValue(n int){v.n=n}
type maybe struct{n int};func(v *maybe)Read()*maybe{if v.n<0{return nil};return v}
type worker struct{n int};func(v worker)Read()(int,error){return v.n,nil};func(v worker)Sum(values ...int)int{n:=v.n;for _,v:=range values{n+=v};return n};func(v worker)Pair()(int,string){return v.n,"ok"}
type box[T reader[int]]struct{v T};func(v *box[T])Read()int{return v.v.Read()}
type holder[T reader[int]]struct{v T};func(v holder[T])Read()int{return v.v.Read()}
func read[E any,T reader[E]](v T)E{return v.Read()}
func forward[E any,T reader[E]](v T)E{return read(v)}
func textValue[T textReader](v T)string{f:=v.Read;return f()}
func checked[T checkedReader](v T)(int,error){return v.Read()}
func many[T manyReader](v T)int{pair:=v.Pair;n,s:=pair();sum:=v.Sum;return sum(n,len(s))}
func counter[T counterReader](v T)int{v.SetValue(v.GetValue()+2);v.SetValue(v.GetValue()+1);return v.GetValue()}
func Run(n int)[]int{v:=number{n};p:=pointer{n+1};f:=read[int,number];return []int{read(v),forward(v),f(v),read(&p),(&box[number]{v}).Read(),holder[number]{v}.Read(),counter(&count{n}),many(worker{n}),read(v)}}
func TextValue(s string)string{return textValue(text{s})}
func SpecializedText(s string)string{return read(text{s})}
func Optional(n int)bool{return read(&maybe{n})==nil}
func CheckedValue(n int)(int,error){return checked(worker{n})}
`
	comparison := `package sourcebounds_test
import("reflect";"testing";g "source-interface-constraints.test";r "source-interface-constraints.test/reference")
func TestBounds(t *testing.T){for _,n:=range []int{-5,0,14}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal("run",n,a,b)};if g.Optional(n)!=r.Optional(n){t.Fatal("nullable",n)};a,ae:=g.CheckedValue(n);b,be:=r.CheckedValue(n);if a!=b||(ae==nil)!=(be==nil){t.Fatal("Result",a,ae,b,be)}};for _,s:=range []string{"","hello","金木犀🍊"}{if g.TextValue(s)!=r.TextValue(s)||g.SpecializedText(s)!=r.SpecializedText(s){t.Fatal("text",s)}}}
`
	runGeneratedGoDifferentialTest(t, root, "source-interface-constraints.test", generated, reference, comparison)
}

func TestSourceInterfaceConstraintLinkedRejection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"models.km": `export interface Reader{function read():string;}export struct Raw{public function read():bstring{return b"raw";}}`,
		"entry.km":  `import {Reader as Bound,Raw} from "./models";function read<T extends Bound>(v:T):string{return v.read();}const value=read(Raw{});`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "sourcebounds")
	if err != nil || len(diagnostics) == 0 || len(generated) != 0 {
		t.Fatalf("err=%v diagnostics=%v generated=%s", err, diagnostics, generated)
	}
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "does not satisfy") {
			return
		}
	}
	t.Fatalf("missing source-contract diagnostic: %v", diagnostics)
}
