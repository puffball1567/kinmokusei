package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericSliceBuiltinsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module generic-slice-builtins.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Slice[E any] interface{~[]E}
type Bytes interface{~[]byte}
type Text interface{~string}
`,
		"bounds.km": `import go contracts from "generic-slice-builtins.test/contracts";
export constraint Slice<E>=contracts.Slice<E>;
export function add<E,S extends Slice<E>>(values:S,element:E):S{return append(values,element);}
export function join<E,D extends Slice<E>,S extends Slice<E>>(destination:D,source:S):D{return append(destination,source...);}
export function transfer<E,D extends Slice<E>,S extends Slice<E>>(destination:D,source:S):int{return copy(destination,source);}
`,
		"entry.km": `import {Slice,add,join,transfer} from "./bounds";
import go contracts from "generic-slice-builtins.test/contracts";
import go net from "net";
type Numbers=distinct int[];
type Other=distinct int[];
type Text=distinct string;
constraint Named=Numbers|Other;
function repeat<S extends Named>(values:S):S{return append(values,values...);}
function unchanged<S extends Slice<int>>(values:S):S{return append(values);}
function appendText<D extends contracts.Bytes,S extends contracts.Text>(destination:D,source:S):D{return append(destination,source...);}
function copyText<D extends contracts.Bytes,S extends contracts.Text>(destination:D,source:S):int{return copy(destination,source);}
class Buffer<E,S extends Slice<E>>{constructor(private values:S){}public function add(element:E):void{this.values=append(this.values,element);}public function read():S{return this.values;}}
class Copier<E>{public function transfer<D extends Slice<E>,S extends Slice<E>>(destination:D,source:S):int{return copy(destination,source);}public function add<S extends Slice<E>>(values:S,element:E):S{return append(values,element);}}
class Base{constructor(public value:int){}public virtual function read():int{return this.value;}}
class Child extends Base{constructor(value:int){super(value);}public override function read():int{return this.value*2;}}
constraint Bases=~Base[];
alias Maybe=Base|null;
function child<S extends Bases>(values:S,value:int):S{return append(values,new Child(value));}
alias Callback=(value:int)=>int;
constraint Functions=~Callback[];
function callbacks<S extends Functions>(values:S):S{return append(values,(n)=>n+1);}
alias Point={x:int};
constraint Points=~Point[];
function points<S extends Points>(values:S):S{return append(values,{x:7});}
export function Contexts():int{const fs:Callback[]=[];const ps:Point[]=[];return callbacks(fs)[0](4)+points(ps)[0].x;}
export function Aliasing():int[]{const values=Numbers(makeSlice<int>(2,6));values[0]=1;values[1]=2;const suffix=Other([3,4]);const appended=join(values,suffix);appended[0]=7;const grown=add(appended[:4:4],9);grown[0]=99;return [len(values),len(appended),values[0],appended[2],appended[3],len(grown),grown[0],grown[4],cap(values)];}
export function Overlap():int[]{const values=Numbers([1,2,3,4,5]);const count=transfer(values[1:],values[:4]);const result=join(values[:2],values[1:4]);return append(result,count);}
export function Bytes():byte[]{const initial=net.IP(makeSlice<byte>(2));initial[0]=1;initial[1]=2;const text=Text("温泉");const appended=appendText(initial,text);const destination=makeSlice<byte>(8);const count=copyText(destination,text);const copied=transfer(destination[6:],appended[:2]);return append(destination,byte(count),byte(copied));}
export function Objects():int[]{const values:Base[]=[];const result=child(values,3);const direct=append(result,new Child(4));let same=0;if(direct[0]===result[0]){same=1;}const box=new Buffer<Maybe,Maybe[]>([]);box.add(new Child(5));box.add(null);const destination:Maybe[]=[null,null];const copied=new Copier<Maybe>().transfer(destination,box.read());let read=0;const first=destination[0];if(first!==null){read=first.read();}const further=new Copier<Base>().add(direct,new Child(6));return [result[0].read(),direct[1].read(),read,copied,len(further),further[2].read(),same];}
export function Nested():int{const child=new Child(3);const values:Base[][]=[[child]];const destination:Base[][]=[[]];const copied=transfer(destination,values);destination[0][0].value=7;return values[0][0].read()+copied;}
export function NilAndNamed():int[]{const values:Numbers=nil;const zero=unchanged(values);const count=transfer(zero,Other([1]));let nilFlag=0;if(zero===nil){nilFlag=1;}const first=repeat(Numbers([2,3]));const second=repeat(Other([4]));return [nilFlag,count,len(first),first[2],len(second),second[1]];}
export function Order():int[]{let trace=0;const first=():Numbers=>{trace=trace*10+1;return Numbers([1]);};const item=():int=>{trace=trace*10+2;return 2;};const second=():Other=>{trace=trace*10+3;return Other([3]);};const a=add(first(),item());const b=join(first(),second());const count=transfer(first(),second());return [trace,len(a),len(b),count];}
export function Panic():int{const values:Numbers=nil;const missing:int[]=[];return len(add(values,missing[0]));}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericslices")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "net"
type numbers []int
type other []int
type text string
func add[E any,S ~[]E](values S,element E)S{return append(values,element)}
func join[E any,D ~[]E,S ~[]E](destination D,source S)D{return append(destination,source...)}
func transfer[E any,D ~[]E,S ~[]E](destination D,source S)int{return copy(destination,source)}
func repeat[S interface{numbers|other}](values S)S{return append(values,values...)}
func unchanged[S ~[]int](values S)S{return append(values)}
func appendText[D ~[]byte,S ~string](destination D,source S)D{return append(destination,source...)}
func copyText[D ~[]byte,S ~string](destination D,source S)int{return copy(destination,source)}
func callbacks[S ~[]func(int)int](values S)S{return append(values,func(n int)int{return n+1})}
func points[S ~[]struct{x int}](values S)S{return append(values,struct{x int}{x:7})}
func Contexts()int{return callbacks([]func(int)int{})[0](4)+points([]struct{x int}{})[0].x}
func Aliasing()[]int{values:=numbers(make([]int,2,6));values[0]=1;values[1]=2;suffix:=other{3,4};appended:=join(values,suffix);appended[0]=7;grown:=add(appended[:4:4],9);grown[0]=99;return []int{len(values),len(appended),values[0],appended[2],appended[3],len(grown),grown[0],grown[4],cap(values)}}
func Overlap()[]int{values:=numbers{1,2,3,4,5};count:=transfer(values[1:],values[:4]);result:=join(values[:2],values[1:4]);return append(result,count)}
func Bytes()[]byte{initial:=net.IP{1,2};s:=text("温泉");appended:=appendText(initial,s);destination:=make([]byte,8);count:=copyText(destination,s);copied:=transfer(destination[6:],appended[:2]);return append(destination,byte(count),byte(copied))}
type reader interface{read()int;set(int)}
type base struct{value int}
func (b *base)read()int{return b.value}
func (b *base)set(n int){b.value=n}
type child struct{base}
func (c *child)read()int{return c.value*2}
func Objects()[]int{values:=[]reader{};result:=append(values,&child{base{3}});direct:=append(result,&child{base{4}});same:=0;if direct[0]==result[0]{same=1};box:=[]reader{&child{base{5}},nil};destination:=[]reader{nil,nil};copied:=copy(destination,box);read:=0;if destination[0]!=nil{read=destination[0].read()};further:=append(direct,&child{base{6}});return []int{result[0].read(),direct[1].read(),read,copied,len(further),further[2].read(),same}}
func Nested()int{c:=&child{base{3}};values:=[][]reader{{c}};destination:=[][]reader{{}};copied:=transfer(destination,values);destination[0][0].set(7);return values[0][0].read()+copied}
func NilAndNamed()[]int{var values numbers;zero:=unchanged(values);count:=transfer(zero,other{1});flag:=0;if zero==nil{flag=1};first:=repeat(numbers{2,3});second:=repeat(other{4});return []int{flag,count,len(first),first[2],len(second),second[1]}}
func Order()[]int{trace:=0;first:=func()numbers{trace=trace*10+1;return numbers{1}};item:=func()int{trace=trace*10+2;return 2};second:=func()other{trace=trace*10+3;return other{3}};a:=add(first(),item());b:=join(first(),second());count:=transfer(first(),second());return []int{trace,len(a),len(b),count}}
func Panic()int{var values numbers;missing:=[]int{};return len(add(values,missing[0]))}
`
	comparison := `package genericslices_test
import("testing";"reflect";g "generic-slice-builtins.test";r "generic-slice-builtins.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestSlices(t *testing.T){for _,pair:=range [][2][]int{{g.Aliasing(),r.Aliasing()},{g.Overlap(),r.Overlap()},{g.Objects(),r.Objects()},{g.NilAndNamed(),r.NilAndNamed()},{g.Order(),r.Order()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatalf("got %v want %v",pair[0],pair[1])}};if !reflect.DeepEqual(g.Bytes(),r.Bytes())||g.Nested()!=r.Nested()||g.Contexts()!=r.Contexts(){t.Fatal("bytes/nested/contexts")};if !panics(func(){g.Panic()})||!panics(func(){r.Panic()}){t.Fatal("panic")}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-slice-builtins.test", generated, reference, comparison)
}
