package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestClassMethodExpressionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"classes.km": `import go errors from "errors";
export abstract class Base {
 constructor(public n:int){}
 public abstract function read():int;
 protected function add(v:int):int{return this.n+v;}
 public function pair():(int,bstring){return this.n,b"base";}
}
export class Child extends Base {
 constructor(n:int){super(n);}
 public override function read():int{return this.n*2;}
 public function promoted(v:int):int{return Child.add(this,v);}
 public static function make(n:int):Child{return new Child(n);}
}
export class Box<T>{
 constructor(public item:T){}
 public function read():T{return this.item;}
 public function count(...xs:T[]):int{return len(xs);}
 public function checked(success:boolean):Result<T>{if(success){return ok(this.item);}return fail(errors.New(b"failure"));}
}
export alias BoxAlias<T>=Box<T>;
export class Item{constructor(public n:int){}}
export class Effects{public static calls:int=0;public function mark():int{Effects.calls++;return 7;}}
`,
		"entry.km": `import {Base,Child as Derived,Box,BoxAlias,Item,Effects} from "./classes";
function apply<T>(cb:(v:T)=>int,v:T):int{return cb(v);}
export function Run(n:int):int[]{
 const child=Derived.make(n);
 const base:Base=child;
 const [value,label]=Derived.pair(child);
 const count=Box<int>.count;
 const box=new Box<int>(n);
 return [Base.read(base),apply(Derived.read,child),Derived.promoted(child,3),value,len(label),BoxAlias[int].read(box),count(box,n,n+1)];
}
export function Text(value:string):string{return Box<string>.read(new Box<string>(value));}
export function Nullable(flag:boolean):boolean{let item:Item|null=null;if(flag){item=new Item(5);}return Box<Item|null>.read(new Box<Item|null>(item))===null;}
export function Checked(n:int,success:boolean):Result<int>{return Box[int].checked(new Box<int>(n),success);}
export function Callback():(v:Effects)=>int{return Effects.mark;}
export function EffectsCount():int{return Effects.calls;}
export function Payload():(v:Box<Item>,success:boolean)=>Result<Item>{return Box<Item>.checked;}
class AliasTarget{constructor(public n:int){}public function read():int{return this.n;}}
alias AliasReader=AliasTarget;
export function Aliased():int{const box=new AliasTarget(4);{const AliasTarget=3;return AliasReader.read(box)+AliasTarget;}}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "classes")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "classes")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}

	reference := `package reference
import "errors"
type reader interface{Read() int}
type child struct{n int}
func(c *child)Read()int{return c.n*2}
func(c *child)Add(v int)int{return c.n+v}
func(c *child)Pair()(int,string){return c.n,"base"}
type box[T any]struct{item T}
func(b *box[T])Read()T{return b.item}
func(b *box[T])Count(xs ...T)int{return len(xs)}
func(b *box[T])Checked(success bool)(T,error){if success{return b.item,nil};var zero T;return zero,errors.New("failure")}
func Run(n int)[]int{c:=&child{n};value,label:=(*child).Pair(c);b:=&box[int]{n};return []int{reader.Read(c),(*child).Read(c),(*child).Add(c,3),value,len(label),(*box[int]).Read(b),(*box[int]).Count(b,n,n+1)}}
func Text(value string)string{return (*box[string]).Read(&box[string]{value})}
func Nullable(flag bool)bool{var n *int;if flag{v:=5;n=&v};return (*box[*int]).Read(&box[*int]{n})==nil}
func Checked(n int,success bool)(int,error){return (*box[int]).Checked(&box[int]{n},success)}
`
	comparison := `package classes_test
import("reflect";"strings";"fmt";"testing";g "class-method-expressions.test";r "class-method-expressions.test/reference")
func capture(f func())(result any){defer func(){result=recover()}();f();return nil}
func TestBehavior(t *testing.T){
 for _,n:=range []int{-5,0,17}{if got,want:=g.Run(n),r.Run(n);!reflect.DeepEqual(got,want){t.Fatal(n,got,want)};for _,b:=range []bool{false,true}{a,ae:=g.Checked(n,b);v,ve:=r.Checked(n,b);if a!=v||fmt.Sprint(ae)!=fmt.Sprint(ve)||g.Nullable(b)!=r.Nullable(b){t.Fatal(n,b,a,ae,v,ve)}}}
 for _,s:=range []string{"","金木犀🍊"}{if g.Text(s)!=r.Text(s){t.Fatal(s)}}
 if g.Aliased()!=7{t.Fatal("concrete alias under lexical shadow")}
}
func TestGoCaller(t *testing.T){
 f:=g.Callback()
 p:=capture(func(){f(nil)})
 if p==nil||!strings.Contains(fmt.Sprint(p),"entry.km:")||g.EffectsCount()!=0{t.Fatal(p,g.EffectsCount())}
 if f(g.NewEffects())!=7||g.EffectsCount()!=1{t.Fatal("call dispatch")}
 read:=g.Payload()
 if value,err:=read(&g.Box[*g.Item]{},false);value!=nil||err==nil{t.Fatal(value,err)}
 if p:=capture(func(){read(&g.Box[*g.Item]{},true)});p==nil||!strings.Contains(fmt.Sprint(p),"entry.km:"){t.Fatal("missing non-null result guard",p)}
}
`
	runGeneratedGoDifferentialTest(t, root, "class-method-expressions.test", generated, reference, comparison)
}
