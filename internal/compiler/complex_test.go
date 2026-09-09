package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComplexNumbersMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"go.mod": "module complex-numbers.test\n\ngo 1.23\n",
		"base.km": `import go cm from "math/cmplx";
type Signal=distinct complex128;
class Box<T>{constructor(public value:T){}}
function conjugate(value:complex128):complex128{return cm.Conj(value);}
const Seed:complex64=complex(1.5,-2.25);
`,
		"entry.km": `import {Signal,Box,conjugate,Seed} from "./base";
import go math from "math";
function Build(a:float,b:float):complex128{return complex(a,b);}
function Narrow(a:float32,b:float32):complex64{return complex(a,b);}
function Components(z:complex64):float32[]{return [real(z),imag(z)];}
function Arithmetic(a:complex128,b:complex128):complex128{let z=-a+b*2;z+=a;z/=b;z++;return z;}
function Constant():complex64{return Seed+complex(2.5,1.25);}
function Convert(z:complex128):complex64{return complex64(z);}
function Named(a:float,b:float):complex128{const z:Signal=Signal(complex(a,b));return complex128(z);}
function Stored(z:complex128):complex128{const b=new Box<complex128>(z);return conjugate(b.value);}
function Lookup(z:complex128):int{const m=makeMap<complex128,int>();m[z]=7;return m[z];}
function Projection():int{return int(real(complex(6,4))+imag(complex(3,5)));}
let trace:int=0;
function next(value:float,tag:int):float{trace=trace*10+tag;return value;}
function Evaluation():int{trace=0;const z=complex(next(2,1),next(3,2));return trace+int(real(z))*100+int(imag(z))*1000;}
function Special(which:int):complex128{let value:float=math.NaN();if(which===1){value=math.Inf(1);}if(which===2){value=math.Copysign(0,-1);}return complex(value,value);}
function Rounding():float{const z:complex64=complex(16777217,0);return float(real(z));}
function Binding():complex128{const a=complex(1,2);const b=a+a;const c=complex(real(b),imag(b));return c;}
constraint Complex=~complex128;
function twice<T extends Complex>(z:T):T{return z+z;}
function Generic(z:complex128):complex128{return twice(z);}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "complexnumbers")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("math";"math/cmplx")
func Build(a,b float64)complex128{return complex(a,b)}
func Narrow(a,b float32)complex64{return complex(a,b)}
func Components(z complex64)[]float32{return []float32{real(z),imag(z)}}
func Arithmetic(a,b complex128)complex128{z:=-a+b*2;z+=a;z/=b;z++;return z}
func Constant()complex64{const seed complex64=complex(1.5,-2.25);return seed+complex(2.5,1.25)}
func Convert(z complex128)complex64{return complex64(z)}
type Signal complex128
func Named(a,b float64)complex128{return complex128(Signal(complex(a,b)))}
func Stored(z complex128)complex128{return cmplx.Conj(z)}
func Lookup(z complex128)int{m:=map[complex128]int{};m[z]=7;return m[z]}
func Projection()int{return int(real(complex(6,4))+imag(complex(3,5)))}
func Evaluation()int{trace:=0;next:=func(v float64,tag int)float64{trace=trace*10+tag;return v};z:=complex(next(2,1),next(3,2));return trace+int(real(z))*100+int(imag(z))*1000}
func Special(which int)complex128{value:=math.NaN();if which==1{value=math.Inf(1)};if which==2{value=math.Copysign(0,-1)};return complex(value,value)}
func Rounding()float64{const z complex64=complex(16777217,0);return float64(real(z))}
func Binding()complex128{const a=complex(1,2);b:=a+a;return complex(real(b),imag(b))}
func Generic(z complex128)complex128{return z+z}
`
	comparison := `package complexnumbers_test
import("testing";"math";"reflect";g "complex-numbers.test";r "complex-numbers.test/reference")
func equal(a,b complex128)bool{return (real(a)==real(b)||math.IsNaN(real(a))&&math.IsNaN(real(b)))&&(imag(a)==imag(b)||math.IsNaN(imag(a))&&math.IsNaN(imag(b)))}
func TestContracts(t *testing.T){for _,a:=range []float64{-5.25,0,1.5,1048576}{for _,b:=range []float64{-3.5,0,2.25}{
z:=g.Build(a,b);if z!=r.Build(a,b)||g.Named(a,b)!=r.Named(a,b)||g.Stored(z)!=r.Stored(z)||g.Convert(z)!=r.Convert(z){t.Fatal("construction/conversion/storage")}
n:=g.Narrow(float32(a),float32(b));if n!=r.Narrow(float32(a),float32(b))||!reflect.DeepEqual(g.Components(n),r.Components(n)){t.Fatal("width/projection")}
if !equal(g.Arithmetic(z,complex(b,a)),r.Arithmetic(z,complex(b,a))){t.Fatal("arithmetic")}
if g.Lookup(z)!=r.Lookup(z){t.Fatal("map")}
if g.Generic(z)!=r.Generic(z){t.Fatal("generic operators")}
}}
if g.Constant()!=r.Constant()||g.Projection()!=r.Projection()||g.Evaluation()!=r.Evaluation(){t.Fatal("constants/evaluation")}
if g.Rounding()!=r.Rounding()||g.Binding()!=r.Binding(){t.Fatal("typed constant rounding and runtime bindings")}
for _,i:=range []int{0,1,2}{a,b:=g.Special(i),r.Special(i);if !equal(a,b)||math.Signbit(real(a))!=math.Signbit(real(b))||math.Signbit(imag(a))!=math.Signbit(imag(b)){t.Fatal("NaN/infinity/signed zero")}}
}
`
	runGeneratedGoDifferentialTest(t, root, "complex-numbers.test", generated, reference, comparison)
}
