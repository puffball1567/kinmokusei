package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecoratorMultilevelDispatchMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, source := range map[string]string{
		"registry.km": `alias Invoke=(receiver:DecoratorValue,args:DecoratorValue[])=>Result<DecoratorValue>;
let adapters:Invoke[]=[];
function register(invoke:Invoke):void{adapters=append(adapters,invoke);}
export function Method(ctx:MethodDecoratorContext):void{register(ctx.invoke);}
export function Get(ctx:GetterDecoratorContext):void{register(ctx.invoke);}
export function Set(ctx:SetterDecoratorContext):void{register(ctx.invoke);}
export function Call(index:int,receiver:DecoratorValue,args:DecoratorValue[]):Result<DecoratorValue>{const invoke=adapters[index];return invoke(receiver,args);}
`,
		"model.km": `import {Method,Get,Set} from "./registry";
import go errors from "errors";
export class Base{
 public stored:int=0;
 public calls:int=0;
 @Method public virtual function load(value:int):Result<int>{this.calls++;return ok(value+1);}
 @Method public virtual function pair(...values:int[]):(int8,int){this.calls++;return 1,len(values);}
 @Method public virtual function touch():void{this.calls++;}
 @Get public virtual get value():int{return this.stored+1;}
 @Set public virtual set value(next:int){this.stored=next+1;}
}
export class Middle extends Base{
 @Method public override function load(value:int):Result<int>{this.calls++;return ok(value+10);}
 @Method public override function pair(...values:int[]):(int8,int){this.calls++;return 10,len(values);}
 @Method public override function touch():void{this.calls+=10;}
 @Get public override get value():int{return this.stored+10;}
 @Set public override set value(next:int){this.stored=next+10;}
}
export final class Leaf extends Middle{
 public final override function load(value:int):Result<int>{this.calls++;if(value<0){return fail(errors.New("leaf failure"));}return ok(value+100);}
 public final override function pair(...values:int[]):(int8,int){this.calls++;let sum=0;for(const value of values){sum+=value;}return 100,sum;}
 public final override function touch():void{this.calls+=100;}
 public final override get value():int{return this.stored+100;}
 public final override set value(next:int){this.stored=next+100;}
}
`,
		"entry.km": `import {Call} from "./registry";
import {Base,Middle,Leaf} from "./model";
export {Base,Middle,Leaf} from "./model";
export function Run(middle:boolean):Result<int[]>{
 const leaf=new Leaf();
 let offset=0;
 let receiver=decoratorValue<Base>(leaf);
 if(middle){offset=5;receiver=decoratorValue<Middle>(leaf);}
 const written=Call(offset+4,receiver,[decoratorValue(3)])?;
 const property=Call(offset+3,receiver,[])?;
 const loaded=Call(offset,receiver,[decoratorValue(7)])?;
 const paired=Call(offset+1,receiver,[decoratorValue(2),decoratorValue(5)])?;
 const touched=Call(offset+2,receiver,[])?;
 const values=decoratorValueAs<DecoratorValue[]>(paired)?;
 const narrow=decoratorValueAs<int8>(values[0])?;
 const sum=decoratorValueAs<int>(values[1])?;
 const value=decoratorValueAs<int>(property)?;
 const load=decoratorValueAs<int>(loaded)?;
 return ok([value,load,int(narrow),sum,leaf.calls]);
}
export function Fails():Result<DecoratorValue>{return Call(5,decoratorValue<Middle>(new Leaf()),[decoratorValue(-1)]);}
export function InvokeBase(value:Base):Result<DecoratorValue>{return Call(0,decoratorValue(value),[decoratorValue(1)]);}
export function InvokeMiddle(value:Middle):Result<DecoratorValue>{return Call(5,decoratorValue(value),[decoratorValue(1)]);}
export function WrongArgument(value:Middle):Result<DecoratorValue>{return Call(5,decoratorValue(value),[decoratorValue("wrong")]);}
export function Direct():Result<int[]>{const leaf=new Leaf();const value:Middle=leaf;value.value=3;const property=value.value;const loaded=value.load(7)?;const [a,b]=value.pair(2,5);value.touch();return ok([property,loaded,int(a),b,leaf.calls]);}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "virtualadapters")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "errors"
type service interface{load(int)(int,error);pair(...int)(int8,int);touch();get()int;set(int)}
type leaf struct{stored,calls int}
func(l *leaf)load(v int)(int,error){l.calls++;if v<0{return 0,errors.New("leaf failure")};return v+100,nil}
func(l *leaf)pair(values ...int)(int8,int){l.calls++;sum:=0;for _,v:=range values{sum+=v};return 100,sum}
func(l *leaf)touch(){l.calls+=100}
func(l *leaf)get()int{return l.stored+100}
func(l *leaf)set(v int){l.stored=v+100}
func Run()[]int{l:=&leaf{};var s service=l;s.set(3);v:=s.get();n,_:=s.load(7);a,b:=s.pair(2,5);s.touch();return []int{v,n,int(a),b,l.calls}}
func Fails()error{var s service=&leaf{};_,err:=s.load(-1);return err}
`
	comparison := `package virtualadapters_test
import("testing";"reflect";"strings";g "decorator-virtual-dispatch.test";r "decorator-virtual-dispatch.test/reference")
func TestDispatch(t *testing.T){for _,middle:=range []bool{false,true}{got,err:=g.Run(middle);if err!=nil||!reflect.DeepEqual(got,r.Run()){t.Errorf("middle=%v got=%v,%v want=%v",middle,got,err,r.Run())}};if got,err:=g.Direct();err!=nil||!reflect.DeepEqual(got,r.Run()){t.Errorf("direct=%v,%v",got,err)}}
func TestFailure(t *testing.T){if _,err:=g.Fails();err==nil||err.Error()!=r.Fails().Error(){t.Errorf("failure=%v",err)}}
func TestUninitializedReceiver(t *testing.T){
 base:=new(g.Base);middle:=new(g.Middle);
 _,a:=g.InvokeBase(base);_,b:=g.InvokeMiddle(middle);
 for _,err:=range []error{a,b}{if err==nil||!strings.Contains(err.Error(),"initialized virtual receiver"){t.Errorf("uninitialized receiver=%v",err)}}
 if base.Calls!=0||middle.Calls!=0{t.Error("uninitialized receiver was invoked")}
 _,a=g.InvokeBase(nil);_,b=g.InvokeMiddle(nil);
 for _,err:=range []error{a,b}{if err==nil||!strings.Contains(err.Error(),"non-null"){t.Errorf("nil receiver=%v",err)}}
}
func TestArgumentCheckBeforeInvocation(t *testing.T){middle:=g.UpcastLeafToMiddle(g.NewLeaf());if _,err:=g.WrongArgument(middle);err==nil{t.Error("wrong argument accepted")};if middle.Calls!=0{t.Error("method called before argument validation")}}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-virtual-dispatch.test", generated, reference, comparison)
}
