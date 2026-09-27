package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"model.km": `import go math from "math";
const suffix="泉";
alias T=int;
export type Small=distinct int16;
export class Limits<T>{
 public static const size:int=Limits.next+1;
 private static const next:int=2;
 protected static const hidden:int=4;
 public static const text:string="温"+suffix;
 public static const enabled:boolean=Limits.size>0;
 public static const narrow:byte=255;
 public static const rounded:float32=16777217;
 public static const fraction:float=math.Pi;
 public static const imaginary:complex64=complex(2,3);
 public static const named:Small=Small(7);
 public static const lexical:T=6;
 public static const length:int=len(Limits.text);
 public static const bound:int=max(2,min(4,3));
 public static array:[2]int=[1,2];
 public static const capacity:int=cap(Limits.array);
 public value:int=Limits.size;
 public static get count():int{return Limits.size;}
}
export class Child extends Limits<string>{public static const total:int=Child.hidden+Limits.size;}
`,
		"bridge.km": `export {Limits as Settings,Child,Small} from "./model";`,
		"entry.km": `import {Settings,Child,Small} from "./bridge";
export {Settings,Child,Small} from "./bridge";
const forward:int=Settings.size;
function pick<T>(a:T,b:T):T{return b;}
export function Numbers():int[]{const a=Settings.size;const b=Child.size;const xs=makeSlice<int>(a);return [a,b,forward,len(xs),Child.total,Settings.lexical,Settings.length,Settings.bound,Settings.capacity,new Settings<int>().value];}
export function Text():string{return Settings.text+string(int32(len(Settings.text)+59));}
export function Values():float[]{const rounded=Settings.rounded;return [float(rounded),Settings.fraction,float(real(Settings.imaginary)),float(imag(Settings.imaginary)),float(Settings.named),float(pick(1,Settings.size))];}
export function Switch(v:int):int{switch(v){case Child.size{return 1;}case Child.total{return 2;}default{return 0;}}}
export function Logic():boolean{return Settings.enabled&&!false;}
export function Scope():int{const suffix="wrong";const next=90;const value=():int=>{const next=80;return Settings.size;};return value();}
export function Runtime():int{let value=Settings.size;const p=&value;*p+=2;return value;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "classconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"const LimitsSize int", "const LimitsRounded float32", "const a = LimitsSize"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import "math"
type small int16
const size int=next+1
const next int=2
const total int=4+size
const text string="温"+"泉"
const rounded float32=16777217
const fraction float64=math.Pi
const imaginary complex64=complex(2,3)
const named small=7
func Numbers()[]int{const a=size;xs:=make([]int,a);return []int{a,size,size,len(xs),total,6,len(text),max(2,min(4,3)),2,size}}
func Text()string{return text+string(int32(len(text)+59))}
func Values()[]float64{return []float64{float64(rounded),fraction,float64(real(imaginary)),float64(imag(imaginary)),float64(named),float64(size)}}
func Switch(v int)int{switch(v){case size:return 1;case total:return 2;default:return 0}}
func Logic()bool{return size>0&&!false}
func Scope()int{return size}
func Runtime()int{v:=size;p:=&v;*p+=2;return v}
`
	comparison := `package classconstants_test
import("testing";"reflect";g "class-constants.test";r "class-constants.test/reference")
const external int=g.LimitsSize
var array [g.LimitsSize]int
const typed g.Small=g.LimitsNamed
func TestConstants(t *testing.T){if !reflect.DeepEqual(g.Numbers(),r.Numbers())||!reflect.DeepEqual(g.Values(),r.Values())||g.Text()!=r.Text()||g.Logic()!=r.Logic()||g.Scope()!=r.Scope()||g.Runtime()!=r.Runtime(){t.Fatal("class constants")};for _,n:=range []int{0,3,7,9}{if g.Switch(n)!=r.Switch(n){t.Fatal("switch",n)}};if external!=3||len(array)!=3||typed!=7||g.LimitsNarrow!=255{t.Fatal("public Go constants")}}
`
	runGeneratedGoDifferentialTest(t, root, "class-constants.test", generated, reference, comparison)
}
