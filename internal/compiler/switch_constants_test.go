package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchConstantsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":    "module switch-constants.test\n\ngo 1.23\n",
		"values.km": `export const Word="a"+"b";export const Three=min(3,4);`,
		"bridge.km": `export {Word,Three} from "./values";`,
		"entry.km": `import {Word,Three} from "./bridge";
import go reflect from "reflect";
type N=distinct int;type Text=distinct string;
constraint Number=~int|~int64;
export function Classify(value:reflect.Value):int{switch(value.Interface()){case 1{return 1;}case int64(1){return 2;}case 1.0{return 3;}case float32(1){return 4;}case N(1){return 5;}case Word{return 6;}case Text("ab"){return 7;}case nil{return 8;}default{return 9;}}}
export function Named():int[]{return [Classify(reflect.ValueOf(N(1))),Classify(reflect.ValueOf(Text("ab")))];}
export function Defaults():int[]{let a=0;let b=0;switch(Three){case 3{a=1;}}switch(1.0){case 1{b=2;}}return [a,b];}
export function Rounded(value:float32):int{switch(value){case 0.1{return 1;}case 16777217.0{return 2;}default{return 3;}}}
export function Booleans(value:boolean):int{switch(value){case true{return 1;}case true{return 2;}default{return 3;}}}
export function Complex(value:complex128):int{switch(value){case 1i{return 1;}case 1i{return 2;}default{return 3;}}}
export function InterfaceFractions(value:reflect.Value):int{switch(value.Interface()){case 0.1{return 1;}case 0.1{return 2;}case 9007199254740992.0{return 3;}case 9007199254740993.0{return 4;}default{return 5;}}}
function generic<T extends Number>(value:T):int{switch(value){case T(1){return 1;}case T(1){return 2;}default{return 3;}}}
export function Generic(value:int):int[]{return [generic(value),generic(int64(value))];}
export function Runtime(value:int):int{for(const i=1;i<2;){switch(value){case i{return 1;}case 1{return 2;}}break;}return 3;}
export function Evaluation():int{let trace=0;const tag=():int=>{trace=trace*10+1;return 2;};const label=(digit:int,value:int):int=>{trace=trace*10+digit;return value;};switch(tag()){case label(2,0),label(3,2){trace=trace*10+5;fallthrough;}case label(4,2){trace=trace*10+6;}default{trace=0;}}return trace;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "switchconstants")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "reflect"
type number int;type text string
const word="a"+"b";const three=min(3,4)
func Classify(value reflect.Value)int{switch value.Interface(){case 1:return 1;case int64(1):return 2;case 1.0:return 3;case float32(1):return 4;case number(1):return 5;case word:return 6;case text("ab"):return 7;case nil:return 8;default:return 9}}
func Named()[]int{return []int{Classify(reflect.ValueOf(number(1))),Classify(reflect.ValueOf(text("ab")))}}
func Defaults()[]int{a,b:=0,0;switch three{case 3:a=1};switch 1.0{case 1:b=2};return []int{a,b}}
func Rounded(value float32)int{switch value{case 0.1:return 1;case 16777217.0:return 2;default:return 3}}
func Booleans(value bool)int{switch value{case true:return 1;case true:return 2;default:return 3}}
func Complex(value complex128)int{switch value{case 1i:return 1;case 1i:return 2;default:return 3}}
func InterfaceFractions(value reflect.Value)int{switch value.Interface(){case 0.1:return 1;case 0.1:return 2;case 9007199254740992.0:return 3;case 9007199254740993.0:return 4;default:return 5}}
func generic[T interface{~int|~int64}](value T)int{switch value{case T(1):return 1;case T(1):return 2;default:return 3}}
func Generic(value int)[]int{return []int{generic(value),generic(int64(value))}}
func Runtime(value int)int{for i:=1;i<2;{switch value{case i:return 1;case 1:return 2};break};return 3}
func Evaluation()int{trace:=0;tag:=func()int{trace=trace*10+1;return 2};label:=func(digit,value int)int{trace=trace*10+digit;return value};switch tag(){case label(2,0),label(3,2):trace=trace*10+5;fallthrough;case label(4,2):trace=trace*10+6;default:trace=0};return trace}
`
	comparison := `package switchconstants_test
import("testing";"reflect";g "switch-constants.test";r "switch-constants.test/reference")
func TestInterfaceFractions(t *testing.T){for _,n:=range []float64{0,0.1,9007199254740992,9007199254740994}{value:=reflect.ValueOf(n);if g.InterfaceFractions(value)!=r.InterfaceFractions(value){t.Fatal("interface fraction",n)}}}
func TestSwitch(t *testing.T){for _,value:=range []any{1,int64(1),float64(1),float32(1),"ab","other",nil}{rv:=reflect.ValueOf(&value).Elem();if g.Classify(rv)!=r.Classify(rv){t.Fatalf("classify %T",value)}};if !reflect.DeepEqual(g.Named(),r.Named())||!reflect.DeepEqual(g.Defaults(),r.Defaults())||g.Evaluation()!=r.Evaluation(){t.Fatal("named/default/evaluation")};for _,value:=range []float32{0,0.1,16777216,16777218}{if g.Rounded(value)!=r.Rounded(value){t.Fatal("rounding",value)}};for _,value:=range []bool{false,true}{if g.Booleans(value)!=r.Booleans(value){t.Fatal("bool")}};for _,value:=range []complex128{0,1i,2i}{if g.Complex(value)!=r.Complex(value){t.Fatal("complex")}};for _,value:=range []int{0,1,2}{if !reflect.DeepEqual(g.Generic(value),r.Generic(value))||g.Runtime(value)!=r.Runtime(value){t.Fatal("runtime/generic",value)}}}
`
	runGeneratedGoDifferentialTest(t, root, "switch-constants.test", generated, reference, comparison)
}

func TestLinkedSwitchDuplicateConstants(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, declaration, target, other string }{
		{"string", `export const Value="a"+"b";`, "string", `"ab"`},
		{"rounded", `export const Value:float32=16777217.;`, "float32", "16777216."},
		{"ordered", `export const Value=min(3,4);`, "int", "3"},
		{"Go import", `import go http from "net/http";export const Value=http.MethodGet;`, "string", `"GET"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"values.km": test.declaration,
				"bridge.km": `export {Value as Alias} from "./values";`,
				"entry.km":  `import {Alias} from "./bridge";function f(v:` + test.target + `):void{switch(v){case Alias{}case ` + test.other + `{}}}`,
			}
			for name, input := range files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "switchconstants")
			if err != nil {
				t.Fatal(err)
			}
			for _, diagnostic := range diagnostics {
				if strings.Contains(diagnostic.Message, "duplicate value switch case") {
					return
				}
			}
			t.Fatalf("missing duplicate case diagnostic: %v", diagnostics)
		})
	}
}
