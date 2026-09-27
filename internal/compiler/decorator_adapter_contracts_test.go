package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecoratorAdapterSourceContracts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	source := `class Leaf{}
interface Service{function value():int;}
class Concrete implements Service{public function value():int{return 6;}}
alias Factory=(arguments:DecoratorValue[])=>Result<DecoratorValue>;
alias Invoker=(receiver:DecoratorValue,arguments:DecoratorValue[])=>Result<DecoratorValue>;
let factories:Factory[]=[];
let methods:Invoker[]=[];
let statics:Factory[]=[];
let calls:int=0;
function C(ctx:ClassDecoratorContext):void{factories=append(factories,ctx.construct);}
function M(ctx:MethodDecoratorContext):void{if(ctx.static){statics=append(statics,ctx.invokeStatic);}else{methods=append(methods,ctx.invoke);}}
@C class Consumer{
 constructor(public leaf:Leaf){calls+=1;}
 @M public function run(leaf:Leaf):Leaf{calls+=1;return leaf;}
 @M public static function many(...leaves:Leaf[]):int{calls+=1;return len(leaves);}
 @M public static function nullable():Leaf|null{return null;}
 @M public static function service():Service{return new Concrete();}
}
@C class Injection{constructor(public service:Service){}}
export function BadConstructor():Result<DecoratorValue>{const f=factories[0];return f([decoratorValue<Leaf|null>(null)]);}
export function BadMethod():Result<DecoratorValue>{const f=methods[0];return f(decoratorValue(new Consumer(new Leaf())),[decoratorValue<Leaf|null>(null)]);}
export function BadVariadic():Result<DecoratorValue>{const f=statics[0];return f([decoratorValue<Leaf|null>(null)]);}
export function BadReturn():Result<Leaf>{const f=statics[1];const value=f([])?;return decoratorValueAs<Leaf>(value);}
export function GoodReturn():Result<Leaf|null>{const f=statics[1];const value=f([])?;return decoratorValueAs<Leaf|null>(value);}
export function GoodDI():Result<int>{const f=factories[1];const value=f([decoratorValue<Service>(new Concrete())])?;const injected=decoratorValueAs<Injection>(value)?;return ok(injected.service.value());}
export function GoodInterfaceReturn():Result<int>{const f=statics[2];const value=f([])?;const service=decoratorValueAs<Service>(value)?;return ok(service.value());}
export function Count():int{return calls;}
`
	if err := os.WriteFile(entry, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "contracts")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func GoodDI()int{return 6}
func Count()int{return 1}
`
	comparison := `package contracts_test
import("testing";g "decorator-contracts.test";r "decorator-contracts.test/reference")
func TestContracts(t *testing.T){
 if _,err:=g.BadConstructor();err==nil{t.Error("constructor accepted nullable argument")}
 if _,err:=g.BadMethod();err==nil{t.Error("method accepted nullable argument")}
 if _,err:=g.BadVariadic();err==nil{t.Error("variadic method accepted nullable argument")}
 if _,err:=g.BadReturn();err==nil{t.Error("method erased nullable return")}
 if got,err:=g.GoodReturn();err!=nil||got!=nil{t.Errorf("nullable return=%v,%v",got,err)}
 if got,err:=g.GoodDI();err!=nil||got!=r.GoodDI(){t.Errorf("interface injection=%v,%v",got,err)}
 if got,err:=g.GoodInterfaceReturn();err!=nil||got!=r.GoodDI(){t.Errorf("interface return=%v,%v",got,err)}
 if got:=g.Count();got!=r.Count(){t.Errorf("invalid calls executed: %v",got)}
}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-contracts.test", generated, reference, comparison)
}
