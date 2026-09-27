package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecoratorAccessorAdaptersMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	files := map[string]string{
		"registry.km": `alias Invoke=(receiver:DecoratorValue,args:DecoratorValue[])=>Result<DecoratorValue>;
alias StaticInvoke=(args:DecoratorValue[])=>Result<DecoratorValue>;
let instances:Invoke[]=[];
let statics:StaticInvoke[]=[];
function register(invoke:Invoke,invokeStatic:StaticInvoke):void{instances=append(instances,invoke);statics=append(statics,invokeStatic);}
export function Read(ctx:GetterDecoratorContext):void{register(ctx.invoke,ctx.invokeStatic);}
export function Write(ctx:SetterDecoratorContext):void{register(ctx.invoke,ctx.invokeStatic);}
export function Method(ctx:MethodDecoratorContext):void{register(ctx.invoke,ctx.invokeStatic);}
export function Call(index:int,receiver:DecoratorValue,args:DecoratorValue[]):Result<DecoratorValue>{const invoke=instances[index];return invoke(receiver,args);}
export function CallStatic(index:int,args:DecoratorValue[]):Result<DecoratorValue>{const invoke=statics[index];return invoke(args);}
`,
		"entry.km": `import {Read,Write,Method,Call,CallStatic} from "./registry";
import go strconv from "strconv";
class Base{
 protected stored:string="";
 @Read public virtual get title():string{return this.stored;}
 @Write public virtual set title(value:string){this.stored=value;}
}
class Child extends Base{
 @Read public override get title():string{return super.title+"!";}
 @Write public override set title(value:string){super.title=value+"?";}
}
class Settings{
 private static stored:int8=1;
 @Read public static get count():int8{return Settings.stored;}
 @Write public static set count(value:int8){Settings.stored=value;}
}
class Shared<T>{
 private static stored:int=0;
 @Read public static get count():int{return Shared.stored;}
 @Write public static set count(value:int){Shared.stored=value;}
}
class Guarded{
 private stored:int=3;
 @Read public get value():int{return this.stored;}
 @Write private set value(next:int){this.stored=next;}
}
class Pair{@Method public static function split():(int,string){return 1,"pair";}}
export function TypedPair():string{const [count,text]=Pair.split();return strconv.Itoa(count)+text;}
export function MultipleResults():Result<string>{const boxed=CallStatic(10,[])?;const values=decoratorValueAs<DecoratorValue[]>(boxed)?;const count=decoratorValueAs<int>(values[0])?;const text=decoratorValueAs<string>(values[1])?;return ok(strconv.Itoa(count)+text);}
export function Run():Result<string>{
 const child=new Child();
 const base=decoratorValue<Base>(child);
 const derived=decoratorValue(child);
 const ignored=Call(1,base,[decoratorValue("first")])?;
 const firstValue=Call(0,base,[])?;
 const first=decoratorValueAs<string>(firstValue)?;
 const written=Call(3,derived,[decoratorValue("second")])?;
 const secondValue=Call(2,derived,[])?;
 const second=decoratorValueAs<string>(secondValue)?;
 const changed=CallStatic(5,[decoratorValue<int8>(7)])?;
 const countValue=CallStatic(4,[])?;
 const count=decoratorValueAs<int8>(countValue)?;
 const sharedWrite=CallStatic(7,[decoratorValue(9)])?;
 const sharedValue=CallStatic(6,[])?;
 const shared=decoratorValueAs<int>(sharedValue)?;
 return ok(first+";"+second+";"+written.typeIdentity+";"+strconv.Itoa(int(count))+";"+strconv.Itoa(shared));
}
export function WrongReceiver():Result<DecoratorValue>{return Call(0,decoratorValue("wrong"),[]);}
export function WrongGetterArity():Result<DecoratorValue>{return Call(0,decoratorValue(new Base()),[decoratorValue(1)]);}
export function WrongSetterType():Result<DecoratorValue>{return CallStatic(5,[decoratorValue(100)]);}
export function WrongSetterArity():Result<DecoratorValue>{return CallStatic(5,[]);}
export function WrongAdapter():Result<DecoratorValue>{return Call(4,decoratorValue(1),[]);}
export function NullReceiver():Result<DecoratorValue>{return Call(0,decoratorValue<Base|null>(null),[]);}
export function PrivateSetter():Result<DecoratorValue>{return Call(9,decoratorValue(new Guarded()),[decoratorValue(4)]);}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "accessors")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "strconv"
type base struct{stored string}
func(b *base)get()string{return b.stored}
func(b *base)set(s string){b.stored=s}
type child struct{base}
func(c *child)get()string{return c.base.get()+"!"}
func(c *child)set(s string){c.base.set(s+"?")}
type property interface{get()string;set(string)}
func split()(int,string){return 1,"pair"}
func TypedPair()string{count,text:=split();return strconv.Itoa(count)+text}
func Run()string{c:=&child{};var p property=c;p.set("first");first:=p.get();c.set("second");second:=c.get();var count int8=7;shared:=9;return first+";"+second+";void;"+strconv.Itoa(int(count))+";"+strconv.Itoa(shared)}
func Errors()[]string{return []string{
 "decorator get Base.title expects a non-null Base receiver",
 "decorator get Base.title expects 0 arguments",
 "decorator set Settings.count argument 0 expects int8",
 "decorator set Settings.count expects 1 argument",
 "static method requires invokeStatic",
 "decorator get Base.title expects a non-null Base receiver",
 "method is not public",
}}
`
	comparison := `package accessors_test
import("testing";g "decorator-accessors.test";r "decorator-accessors.test/reference")
func TestAccessors(t *testing.T){
 if got,err:=g.Run();err!=nil||got!=r.Run(){t.Fatalf("Run=%q,%v want %q",got,err,r.Run())}
 if got:=g.TypedPair();got!=r.TypedPair(){t.Fatalf("TypedPair=%q want %q",got,r.TypedPair())}
 if got,err:=g.MultipleResults();err!=nil||got!=r.TypedPair(){t.Errorf("MultipleResults=%q,%v",got,err)}
 _,a:=g.WrongReceiver();_,b:=g.WrongGetterArity();_,c:=g.WrongSetterType();_,d:=g.WrongSetterArity();_,e:=g.WrongAdapter();_,f:=g.NullReceiver();_,h:=g.PrivateSetter();
 for i,err:=range []error{a,b,c,d,e,f,h}{if want:=r.Errors()[i];err==nil||err.Error()!=want{t.Errorf("error %d=%v want %s",i,err,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-accessors.test", generated, reference, comparison)
}

func TestDecoratorCallableAvailabilityBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, declaration, reason string
		instance, static          bool
	}{
		{"getter", `class C{@D public get value():int{return 1;}}`, "", true, false},
		{"setter", `class C{@D public set value(next:int){}}`, "", true, false},
		{"private getter", `class C{@D private get value():int{return 1;}}`, "method is not public", false, false},
		{"protected setter", `class C{@D protected set value(next:int){}}`, "method is not public", false, false},
		{"abstract getter", `abstract class C{@D public abstract get value():int;}`, "abstract methods cannot be invoked", false, false},
		{"abstract setter", `abstract class C{@D public abstract set value(next:int);}`, "abstract methods cannot be invoked", false, false},
		{"generic receiver", `class C<T>{@D public get value():int{return 1;}}`, "generic methods require concrete type arguments", false, false},
		{"generic static getter", `class C<T>{@D public static get value():int{return 1;}}`, "", false, true},
		{"generic static setter", `class C<T>{@D public static set value(next:int){}}`, "", false, true},
		{"multiple results", `class C{@D public function pair():(int,string){return 1,"x";}}`, "", true, false},
		{"static multiple results", `class C{@D public static function pair():(int,string){return 1,"x";}}`, "", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry := filepath.Join(t.TempDir(), "entry.km")
			source := "function D(ctx:DecoratorContext):void{}\n" + test.declaration
			if err := os.WriteFile(entry, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			checked, err := CheckFiles([]string{entry})
			if err != nil || len(checked.Diagnostics) != 0 {
				t.Fatalf("err=%v diagnostics=%v", err, checked.Diagnostics)
			}
			target := checked.Program.Decorators[0].Target
			if target.Invocable != test.instance || target.StaticInvocable != test.static {
				t.Fatalf("availability=%t/%t want %t/%t", target.Invocable, target.StaticInvocable, test.instance, test.static)
			}
			if test.reason != "" && (target.InvokeUnavailableReason != test.reason || target.StaticInvokeUnavailableReason != test.reason) {
				t.Errorf("reasons=%q/%q want %q", target.InvokeUnavailableReason, target.StaticInvokeUnavailableReason, test.reason)
			}
			if _, diagnostics, err := EmitGo([]string{entry}, "availability"); err != nil || len(diagnostics) != 0 {
				t.Fatalf("unavailable adapters must still emit valid Go: err=%v diagnostics=%v", err, diagnostics)
			}
		})
	}
}
