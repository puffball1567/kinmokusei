package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScalarConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module scalar-constants.test\n\ngo 1.23\n",
		"values.km": `import go {MethodGet} from "net/http";
const Greeting=Prefix+"泉";const Prefix="温";
const Method=MethodGet;const Enabled=1.0<2.0;
export {Greeting,Method,Enabled};`,
		"bridge.km": `export {Greeting as TextValue,Method,Enabled} from "./values";`,
		"entry.km": `import {TextValue,Method,Enabled} from "./bridge";
import go cmp from "cmp";
type Text=distinct string;type Flag=distinct boolean;
function choose<T>(a:T,b:T):T{return a;}
class Box<T>{public function choose(a:T,b:T):T{return b;}}
export function Strings():string[]{const first=TextValue;const second=first+"!";const method:Text=Method;const suffix:Text="!";const typed="go"+suffix;return [string(first),string(second),string(method),string(typed)];}
export function Booleans():boolean[]{const first=Enabled;const second=!first;const flag:Flag=first;const inverted=!flag;const typed=true&&inverted;const comparison=TextValue<"雪";return [boolean(first),boolean(second),boolean(typed),boolean(comparison)];}
export function Lengths():int[]{const text=TextValue;const length=len(text);const next=length+1;const named:Text=text;const size=len(named);const xs=makeSlice<int>(length,next);return [length,next,size,int(text[length-1]),len(text[:length]),cap(xs),int(byte(length))];}
export function Generics():string{const text="left";const right:Text="right";const flag:Flag=true;const box=new Box<Text>();const inferred=choose(text,right);const explicit=choose<Text>(text,right);const selected=box.choose(text,right);const order=cmp.Compare(text,right);if(choose(false,flag)){return "bad";}return string(inferred)+string(explicit)+string(selected)+string(int32(order+50));}
export function Conditions(value:boolean):int{let flag:Flag=Flag(value);let total=0;if(flag){total++;}while(flag){total++;break;}for(;flag;){total++;flag=false;}return total;}
export function Shadow():string{const original="outer";const alias=original;const capture=():string=>{const original="inner";const result=alias+original;return result;};return capture();}
export function Runtime():int{let count=0;const get=():string=>{count++;return "abc";};const yes=():boolean=>{count++;return true;};const initial=get();const stored=initial;const address=&stored;let changing="x";const saved=changing;changing="long";const skip=false&&yes();const kept=skip;const boolAddress=&kept;const piece="abcd"[:2];const size=len(piece);const sizeAddress=&size;return count*100+len(*address)+len(saved)+*sizeAddress;}
export function Loop():int{let total=0;for(const text="abc";total==0;){const alias=text;const address=&alias;total=len(*address);}return total;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "scalarconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const second = first +", "const inverted = !flag", "const length = len(text)", "const next = length + 1", "var stored = initial", "var kept = skip", "var size = len(piece)", "var alias = text"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import("net/http";"cmp")
type text string;type flag bool
const greeting="温"+"泉";const enabled=1.0<2.0
func choose[T any](a,b T)T{return a}
func Strings()[]string{const first=greeting;const second=first+"!";const method text=http.MethodGet;const suffix text="!";const typed="go"+suffix;return []string{string(first),string(second),string(method),string(typed)}}
func Booleans()[]bool{const first=enabled;const second=!first;const value flag=first;const inverted=!value;const typed=true&&inverted;const comparison=greeting<"雪";return []bool{bool(first),bool(second),bool(typed),bool(comparison)}}
func Lengths()[]int{const value=greeting;const length=len(value);const next=length+1;const named text=value;const size=len(named);xs:=make([]int,length,next);return []int{length,next,size,int(value[length-1]),len(value[:length]),cap(xs),int(byte(length))}}
func Generics()string{const value="left";const right text="right";const truth flag=true;inferred:=choose(value,right);explicit:=choose[text](value,right);selected:=right;order:=cmp.Compare(value,right);if choose(false,truth){return "bad"};return string(inferred)+string(explicit)+string(selected)+string(int32(order+50))}
func Conditions(value bool)int{v:=flag(value);total:=0;if v{total++};for v{total++;break};for ;v;{total++;v=false};return total}
func Shadow()string{const original="outer";const alias=original;capture:=func()string{const original="inner";const result=alias+original;return result};return capture()}
func Runtime()int{count:=0;get:=func()string{count++;return "abc"};yes:=func()bool{count++;return true};initial:=get();stored:=initial;address:=&stored;changing:="x";saved:=changing;changing="long";_=changing;skip:=false&&yes();kept:=skip;boolAddress:=&kept;_=boolAddress;piece:="abcd"[:2];size:=len(piece);sizeAddress:=&size;return count*100+len(*address)+len(saved)+*sizeAddress}
func Loop()int{total:=0;for text:="abc";total==0;{alias:=text;address:=&alias;total=len(*address)};return total}
`
	comparison := `package scalarconstants_test
import("testing";"reflect";g "scalar-constants.test";r "scalar-constants.test/reference")
func TestScalar(t *testing.T){if !reflect.DeepEqual(g.Strings(),r.Strings())||!reflect.DeepEqual(g.Booleans(),r.Booleans())||!reflect.DeepEqual(g.Lengths(),r.Lengths()){t.Fatal("constants")};if g.Generics()!=r.Generics()||g.Shadow()!=r.Shadow()||g.Runtime()!=r.Runtime()||g.Loop()!=r.Loop(){t.Fatal("inference/storage")};for _,v:=range []bool{false,true}{if g.Conditions(v)!=r.Conditions(v){t.Fatal("conditions")}}}
`
	runGeneratedGoDifferentialTest(t, root, "scalar-constants.test", generated, reference, comparison)
}
