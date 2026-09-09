package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNumericLiteralsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `function Integers():int[]{return [0b101,0B_110,0o10,0O_11,012,0_13,0xFF,0X_FF,1_000];}
function Floats():float[]{return [.5,1.,1e2,1E-2,1_2.3_4e+1,0x1.fp2,0X_1.FP-2,0x.8p0];}
function Imaginaries():complex128[]{return [2i,0123i,08i,0_123i,.25i,2.e3i,1E-2i,0b11i,0o10i,0xFi,0x1.fp2i];}
function Arithmetic():complex64{return (1e1+2.5i)*(0x1p-1-1i);}
function Narrow():float32{return 1.25e2;}
function WideConstant():float{return 1e400/1e400;}
function ArrayLength(a:[010]int):[0x8]int{return a;}
function Index(a:[0x10]int):int{return a[010];}
function Bounds(a:int[]):int[]{return a[0b1:0o3];}
function Inference():complex128{let z=2i;z+=1;return z;}
class Box<T>{constructor(public value:T){}}
function Stored():complex64{const b=new Box<complex64>(1+2i);return b.value;}
function Convert():int{return int(1e2);}
function Rounding():float32{const z:complex64=1.6777217e7i;return imag(z);}
function BaseSwitch(v:int):int{switch(v){case 010{return 1;}case 0x10{return 2;}}return 0;}
function FloatSwitch(v:float):int{switch(v){case 1e2{return 1;}case 0x1p3{return 2;}}return 0;}
`
	if err := os.WriteFile(filepath.Join(root, "entry.km"), []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "numericliterals")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
func Integers()[]int{return []int{0b101,0B_110,0o10,0O_11,012,0_13,0xFF,0X_FF,1_000}}
func Floats()[]float64{return []float64{.5,1.,1e2,1E-2,1_2.3_4e+1,0x1.fp2,0X_1.FP-2,0x.8p0}}
func Imaginaries()[]complex128{return []complex128{2i,0123i,08i,0_123i,.25i,2.e3i,1E-2i,0b11i,0o10i,0xFi,0x1.fp2i}}
func Arithmetic()complex64{return (1e1+2.5i)*(0x1p-1-1i)}
func Narrow()float32{return 1.25e2}
func WideConstant()float64{return 1e400/1e400}
func ArrayLength(a [010]int)[0x8]int{return a}
func Index(a [0x10]int)int{return a[010]}
func Bounds(a []int)[]int{return a[0b1:0o3]}
func Inference()complex128{z:=2i;z+=1;return z}
func Stored()complex64{return 1+2i}
func Convert()int{return int(1e2)}
func Rounding()float32{const z complex64=1.6777217e7i;return imag(z)}
func BaseSwitch(v int)int{switch v{case 010:return 1;case 0x10:return 2};return 0}
func FloatSwitch(v float64)int{switch v{case 1e2:return 1;case 0x1p3:return 2};return 0}
`
	comparison := `package numericliterals_test
import("testing";"reflect";g "numeric-literals.test";r "numeric-literals.test/reference")
func TestLiterals(t *testing.T){
if !reflect.DeepEqual(g.Integers(),r.Integers())||!reflect.DeepEqual(g.Floats(),r.Floats())||!reflect.DeepEqual(g.Imaginaries(),r.Imaginaries()){t.Fatal("literal values")}
if g.Arithmetic()!=r.Arithmetic()||g.Narrow()!=r.Narrow()||g.WideConstant()!=r.WideConstant()||g.Inference()!=r.Inference()||g.Stored()!=r.Stored()||g.Convert()!=r.Convert()||g.Rounding()!=r.Rounding(){t.Fatal("numeric semantics")}
a:=[8]int{1,2,3,4,5,6,7,8};b:=[16]int{0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15}
if g.ArrayLength(a)!=r.ArrayLength(a)||g.Index(b)!=r.Index(b)||!reflect.DeepEqual(g.Bounds(a[:]),r.Bounds(a[:])){t.Fatal("array lengths/index/slice")}
for _,v:=range []int{0,8,16,100}{if g.BaseSwitch(v)!=r.BaseSwitch(v)||g.FloatSwitch(float64(v))!=r.FloatSwitch(float64(v)){t.Fatal("switch")}}
}
`
	runGeneratedGoDifferentialTest(t, root, "numeric-literals.test", generated, reference, comparison)
}
