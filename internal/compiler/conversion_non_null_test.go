package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNonNullConversionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":        "module conversion-non-null.test\n\ngo 1.23\n\nrequire example.com/native v0.0.0\nreplace example.com/native => ./native\n",
		"native/go.mod": "module example.com/native\n\ngo 1.23\n",
		"native/native.go": `package native
type Raw func()int
func Get(p bool)Raw{if p{return func()int{return 7}};return nil}
type hiddenFunc func()int
func Hidden(p bool)hiddenFunc{if p{return func()int{return 8}};return nil}
func Identity(p bool)func(int)int{if p{return func(n int)int{return n+2}};return nil}
`,
		"models.km": "export alias Callback=()=>int;\nexport type Native=distinct ()=>int;\nexport alias Identity<T>=(value:T)=>T;\nexport alias View=interface{Error():bstring;};\n",
		"entry.km": `import go native from "example.com/native";
import {Callback,Native,Identity,View} from "./models";
export function Convert(present:boolean):Callback{return Callback(native.Get(present));}
export function Named(present:boolean):Native{return Native(native.Get(present));}
export function Hidden(present:boolean):Callback{return Callback(native.Hidden(present));}
export function GenericAlias(present:boolean):Identity<int>{return Identity<int>(native.Identity(present));}
constraint Callbacks=()=>int;
export function Bound<T extends Callbacks>(value:Callback):T{return T(value);}
export function RawNil(present:boolean):boolean{return native.Raw(native.Get(present))===nil;}
export function Interface(value:error):View{return View(value);}
export function Snapshot(present:boolean):int{const value:Callback|null=Callback(native.Get(present));return value();}
export function AssertSnapshot(value:interface{}):int{const callback:Callback|null=value as! Callback;return callback();}
export function NullableRaw(present:boolean):boolean{const value:Callback|null=native.Get(present);return value===null;}
export function Ordered(present:boolean,visit:(step:int)=>void):Callback{const value=():native.Raw=>{visit(1);return native.Get(present);};return Callback(value());}
export function Finally(present:boolean,visit:()=>void):Callback{try{return Callback(native.Get(present));}finally{visit();}}
alias Once=[1]Callback;
export function FixedLen(present:boolean):int{const raw=native.Get(present);const n=len(copyArray[Once]([Callback(raw)]));const address=&n;return *address;}
export function FixedCap(present:boolean):int{const raw=native.Get(present);return cap(copyArray[Once]([Callback(raw)]));}
function accept(n:int,callback:Callback):void{}
export function PanicOrder():void{const raw:native.Raw=nil;const values:int[]=[];accept(values[0],Callback(raw));}
export function Discard(present:boolean):void{const raw=native.Get(present);const unused=Callback(raw);}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "conversionnonnull")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{filepath.Join(root, "entry.km")}, "conversionnonnull")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	if bytes.Contains(generated, []byte("native.hiddenFunc")) {
		t.Fatalf("named private Go function type:\n%s", generated)
	}
	reference := `package reference
func Convert(p bool)func()int{var value func()int;if p{value=func()int{return 7}};if value==nil{panic("nil function conversion")};return value}
type Native func()int
func Named(p bool)Native{return Native(Convert(p))}
func Hidden(p bool)func()int{var value func()int;if p{value=func()int{return 8}};if value==nil{panic("nil conversion")};return value}
func GenericAlias(p bool)func(int)int{var value func(int)int;if p{value=func(n int)int{return n+2}};if value==nil{panic("nil conversion")};return value}
func Bound[T ~func()int](value func()int)T{converted:=T(value);if converted==nil{panic("nil conversion")};return converted}
func RawNil(p bool)bool{var value func()int;if p{value=func()int{return 7}};return value==nil}
func Interface(value error)interface{}{if value==nil{panic("nil interface conversion")};return value}
func Snapshot(p bool)int{return Convert(p)()}
func AssertSnapshot(input interface{})int{value:=input.(func()int);if value==nil{panic("nil assertion")};return value()}
func NullableRaw(p bool)bool{return !p}
func Ordered(p bool,visit func(int))func()int{load:=func()func()int{visit(1);if p{return func()int{return 7}};return nil};v:=load();if v==nil{panic("nil conversion")};return v}
func Finally(p bool,visit func())func()int{defer visit();return Convert(p)}
func FixedLen(p bool)int{return len([1]func()int{Convert(p)})}
func FixedCap(p bool)int{return cap([1]func()int{Convert(p)})}
func PanicOrder(){values:=[]int{};first:=values[0];_ =first;var callback func()int;if callback==nil{panic("nil conversion")}}
func Discard(p bool){_ =Convert(p)}
`
	comparison := `package conversionnonnull_test
import("errors";"fmt";"reflect";"strings";"testing";g "conversion-non-null.test";r "conversion-non-null.test/reference")
func capture(f func())(failure any){defer func(){failure=recover()}();f();return nil}
type failure struct{}
func(*failure)Error()string{return "failure"}
func TestConversion(t *testing.T){for _,p:=range []bool{false,true}{
 gp:=capture(func(){g.Convert(p)});rp:=capture(func(){r.Convert(p)});if(gp==nil)!=(rp==nil){t.Fatal(p,gp,rp)}
 if !p&&fmt.Sprint(gp)!="non-null conversion failed at entry.km:3:58"{t.Fatal(gp)}
 if gp==nil{if got,want:=g.Convert(p)(),r.Convert(p)();got!=want{t.Fatal(got,want)}}
 for _,pair:=range [][2]func(bool)int{{g.Snapshot,r.Snapshot},{g.FixedLen,r.FixedLen},{g.FixedCap,r.FixedCap}}{var got,want int;gp=capture(func(){got=pair[0](p)});rp=capture(func(){want=pair[1](p)});if(gp==nil)!=(rp==nil)||gp==nil&&got!=want{t.Fatal(got,want,gp,rp)}}
 gp=capture(func(){g.Named(p)});rp=capture(func(){r.Named(p)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)};if p{if got,want:=g.Named(p)(),r.Named(p)();got!=want{t.Fatal(got,want)}}
 gp=capture(func(){g.Hidden(p)});rp=capture(func(){r.Hidden(p)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)};if p{if got,want:=g.Hidden(p)(),r.Hidden(p)();got!=want{t.Fatal(got,want)}}
 gp=capture(func(){g.GenericAlias(p)});rp=capture(func(){r.GenericAlias(p)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)};if p{if got,want:=g.GenericAlias(p)(4),r.GenericAlias(p)(4);got!=want{t.Fatal(got,want)}}
 if got,want:=g.RawNil(p),r.RawNil(p);got!=want{t.Fatal(got,want)};if got,want:=g.NullableRaw(p),r.NullableRaw(p);got!=want{t.Fatal(got,want)}
 gp=capture(func(){g.Discard(p)});rp=capture(func(){r.Discard(p)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)}
 var gs,rs []int;gp=capture(func(){g.Ordered(p,func(n int){gs=append(gs,n)})});rp=capture(func(){r.Ordered(p,func(n int){rs=append(rs,n)})});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(gs,rs){t.Fatal(gp,rp,gs,rs)}
 gs,rs=nil,nil;gp=capture(func(){g.Finally(p,func(){gs=append(gs,1)})});rp=capture(func(){r.Finally(p,func(){rs=append(rs,1)})});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(gs,rs){t.Fatal(gp,rp,gs,rs)}
 }}
func TestBound(t *testing.T){for _,v:=range []func()int{nil,func()int{return 7}}{gp:=capture(func(){g.Bound[func()int](v)});rp:=capture(func(){r.Bound[func()int](v)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)}}}
func TestInterface(t *testing.T){for _,v:=range []error{nil,(*failure)(nil),errors.New("failure")}{var got,want any;gp:=capture(func(){got=g.Interface(v)});rp:=capture(func(){want=r.Interface(v)});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(got,want){t.Fatal(got,want,gp,rp)}}}
func TestAssertionProof(t *testing.T){for _,v:=range []any{nil,func()int(nil),func()int{return 7}}{var got,want int;gp:=capture(func(){got=g.AssertSnapshot(v)});rp:=capture(func(){want=r.AssertSnapshot(v)});if(gp==nil)!=(rp==nil)||got!=want{t.Fatal(got,want,gp,rp)}}}
func TestPanicOrder(t *testing.T){gp,rp:=capture(g.PanicOrder),capture(r.PanicOrder);if(gp==nil)!=(rp==nil)||!strings.Contains(fmt.Sprint(gp),"index out of range")||!strings.Contains(fmt.Sprint(rp),"index out of range"){t.Fatal(gp,rp)}}
`
	runGeneratedGoDifferentialTestInExistingModule(t, root, "conversion-non-null.test", generated, reference, comparison, nil, nil)
}
