package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceConstraintCompositionMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `import go fmt from "fmt";
import go errors from "errors";
export constraint Readable<E>=Reader<E>&comparable;
export constraint Tagged<E>=Readable<E>&interface{tag():int;};
export constraint Exact=Reader<int>&Number;
export constraint Integer=Reader<int>&~int;
export constraint Printable=Reader<int>&fmt.Stringer;
export constraint Checked=ResultReader&comparable;
export constraint Counter=Getter&Setter&comparable;
export interface Reader<E>{function read():E;}
export interface ResultReader{function read():Result<int>;}
export interface Getter{get value():int;}
export interface Setter{set value(v:int):void;}
export interface Many{function sum(...values:int[]):int;function pair():(int,bstring);}
export constraint Multiple=Many&comparable;
export struct Number{public n:int;public function read():int{return this.n;}public function tag():int{return this.n+1;}public function String():bstring{return b"number";}}
export struct Text{public text:string;public function read():string{return this.text;}public function tag():int{return len(this.text);}}
export struct Pointer{public n:int;public pointer function read():int{return this.n;}}
export type Score=distinct int;
public function read(this:Score):int{return int(this);}
export struct Worker{public n:int;public function read():Result<int>{if(this.n<0){return fail(errors.New(b"negative"));}return ok(this.n);}public function sum(...values:int[]):int{let n=this.n;for(const v of values){n+=v;}return n;}public function pair():(int,bstring){return this.n,b"ok";}}
export class Count{constructor(private n:int){}public get value():int{return this.n;}public set value(v:int):void{this.n=v;}}
export class Maybe{constructor(public n:int){}public function read():Maybe|null{if(this.n<0){return null;}return this;}}
export struct Holder<T extends Readable<int>>{public v:T;public function read():int{return this.v.read();}}
export class Box<T extends Readable<int>>{constructor(private v:T){}public function read():int{return this.v.read();}}
`,
		"entry.km": `import {Readable as Bound,Tagged,Exact,Integer,Printable,Checked,Counter,Multiple,Reader,Number,Text,Pointer,Score,Worker,Count,Maybe,Holder,Box} from "./models";
function read<E,T extends Bound<E>>(v:T):E{const f=v.read;return f();}
function forward<E,T extends Bound<E>>(v:T):E{return read(v);}
function tagged<E,T extends Tagged<E>>(v:T):int{_=v.read();return v.tag();}
function exact<T extends Exact>(v:T):int{return v.read();}
function score<T extends Integer>(v:T):int{return v.read()+int(v);}
function printed<T extends Printable>(v:T):bstring{_=v.read();return v.String();}
function counter<T extends Counter>(v:T):int{v.value+=2;v.value++;return v.value;}
function many<T extends Multiple>(v:T):int{const pair=v.pair;const [n,s]=pair();const sum=v.sum;return sum(n,len(s));}
function checked<T extends Checked>(v:T):Result<int>{return v.read();}
class Extract<E>{public function get<T extends Bound<E>>(v:T):E{return v.read();}}
export function Run(n:int):int[]{const v=Number{n:n};let p=Pointer{n:n+1};const f:(v:Number)=>int=read;return [read(v),forward(v),f(v),read(&p),tagged(v),exact(v),score(Score(n)),counter(new Count(n)),many(Worker{n:n}),Holder<Number>{v:v}.read(),new Box<Number>(v).read(),new Extract<int>().get(v)];}
export function TextValue(s:string):string{const v=Text{text:s};_=tagged(v);return read(v);}
export function Specialized(s:string):string{return new Extract<string>().get(Text{text:s});}
export function Optional(n:int):boolean{return read(new Maybe(n))===null;}
export function Printed(n:int):bstring{return printed(Number{n:n});}
export function CheckedValue(n:int):Result<int>{return checked(Worker{n:n});}
export function Runtime(v:Reader<int>):int{return forward(v);}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, ds, err := EmitGo(paths, "composition")
	if err != nil || len(ds) != 0 {
		t.Fatalf("emit: %v %v", err, ds)
	}
	mapped, ds, err := EmitGoWithSourceMap(paths, "composition")
	if err != nil || len(ds) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped: %v %v", err, ds)
	}
	reference := `package reference
import("errors";"fmt")
type reader[E any]interface{Read()E}
type readable[E any]interface{reader[E];comparable}
type tagged[E any]interface{readable[E];Tag()int}
type exact interface{reader[int];number}
type integer interface{reader[int];~int}
type printable interface{reader[int];fmt.Stringer}
type checkedReader interface{Read()(int,error);comparable}
type counterReader interface{GetValue()int;SetValue(int);comparable}
type multiple interface{Sum(...int)int;Pair()(int,string);comparable}
type number struct{n int};func(v number)Read()int{return v.n};func(v number)Tag()int{return v.n+1};func(v number)String()string{return "number"}
type text struct{s string};func(v text)Read()string{return v.s};func(v text)Tag()int{return len(v.s)}
type pointer struct{n int};func(v *pointer)Read()int{return v.n}
type scoreType int;func(v scoreType)Read()int{return int(v)}
type count struct{n int};func(v *count)GetValue()int{return v.n};func(v *count)SetValue(n int){v.n=n}
type maybe struct{n int};func(v *maybe)Read()*maybe{if v.n<0{return nil};return v}
type worker struct{n int};func(v worker)Read()(int,error){if v.n<0{return 0,errors.New("negative")};return v.n,nil};func(v worker)Sum(values ...int)int{n:=v.n;for _,v:=range values{n+=v};return n};func(v worker)Pair()(int,string){return v.n,"ok"}
type holder[T readable[int]]struct{v T};func(v holder[T])Read()int{return v.v.Read()}
type box[T readable[int]]struct{v T};func(v *box[T])Read()int{return v.v.Read()}
func read[E any,T readable[E]](v T)E{f:=v.Read;return f()}
func forward[E any,T readable[E]](v T)E{return read(v)}
func tag[E any,T tagged[E]](v T)int{_=v.Read();return v.Tag()}
func exactValue[T exact](v T)int{return v.Read()}
func score[T integer](v T)int{return v.Read()+int(v)}
func printed[T printable](v T)string{_=v.Read();return v.String()}
func counter[T counterReader](v T)int{v.SetValue(v.GetValue()+2);v.SetValue(v.GetValue()+1);return v.GetValue()}
func many[T multiple](v T)int{pair:=v.Pair;n,s:=pair();sum:=v.Sum;return sum(n,len(s))}
func checked[T checkedReader](v T)(int,error){return v.Read()}
func Run(n int)[]int{v:=number{n};p:=pointer{n+1};var f func(number)int=read;return []int{read(v),forward(v),f(v),read(&p),tag(v),exactValue(v),score(scoreType(n)),counter(&count{n}),many(worker{n}),holder[number]{v}.Read(),(&box[number]{v}).Read(),read(v)}}
func TextValue(s string)string{v:=text{s};_=tag(v);return read(v)}
func Specialized(s string)string{return read(text{s})}
func Optional(n int)bool{return read(&maybe{n})==nil}
func Printed(n int)string{return printed(number{n})}
func CheckedValue(n int)(int,error){return checked(worker{n})}
func Runtime(n int)int{var v reader[int]=number{n};return forward(v)}
`
	comparison := `package composition_test
import("reflect";"testing";g "source-constraint-composition.test";r "source-constraint-composition.test/reference")
func TestComposition(t *testing.T){for _,n:=range []int{-5,0,14}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal("run",n,a,b)};if g.Optional(n)!=r.Optional(n)||g.Printed(n)!=r.Printed(n)||g.Runtime(g.Number{N:n})!=r.Runtime(n){t.Fatal("method",n)};a,ae:=g.CheckedValue(n);b,be:=r.CheckedValue(n);if a!=b||(ae==nil)!=(be==nil){t.Fatal("Result",a,ae,b,be)};if ae!=nil&&ae.Error()!=be.Error(){t.Fatal("failure",ae,be)}};for _,s:=range []string{"","hello","金木犀🍊"}{if g.TextValue(s)!=r.TextValue(s)||g.Specialized(s)!=r.Specialized(s){t.Fatal("text",s)}}}
`
	runGeneratedGoDifferentialTest(t, root, "source-constraint-composition.test", generated, reference, comparison)
}

func TestSourceConstraintCompositionGoBoundary(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"private method", `interface Marker{}constraint C=Marker&api.Hidden;`, "unexported Go method"},
		{"raw Go generic nullable", `class Item{}interface Marker{function tag():int;}constraint C<E>=Marker&api.Getter<E>;function use<T extends C<Item|null>>(v:T):void{}`, "must preserve source type information"},
		{"raw Go generic owner", `class Item{}interface Marker{function tag():int;}constraint C<E>=Marker&api.Getter<E>;class Host<E>{public function use<T extends C<E>>(v:T):void{}}class S{public function tag():int{return 1;}public function getValue():Item|null{return null;}}function bad():void{new Host<Item|null>().use(new S());}`, "must preserve source type information"},
		{"independent Go method", `class Item{}interface Reader<E>{function read():E;}constraint C<E>=Reader<E>&api.Total;struct S{public function read():Item|null{return null;}public function sum(...n:int[]):int{return len(n);}}function use<E,T extends C<E>>(v:T):E{_=v.Sum();return v.read();}const n:Item|null=use(S{});`, ""},
		{"reused raw Go guard", `class Item{}interface Marker{function tag():int;}constraint A<E>=Marker&api.Getter<E>;constraint C<E>=A<E>&comparable;function use<T extends C<Item|null>>(v:T):void{}`, "must preserve source type information"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := writeNativeConstraintFixture(t)
			path := filepath.Join(root, "entry.km")
			if err := os.WriteFile(path, []byte(`import go api from "native-constraint-methods.test/contracts";`+test.input), 0o644); err != nil {
				t.Fatal(err)
			}
			_, ds, err := EmitGo([]string{path}, "boundary")
			text := diagnosticsText(ds)
			if err != nil || test.want == "" && text != "" || test.want != "" && !strings.Contains(text, test.want) {
				t.Fatalf("emit: %v %v want=%q", err, ds, test.want)
			}
		})
	}
}
