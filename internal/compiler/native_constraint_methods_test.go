package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeNativeConstraintFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"go.mod": "module native-constraint-methods.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Getter[E any] interface{GetValue()E}
type Setter[E any] interface{SetValue(E)}
type Total interface{Sum(...int)int}
type Hidden interface{hidden()}
type Text interface{SetValue(string)}
type Texts interface{SetValue([]string)}
type Callback interface{SetValue(func(string))}
type Errors interface{GetValue()error}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestNativeConstraintMethodsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := writeNativeConstraintFixture(t)
	files := map[string]string{
		"model.km": `
export interface View<T>{get value():T;set value(v:T);}
export class Property<T> implements View<T>{constructor(private raw:T){}public get value():T{return this.raw;}public set value(v:T){this.raw=v;}}
export class Base<T>{constructor(private raw:T){}public virtual function getValue():T{return this.raw;}public virtual function setValue(v:T):void{this.raw=v;}}
export class Child<T> extends Base<T>{constructor(v:T){super(v);}public override function getValue():T{return super.getValue();}public override function setValue(v:T):void{super.setValue(v);}}
export struct Cell<T>{public value:T;public function getValue():T{return this.value;}public pointer function setValue(v:T):void{this.value=v;}}
export interface Read<T>{function getValue():T;}
export interface Derived<T> extends Read<T>{}
export class Stream{public function read(bytes:byte[]):Result<int>{if(len(bytes)===0){return ok(0);}bytes[0]=42;return ok(1);}public function close():Result<void>{return ok();}}
export class Total{public function sum(...values:int[]):int{let result=0;for(const value of values){result+=value;}return result;}}
`,
		"entry.km": `
import {View,Property,Base,Child,Cell,Derived,Stream,Total} from "./model";
import go api from "native-constraint-methods.test/contracts";
import go fmt from "fmt";
import go io from "io";
import go strconv from "strconv";
constraint Access<E>=api.Getter<E>&api.Setter<E>;
constraint Reader=io.Reader&io.Closer;
constraint Named=fmt.Stringer&comparable;
constraint Number=fmt.Stringer&~int;
function access<E,T extends Access<E>>(cell:T,value:E):E{const set=cell.SetValue;set(value);const get=cell.GetValue;return get();}
function read<E,T extends api.Getter<E>>(cell:T):E{return cell.GetValue();}
function show<T extends Named>(value:T):bstring{return value.String();}
function twice<T extends Number>(value:T):bstring{return (value+value).String();}
function close<T extends io.Closer>(value:T):error{return value.Close();}
function readOne<T extends Reader>(value:T,bytes:byte[]):Result<int>{const [count,problem]=value.Read(bytes);if(problem!==nil){return fail(problem);}const closed=value.Close();if(closed!==nil){return fail(closed);}return ok(count);}
function sum<T extends api.Total>(value:T,...values:int[]):int{return value.Sum(values...);}
class Holder<E,T extends Access<E>>{constructor(private cell:T){}public function update(value:E):E{return access(this.cell,value);}}
type Score=distinct int;
public function string(this:Score):bstring{return strconv.Itoa(int(this));}
class Label{constructor(private raw:bstring){}public function string():bstring{return this.raw;}}
function Values(n:int):int[]{const property=new Property<int>(0);const contract:View<int>=property;const a=access(contract,n);const holder=new Holder<int,Property<int>>(property);const b=holder.update(n+1);const child=new Child<int>(0);const c=access(child,n+2);const base:Base<int>=child;const d=read(base);let cell=Cell<int>{value:0};const e=access(&cell,n+3);const f=read(cell);return [a,b,c,d,e,f,cell.value];}
function SourceInterface(value:Derived<int>):int{return read(value);}
function Format(n:int):bstring[]{return [show(new Label(strconv.Itoa(n))),twice(Score(n))];}
function Raw(value:bstring):bstring{return access(new Property<bstring>(b""),value);}
function Closed():boolean{return close(new Stream())===nil;}
function ReadOne(bytes:byte[]):Result<int>{return readOne(new Stream(),bytes);}
function Sum(values:int[]):int{return sum(new Total(),values...);}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "nativemethods")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "strconv"
type Access[E any]interface{GetValue()E;SetValue(E)}
type Property[T any]struct{raw T}
func(p *Property[T])GetValue()T{return p.raw}
func(p *Property[T])SetValue(v T){p.raw=v}
type Cell[T any]struct{value T}
func(c Cell[T])GetValue()T{return c.value}
func(c *Cell[T])SetValue(v T){c.value=v}
func access[E any,T Access[E]](cell T,value E)E{set:=cell.SetValue;set(value);get:=cell.GetValue;return get()}
func read[E any,T interface{GetValue()E}](cell T)E{return cell.GetValue()}
func Values(n int)[]int{p:=new(Property[int]);a:=access(p,n);b:=access(p,n+1);child:=new(Property[int]);c:=access(child,n+2);d:=read(child);cell:=Cell[int]{};e:=access(&cell,n+3);f:=read(cell);return []int{a,b,c,d,e,f,cell.value}}
func SourceInterface(value interface{GetValue()int})int{return read(value)}
func Format(n int)[]string{return []string{strconv.Itoa(n),strconv.Itoa(n+n)}}
func Raw(value string)string{return access(new(Property[string]),value)}
func Closed()bool{return true}
func ReadOne(bytes []byte)(int,error){if len(bytes)==0{return 0,nil};bytes[0]=42;return 1,nil}
func Sum(values []int)int{n:=0;for _,v:=range values{n+=v};return n}
`
	comparison := `package nativemethods_test
import("reflect";"testing";generated "native-constraint-methods.test";reference "native-constraint-methods.test/reference")
type Read int
func(r Read)GetValue()int{return int(r)}
func TestBehavior(t *testing.T){
for _,n:=range []int{-10,0,11}{if got,want:=generated.Values(n),reference.Values(n);!reflect.DeepEqual(got,want){t.Errorf("values=%v want=%v",got,want)};if got,want:=generated.Format(n),reference.Format(n);!reflect.DeepEqual(got,want){t.Errorf("format=%v want=%v",got,want)};if got,want:=generated.SourceInterface(Read(n)),reference.SourceInterface(Read(n));got!=want{t.Errorf("interface=%v want=%v",got,want)}}
for _,text:=range []string{"","hello","金木犀",string([]byte{255})}{if got,want:=generated.Raw(text),reference.Raw(text);got!=want{t.Errorf("raw=%q want=%q",got,want)}}
if got,want:=generated.Closed(),reference.Closed();got!=want{t.Errorf("close=%v want=%v",got,want)}
for _,size:=range []int{0,1,4}{a,b:=make([]byte,size),make([]byte,size);got,ge:=generated.ReadOne(a);want,we:=reference.ReadOne(b);if got!=want||(ge==nil)!=(we==nil)||!reflect.DeepEqual(a,b){t.Errorf("read=(%v,%v,%v) want=(%v,%v,%v)",got,ge,a,want,we,b)}}
for _,values:=range [][]int{nil,{}, {1,-2,3}}{if got,want:=generated.Sum(values),reference.Sum(values);got!=want{t.Errorf("sum=%v want=%v",got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "native-constraint-methods.test", generated, reference, comparison)
}

func TestNativeConstraintMethodsRejectErasedContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, bound, method, argument string }{
		{"text input", "api.Text", "public function setValue(v:string):void{}", "new Label()"},
		{"text slice", "api.Texts", "public function setValue(v:string[]):void{}", "new Label()"},
		{"text callback", "api.Callback", "public function setValue(v:(text:string)=>void):void{}", "new Label()"},
		{"private contract", "api.Hidden", "public function hidden():void{}", "new Label()"},
		{"nullable result", "api.Errors", "public function getValue():error|null{return null;}", "new Label()"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := writeNativeConstraintFixture(t)
			input := `import go api from "native-constraint-methods.test/contracts";function keep<T extends ` + test.bound + `>(v:T):T{return v;}class Label{` + test.method + `}function bad():void{keep(` + test.argument + `);}`
			path := filepath.Join(root, "entry.km")
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, err := EmitGo([]string{path}, "invalid")
			if err != nil || !strings.Contains(diagnosticsText(diagnostics), "does not satisfy") {
				t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
			}
		})
	}
}

func TestNativeConstraintMethodsRejectConflictingInference(t *testing.T) {
	t.Parallel()
	root := writeNativeConstraintFixture(t)
	path := filepath.Join(root, "entry.km")
	input := `import go api from "native-constraint-methods.test/contracts";
constraint Both<E>=api.Getter<E>&api.Setter<E>;
function read<E,T extends Both<E>>(value:T):E{return value.GetValue();}
class Label{public function getValue():int{return 1;}public function setValue(v:bstring):void{}}
function bad():void{const value=read(new Label());}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "invalid")
	if err != nil || len(generated) != 0 || !strings.Contains(diagnosticsText(diagnostics), "cannot infer type argument E") {
		t.Fatalf("err=%v diagnostics=%v generated=%s", err, diagnostics, generated)
	}
}
