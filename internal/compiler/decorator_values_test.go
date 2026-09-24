package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecoratorValueStorageAndContracts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	source := `class Leaf{}
interface Service { function value():int; }
class Implementation implements Service { public function value():int{return 7;} }
export function Narrow():Result<int8>{return decoratorValueAs<int8>(decoratorValue<int8>(12));}
export function Fraction():Result<float32>{return decoratorValueAs<float32>(decoratorValue<float32>(1.5));}
export function NullRoundTrip():Result<Leaf|null>{return decoratorValueAs<Leaf|null>(decoratorValue<Leaf|null>(null));}
export function InterfaceRoundTrip():Result<int>{const value=decoratorValueAs<Service>(decoratorValue<Service>(new Implementation()))?;return ok(value.value());}
export function NullInterface():Result<Service|null>{return decoratorValueAs<Service|null>(decoratorValue<Service|null>(null));}
type Tiny=distinct int8;
export function Named():Result<Tiny>{return decoratorValueAs<Tiny>(decoratorValue<Tiny>(4));}
class Box<T>{constructor(public value:T){}}
export function Generic():Result<int>{const value=decoratorValueAs<Box<int>>(decoratorValue(new Box<int>(9)))?;return ok(value.value);}
export function BadNullable():Result<Leaf>{const leaf:Leaf|null=null;return decoratorValueAs<Leaf>(decoratorValue(leaf));}
alias MaybeLeaf=Leaf|null;
export function BadNested():Result<Leaf[]>{const leaves:MaybeLeaf[]=[null];return decoratorValueAs<Leaf[]>(decoratorValue(leaves));}
export function BadCallback():Result<()=>Leaf>{const callback:()=>Leaf|null=()=>null;return decoratorValueAs<()=>Leaf>(decoratorValue(callback));}
let contexts:DecoratorValue[]=[];
function Capture(context:ClassDecoratorContext):void{contexts=append(contexts,decoratorValue(context));}
@Capture class Decorated{}
export function BadContext():Result<MethodDecoratorContext>{return decoratorValueAs<MethodDecoratorContext>(contexts[0]);}
let calls:int=0;
function Once():DecoratorValue{calls+=1;return decoratorValue<int8>(3);}
export function SingleEvaluation():Result<int>{const value=decoratorValueAs<int8>(Once())?;return ok(calls);}
`
	if err := os.WriteFile(entry, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "values")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Narrow()int8{return 12}
func Fraction()float32{return 1.5}
func InterfaceRoundTrip()int{return 7}
func SingleEvaluation()int{return 1}
`
	comparison := `package values_test
import("testing";g "decorator-values.test";r "decorator-values.test/reference")
func TestStorage(t *testing.T){
 if got,err:=g.Narrow();err!=nil||got!=r.Narrow(){t.Errorf("narrow=%v,%v",got,err)}
 if got,err:=g.Fraction();err!=nil||got!=r.Fraction(){t.Errorf("fraction=%v,%v",got,err)}
 if got,err:=g.NullRoundTrip();err!=nil||got!=nil{t.Errorf("nullable=%v,%v",got,err)}
 if got,err:=g.InterfaceRoundTrip();err!=nil||got!=r.InterfaceRoundTrip(){t.Errorf("interface=%v,%v",got,err)}
 if got,err:=g.NullInterface();err!=nil||got!=nil{t.Errorf("nil interface=%v,%v",got,err)}
 if got,err:=g.Named();err!=nil||got!=4{t.Errorf("named=%v,%v",got,err)}
 if got,err:=g.Generic();err!=nil||got!=9{t.Errorf("generic=%v,%v",got,err)}
 if got,err:=g.SingleEvaluation();err!=nil||got!=r.SingleEvaluation(){t.Errorf("evaluation=%v,%v",got,err)}
 if _,err:=g.BadNullable();err==nil{t.Error("nullable contract bypass")}
 if _,err:=g.BadNested();err==nil{t.Error("nested nullable contract bypass")}
 if _,err:=g.BadCallback();err==nil{t.Error("callback nullable contract bypass")}
 if _,err:=g.BadContext();err==nil{t.Error("nominal context contract bypass")}
}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-values.test", generated, reference, comparison)
}
