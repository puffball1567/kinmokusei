package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericNumericArgumentsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"go.mod": "module generic-numeric.test\n\ngo 1.23\n",
		"base.km": `function keep<T>(v:T):T{return v;}
function pick<T>(a:T,b:T):T{return b;}
function first<T>(fallback:T,...values:T[]):T{if(len(values)>0){return values[0];}return fallback;}
function pair<A,B>(a:A,b:B):B{return b;}
const Count=255;
class Box<T>{constructor(public value:T){} public function keep<U>(v:U):U{return v;} public static function pick<U>(a:U,b:T):T{return b;}}
`,
		"entry.km": `import {keep,pick,first,pair,Count,Box} from "./base";
import go cmp from "cmp";import go slices from "slices";import go math from "math";import go unicode from "unicode";
type Signal=distinct float32;
function Narrow():float32{return keep<float32>(1.25);}
function Byte():byte{return keep<byte>(Count);}
function Named():Signal{return keep<Signal>(1.25);}
function Left(v:float32):float32{return pick(1.25,v);}
function Right(v:float32):float32{return pick(v,1.25);}
function Promoted():float{return pick(1,2.5);}
function Rune():int32{return pick(1,unicode.ReplacementChar);}
function Complex():complex128{return pick(1.5,2i);}
function NarrowComplex():complex64{return keep<complex64>(1+2i);}
function Partial():float32{return pair<int,float32>(1,1.25);}
function InferredPartial(v:float32):float32{return pair<int>(1,v);}
function Variadic(v:float32):float32{return first(0,1.25,v);}
function Spread(v:float32[]):float32{return first(0,v...);}
function GoCompare():int{return cmp.Compare<float32>(math.Pi,3.25)+cmp.Compare(1,2.5);}
function GoComplex(v:complex64[]):int{return slices.Index(v,2i);}
function Methods():float32{const b=new Box<int>(3);return b.keep<float32>(1.25)+Box.pick("tag",float32(2.5));}
function ImportedConstant():float32{const pi=math.Pi;return keep<float32>(pi+1);}
function Rounded():float32{const n:float32=16777217.;return keep(n)-16777216.;}
let trace:int=0;
function value(v:float32):float32{trace++;return v;}
function Evaluation():int{trace=0;const v=pick(1.25,value(2.5));return trace+int(v)*10;}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericnumeric")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("cmp";"slices";"math";"unicode")
func keep[T any](v T)T{return v}
func pick[T any](a,b T)T{return b}
func first[T any](fallback T,values ...T)T{if len(values)>0{return values[0]};return fallback}
func pair[A,B any](a A,b B)B{return b}
func Narrow()float32{return keep[float32](1.25)}
func Byte()byte{const Count=255;return keep[byte](Count)}
type Signal float32
func Named()Signal{return keep[Signal](1.25)}
func Left(v float32)float32{return pick(1.25,v)}
func Right(v float32)float32{return pick(v,1.25)}
func Promoted()float64{return pick(1,2.5)}
func Rune()int32{return pick(1,unicode.ReplacementChar)}
func Complex()complex128{return pick(1.5,2i)}
func NarrowComplex()complex64{return keep[complex64](1+2i)}
func Partial()float32{return pair[int,float32](1,1.25)}
func InferredPartial(v float32)float32{return pair[int](1,v)}
func Variadic(v float32)float32{return first(0,1.25,v)}
func Spread(v []float32)float32{return first(0,v...)}
func GoCompare()int{return cmp.Compare[float32](math.Pi,3.25)+cmp.Compare(1,2.5)}
func GoComplex(v []complex64)int{return slices.Index(v,2i)}
func Methods()float32{return keep[float32](1.25)+pair[string,float32]("tag",float32(2.5))}
func ImportedConstant()float32{const pi=math.Pi;return keep[float32](pi+1)}
func Rounded()float32{const n float32=16777217.;return keep(n)-16777216.}
func Evaluation()int{trace:=0;value:=func(v float32)float32{trace++;return v};v:=pick(1.25,value(2.5));return trace+int(v)*10}
`
	comparison := `package genericnumeric_test
import("testing";g "generic-numeric.test";r "generic-numeric.test/reference")
func TestNumericGenerics(t *testing.T){
if g.Narrow()!=r.Narrow()||g.Byte()!=r.Byte()||float32(g.Named())!=float32(r.Named())||g.Promoted()!=r.Promoted()||g.Rune()!=r.Rune()||g.Complex()!=r.Complex()||g.NarrowComplex()!=r.NarrowComplex()||g.Partial()!=r.Partial(){t.Fatal("constants/defaults/explicit")}
for _,v:=range []float32{-2.5,0,3.75}{if g.Left(v)!=r.Left(v)||g.Right(v)!=r.Right(v)||g.Variadic(v)!=r.Variadic(v)||g.InferredPartial(v)!=r.InferredPartial(v){t.Fatal("typed inference")}}
for _,v:=range [][]float32{nil,{1.25,3.5}}{if g.Spread(v)!=r.Spread(v){t.Fatal("spread")}}
for _,v:=range [][]complex64{nil,{1+2i,2i},{3i}}{if g.GoComplex(v)!=r.GoComplex(v){t.Fatal("Go complex")}}
if g.GoCompare()!=r.GoCompare()||g.Methods()!=r.Methods()||g.ImportedConstant()!=r.ImportedConstant()||g.Rounded()!=r.Rounded()||g.Evaluation()!=r.Evaluation(){t.Fatal("interop/methods/rounding/evaluation")}
}
`
	runGeneratedGoDifferentialTest(t, root, "generic-numeric.test", generated, reference, comparison)
}
