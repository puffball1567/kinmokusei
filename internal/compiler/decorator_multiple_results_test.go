package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecoratorMultipleResultsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	files := map[string]string{
		"registry.km": `alias Invoke=(receiver:DecoratorValue,args:DecoratorValue[])=>Result<DecoratorValue>;
alias StaticInvoke=(args:DecoratorValue[])=>Result<DecoratorValue>;
let instances:Invoke[]=[];
let statics:StaticInvoke[]=[];
export function Capture(ctx:MethodDecoratorContext):void{instances=append(instances,ctx.invoke);statics=append(statics,ctx.invokeStatic);}
export function Call(index:int,receiver:DecoratorValue,args:DecoratorValue[]):Result<DecoratorValue>{const invoke=instances[index];return invoke(receiver,args);}
export function CallStatic(index:int,args:DecoratorValue[]):Result<DecoratorValue>{const invoke=statics[index];return invoke(args);}
`,
		"entry.km": `import {Capture,Call,CallStatic} from "./registry";
import go errors from "errors";
import go strconv from "strconv";
class Leaf{}
interface Reader{function read():string;}
class Text implements Reader{public function read():string{return "read";}}
class Box<T>{constructor(public value:T){}}
let calls:int=0;
class Service{
 @Capture public function values(value:int8):(int8,string,Leaf|null,Reader,Box<int>,float32){calls+=1;return value,"text",null,new Text(),new Box<int>(9),1.25;}
 @Capture public static function parse(failed:boolean):(int,error){calls+=1;if(failed){return 0,errors.New("tuple failure");}return 42,nil;}
 @Capture public static function checked(failed:boolean):Result<int>{if(failed){return fail(errors.New("Result failure"));}return ok(42);}
 @Capture public static function summary(...values:int[]):(int,int){calls+=1;let total=0;for(const value of values){total+=value;}return len(values),total;}
}
class Base{@Capture public virtual function pair():(string,int8){return "base",3;}}
class Child extends Base{@Capture public override function pair():(string,int8){return "child",4;}}
alias MaybeLeaf=Leaf|null;
class Nested{@Capture public static function pair():(MaybeLeaf[],()=>Leaf|null){const callback:()=>Leaf|null=()=>null;return [null],callback;}}
function Slots(value:DecoratorValue):Result<DecoratorValue[]>{return decoratorValueAs<DecoratorValue[]>(value);}
export function Run():Result<string>{
 calls=0;
 const boxed=Call(0,decoratorValue(new Service()),[decoratorValue<int8>(7)])?;
 const slots=Slots(boxed)?;
 const narrow=decoratorValueAs<int8>(slots[0])?;
 const text=decoratorValueAs<string>(slots[1])?;
 const leaf=decoratorValueAs<Leaf|null>(slots[2])?;
 const reader=decoratorValueAs<Reader>(slots[3])?;
 const generic=decoratorValueAs<Box<int>>(slots[4])?;
 const fraction=decoratorValueAs<float32>(slots[5])?;
 if(leaf!==null){return fail(errors.New("nullable slot did not retain null"));}
 return ok(strconv.Itoa(int(narrow))+";"+text+";"+reader.read()+";"+strconv.Itoa(generic.value)+";"+strconv.FormatFloat(float(fraction),102,2,32)+";"+strconv.Itoa(calls)+";"+boxed.typeIdentity);
}
export function Parse(failed:boolean):Result<string>{
 const boxed=CallStatic(1,[decoratorValue(failed)])?;
 const slots=Slots(boxed)?;
 const value=decoratorValueAs<int>(slots[0])?;
 const err=decoratorValueAs<error>(slots[1])?;
 if(err!==nil){return ok(err.Error());}
 return ok(strconv.Itoa(value));
}
export function Checked():Result<DecoratorValue>{return CallStatic(2,[decoratorValue(true)]);}
export function Summary():Result<int>{const boxed=CallStatic(3,[decoratorValue(2),decoratorValue(5)])?;const slots=Slots(boxed)?;const count=decoratorValueAs<int>(slots[0])?;const sum=decoratorValueAs<int>(slots[1])?;return ok(count*10+sum);}
export function EmptySummary():Result<int>{const boxed=CallStatic(3,[])?;const slots=Slots(boxed)?;const count=decoratorValueAs<int>(slots[0])?;const sum=decoratorValueAs<int>(slots[1])?;return ok(count*10+sum);}
export function Virtual():Result<string>{const child=new Child();const base=Call(4,decoratorValue<Base>(child),[])?;const derived=Call(5,decoratorValue(child),[])?;const a=Slots(base)?;const b=Slots(derived)?;const text=decoratorValueAs<string>(a[0])?;const code=decoratorValueAs<int8>(b[1])?;return ok(text+strconv.Itoa(int(code)));}
export function Direct():string{const [value,err]=Service.parse(false);if(err!==nil){return err.Error();}return strconv.Itoa(value);}
export function BadNullable():Result<Leaf>{const boxed=Call(0,decoratorValue(new Service()),[decoratorValue<int8>(1)])?;const slots=Slots(boxed)?;return decoratorValueAs<Leaf>(slots[2]);}
export function BadWidth():Result<int>{const boxed=Call(0,decoratorValue(new Service()),[decoratorValue<int8>(1)])?;const slots=Slots(boxed)?;return decoratorValueAs<int>(slots[0]);}
export function WrongArgument():Result<DecoratorValue>{return Call(0,decoratorValue(new Service()),[decoratorValue(1)]);}
export function NestedValues():Result<boolean>{const boxed=CallStatic(6,[])?;const slots=Slots(boxed)?;const values=decoratorValueAs<MaybeLeaf[]>(slots[0])?;const callback=decoratorValueAs<()=>Leaf|null>(slots[1])?;return ok(len(values)==1&&values[0]===null&&callback()===null);}
export function BadNested():Result<Leaf[]>{const boxed=CallStatic(6,[])?;const slots=Slots(boxed)?;return decoratorValueAs<Leaf[]>(slots[0]);}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "multipleresults")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("errors";"fmt";"strconv")
type reader interface{read()string}
type text struct{}
func(text)read()string{return "read"}
type box[T any]struct{value T}
func values(value int8)(int8,string,*struct{},reader,box[int],float32){return value,"text",nil,text{},box[int]{9},1.25}
func Run()string{calls:=0;v,s,_,r,b,f:=values(7);calls++;return fmt.Sprintf("%d;%s;%s;%d;%.2f;%d;DecoratorValue[]",v,s,r.read(),b.value,f,calls)}
func parse(failed bool)(int,error){if failed{return 0,errors.New("tuple failure")};return 42,nil}
func Parse(failed bool)string{v,err:=parse(failed);if err!=nil{return err.Error()};return strconv.Itoa(v)}
func Checked()error{return errors.New("Result failure")}
func summary(values ...int)(int,int){total:=0;for _,v:=range values{total+=v};return len(values),total}
func Summary()int{n,sum:=summary(2,5);return n*10+sum}
func EmptySummary()int{n,sum:=summary();return n*10+sum}
func NestedValues()bool{values:=[]*struct{}{nil};callback:=func()*struct{}{return nil};return len(values)==1&&values[0]==nil&&callback()==nil}
type base struct{}
func(base)pair()(string,int8){return "base",3}
type child struct{base}
func(child)pair()(string,int8){return "child",4}
type virtual interface{pair()(string,int8)}
func Virtual()string{c:=child{};var b virtual=c;s,_:=b.pair();_,n:=c.pair();return s+strconv.Itoa(int(n))}
`
	comparison := `package multipleresults_test
import("testing";g "decorator-multiple-results.test";r "decorator-multiple-results.test/reference")
func TestMultipleResults(t *testing.T){
 if got,err:=g.Run();err!=nil||got!=r.Run(){t.Fatalf("Run=%q,%v want %q",got,err,r.Run())}
 for _,failed:=range []bool{false,true}{if got,err:=g.Parse(failed);err!=nil||got!=r.Parse(failed){t.Errorf("Parse(%v)=%q,%v want %q",failed,got,err,r.Parse(failed))}}
 if got,err:=g.Summary();err!=nil||got!=r.Summary(){t.Errorf("Summary=%v,%v",got,err)}
 if got,err:=g.EmptySummary();err!=nil||got!=r.EmptySummary(){t.Errorf("EmptySummary=%v,%v",got,err)}
 if got,err:=g.NestedValues();err!=nil||got!=r.NestedValues(){t.Errorf("NestedValues=%v,%v",got,err)}
 if got,err:=g.Virtual();err!=nil||got!=r.Virtual(){t.Errorf("Virtual=%q,%v",got,err)}
 if got:=g.Direct();got!=r.Parse(false){t.Errorf("Direct=%q",got)}
 if _,err:=g.Checked();err==nil||err.Error()!=r.Checked().Error(){t.Errorf("Result error=%v",err)}
 if _,err:=g.BadNullable();err==nil{t.Error("nullable slot lost its contract")}
 if _,err:=g.BadWidth();err==nil{t.Error("narrow slot lost its width")}
 if _,err:=g.WrongArgument();err==nil{t.Error("wrong input type accepted")}
 if _,err:=g.BadNested();err==nil{t.Error("nested nullable contract was erased")}
}
`
	runGeneratedGoDifferentialTest(t, root, "decorator-multiple-results.test", generated, reference, comparison)
}
