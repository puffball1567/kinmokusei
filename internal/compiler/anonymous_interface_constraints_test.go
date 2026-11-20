package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnonymousInterfaceConstraintsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module anonymous-bounds.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Getter[E any] interface{Get() E}
type Numbers interface{~int}
type Callback func(int) int
type Caller interface{Call(Callback) int}
type WrongCaller struct{}
func (WrongCaller) Call(fn func(int)int)int{return fn(12)}
`,
		"models.km": `import go errors from "errors";
export alias View<E>=interface{read():E;};
export struct Number{public n:int;public function read():int{return this.n;}}
export struct Text{public text:string;public function read():string{return this.text;}}
export struct Nested{public text:string;public function read():interface{read():string;}{return Text{text:this.text};}}
export struct Pointer{public n:int;public pointer function read():int{return this.n;}}
export class Ref{constructor(public n:int){}public function read():Ref|null{if(this.n<0){return null;}return this;}}
export class TextRef{constructor(private text:string){}public function read():string{return this.text;}}
export struct Worker{
 public n:int;
 public function checked():Result<int>{if(this.n<0){return fail(errors.New("negative"));}return ok(this.n);}
 public function sum(...v:int[]):int{let n=this.n;for(const item of v){n+=item;}return n;}
 public function pair():(int,bstring){return this.n,b"ok";}
}
export struct Holder<T extends interface{read():int;}>{public v:T;public function read():int{return this.v.read();}}
export type Items<T extends interface{read():int;}>=distinct T[];
`,
		"entry.km": `import {View as Readable,Number,Text,Nested,Pointer,Ref,TextRef,Worker,Holder,Items} from "./models";
import go fmt from "fmt";
import go io from "io";
import go contracts from "anonymous-bounds.test/contracts";
alias RawView=interface{fmt.Stringer;};
function read<E,T extends interface{read():E;}>(v:T):E{return v.read();}
function forward<E,T extends Readable<E>>(v:T):E{return read(v);}
function checked<T extends interface{checked():Result<int>;}>(v:T):Result<int>{const f=v.checked;return f();}
function many<T extends interface{sum(...v:int[]):int;pair():(int,bstring);}>(v:T):int{const [n,s]=v.pair();const sum=v.sum;return sum(n,len(s));}
class Extract<E>{public function get<T extends interface{read():E;}>(v:T):E{return v.read();}}
function view<T extends Readable<string>>(v:T):interface{read():string;}{return v;}
const nilView:interface{read():string;}|null=null;
export function NilGlobal():boolean{return nilView===null;}
export function Run(n:int):int[]{const v=Number{n:n};let p=Pointer{n:n+1};const f=read<int,Number>;const xs=Items<Number>([v]);return [read(v),forward(v),f(v),read(&p),Holder<Number>{v:v}.read(),many(Worker{n:n}),new Extract<int>().get(v),read(xs[0])];}
export function TextValue(s:string):string{return read(Text{text:s});}
export function SpecializedText(s:string):string{return new Extract<string>().get(Text{text:s});}
export function Optional(n:int):boolean{return read(new Ref(n))===null;}
export function CheckedValue(n:int):Result<int>{return checked(Worker{n:n});}
export function Structural(s:string):string{const v=Readable<string>(view(Text{text:s}));const f=v.read;return f();}
export function NestedText(s:string):string{return read(Nested{text:s}).read();}
export function RawText(v:fmt.Stringer):bstring{return RawView(v).String();}
export function Arrow(s:string):string{const create=(s:string):interface{read():string;}=>new TextRef(s);const f=create(s).read;return f();}
export function NilValue(v:interface{read():string;}):string{return v.read();}
function readAndClose<T extends interface{io.Reader;io.ReadCloser;}>(v:T):(int,error){
 const buf:byte[]=[0,0];
 const [n,err]=v.Read(buf);
 if(err!=nil){return n,err;}
 return n+int(buf[0]),v.Close();
}
export function EmbeddedRead(v:interface{io.ReadCloser;}):(int,error){return readAndClose(v);}
export function EmbeddedGeneric(v:interface{contracts.Getter<int>;}):int{return v.Get();}
export function EmbeddedCallback(v:interface{contracts.Caller;}):int{return v.Call((x:int)=>x*2);}
export function EmbeddedView(s:string):string{
 const v:interface{Readable<string>;interface{read():string;};}=Text{text:s};
 return v.read();
}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(entry, "anonymousbounds")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(entry, "anonymousbounds")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(mapped.GoSource, generated) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source map err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import ("errors";"io")
type number struct{n int};func(v number)Read()int{return v.n}
type text struct{s string};func(v text)Read()string{return v.s}
type nested struct{s string};func(v nested)Read()interface{Read()string}{return text{v.s}}
type pointer struct{n int};func(v *pointer)Read()int{return v.n}
type ref struct{n int};func(v *ref)Read()*ref{if v.n<0{return nil};return v}
type textRef struct{s string};func(v *textRef)Read()string{return v.s}
type worker struct{n int}
func(v worker)Checked()(int,error){if v.n<0{return 0,errors.New("negative")};return v.n,nil}
func(v worker)Sum(xs ...int)int{n:=v.n;for _,x:=range xs{n+=x};return n}
func(v worker)Pair()(int,string){return v.n,"ok"}
type holder[T interface{Read()int}]struct{v T};func(v holder[T])Read()int{return v.v.Read()}
type items[T interface{Read()int}][]T
func read[E any,T interface{Read()E}](v T)E{return v.Read()}
func forward[E any,T interface{Read()E}](v T)E{return read[E](v)}
func checked[T interface{Checked()(int,error)}](v T)(int,error){f:=v.Checked;return f()}
func many[T interface{Sum(...int)int;Pair()(int,string)}](v T)int{n,s:=v.Pair();sum:=v.Sum;return sum(n,len(s))}
func view[T interface{Read()string}](v T)interface{Read()string}{return v}
func Run(n int)[]int{v:=number{n};p:=pointer{n+1};f:=read[int,number];xs:=items[number]{v};return []int{read[int](v),forward[int](v),f(v),read[int](&p),holder[number]{v}.Read(),many(worker{n}),read[int](v),read[int](xs[0])}}
func TextValue(s string)string{return read[string](text{s})}
func SpecializedText(s string)string{return read[string](text{s})}
func Optional(n int)bool{return read[*ref](&ref{n})==nil}
func CheckedValue(n int)(int,error){return checked(worker{n})}
func Structural(s string)string{v:=interface{Read()string}(view(text{s}));f:=v.Read;return f()}
func NestedText(s string)string{return read[interface{Read()string}](nested{s}).Read()}
func RawText(v interface{String()string})string{return interface{String()string}(v).String()}
func Arrow(s string)string{create:=func(s string)interface{Read()string}{return &textRef{s}};f:=create(s).Read;return f()}
func NilValue(v interface{Read()string})string{return v.Read()}
func NilGlobal()bool{var v interface{Read()string};return v==nil}
func readAndClose[T interface{io.Reader;io.ReadCloser}](v T)(int,error){
 buf:=make([]byte,2)
 n,err:=v.Read(buf)
 if err!=nil{return n,err}
 return n+int(buf[0]),v.Close()
}
func EmbeddedRead(v interface{io.ReadCloser})(int,error){return readAndClose(v)}
func EmbeddedGeneric(v interface{Get()int})int{return v.Get()}
type callback func(int)int
type Caller int
func (v Caller) Call(fn callback)int{return fn(int(v))}
func EmbeddedCallback(v interface{Call(callback)int})int{return v.Call(func(x int)int{return x*2})}
func EmbeddedView(s string)string{var v interface{Read()string}=text{s};return v.Read()}
`
	comparison := `package anonymousbounds_test
import("reflect";"strings";"testing";g "anonymous-bounds.test";r "anonymous-bounds.test/reference";"anonymous-bounds.test/contracts")
func panics(f func()string)(did bool){defer func(){did=recover()!=nil}();f();return}
type raw string;func(v raw)String()string{return string(v)}
type trackedReader struct{*strings.Reader; closed bool}
func (r *trackedReader) Close()error{r.closed=true;return nil}
type getter int
func (v getter) Get()int{return int(v)}
type caller int
func (v caller) Call(fn contracts.Callback)int{return fn(int(v))}
func TestContracts(t *testing.T){
 for _,n:=range []int{-10,0,17}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal("methods",n,a,b)};if g.Optional(n)!=r.Optional(n){t.Fatal("nullable",n)};a,ae:=g.CheckedValue(n);b,be:=r.CheckedValue(n);if a!=b||(ae==nil)!=(be==nil){t.Fatal("Result",a,ae,b,be)};if ae!=nil&&ae.Error()!=be.Error(){t.Fatal("failure",ae,be)}}
 for _,s:=range []string{"","osmanthus","金木犀"}{for _,pair:=range [][2]func(string)string{{g.TextValue,r.TextValue},{g.SpecializedText,r.SpecializedText},{g.Structural,r.Structural},{g.NestedText,r.NestedText},{g.Arrow,r.Arrow},{g.EmbeddedView,r.EmbeddedView}}{if pair[0](s)!=pair[1](s){t.Fatal("text contract",s)}}}
 if a,b:=g.RawText(raw("\xff")),r.RawText(raw("\xff"));a!=b{t.Fatal("raw Go interface conversion",a,b)}
 if !panics(func()string{return g.NilValue(nil)})||!panics(func()string{return r.NilValue(nil)}){t.Fatal("nil interface panic")}
 if !g.NilGlobal()||g.NilGlobal()!=r.NilGlobal(){t.Fatal("nil global binding")}
}
func TestEmbedding(t *testing.T){
 for _,v:=range []getter{-23,0,64}{if g.EmbeddedGeneric(v)!=r.EmbeddedGeneric(v){t.Fatal(v)}}
 for _,v:=range []caller{-13,0,19}{if g.EmbeddedCallback(v)!=r.EmbeddedCallback(r.Caller(v)){t.Fatal(v)}}
 for _,s:=range []string{"","abc","\xff"}{
  a,b:=&trackedReader{Reader:strings.NewReader(s)},&trackedReader{Reader:strings.NewReader(s)}
  n,e:=g.EmbeddedRead(a);want,err:=r.EmbeddedRead(b)
  if n!=want||e!=err||a.closed!=b.closed{t.Fatal(s,n,e,a.closed,want,err,b.closed)}
 }
}
`
	runGeneratedGoDifferentialTest(t, root, "anonymous-bounds.test", generated, reference, comparison)
	for _, example := range []struct{ embedded, want string }{
		{"contracts.Getter", "requires 1 type arguments"},
		{"contracts.Numbers", "embedding requires a Go method interface"},
	} {
		path := filepath.Join(root, "bad.km")
		input := `import go contracts from "anonymous-bounds.test/contracts"; alias Bad = interface { ` + example.embedded + `; };`
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
		_, diagnostics, err := EmitGo([]string{path}, "anonymousbounds")
		if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, example.want) {
			t.Fatalf("embed %s: err=%v diagnostics=%v", example.embedded, err, diagnostics)
		}
	}
	path := filepath.Join(root, "wrong.km")
	wrong := `import go contracts from "anonymous-bounds.test/contracts";
function use(v:interface{contracts.Caller;}):void{}
function wrong(v:contracts.WrongCaller):void{use(v);}`
	if err := os.WriteFile(path, []byte(wrong), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err = EmitGo([]string{path}, "anonymousbounds")
	if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "cannot use") {
		t.Fatalf("embedded callback identity: err=%v diagnostics=%v", err, diagnostics)
	}
}
