package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTypeParameterConversionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"convert.km": `
constraint Signed = ~int8 | ~int64;
constraint Real = ~float32 | ~float64;
constraint Text = ~string;
constraint Bytes = ~byte[];
constraint Triple = ~[3]int;
function T(value: int): int { return value + 100; }
function Convert<T extends Signed>(value: int): T { return T(value); }
function Between<T extends Signed, U extends Real>(value: U): T { return T(value); }
function Round<T extends Real>(): T { const value = T(1.1); return value + T(1.2); }
function TextOf<T extends Text>(value: int32): T { return T(value); }
function BytesOf<T extends Bytes>(value: string): T { return T(value); }
function ArrayOf<T extends Triple>(value: int[]): T { return T(value); }
function Identity<T>(value: T): T { return T(value); }
function Builtin<len extends Signed>(value: int): len { return len(value); }
function Shadow<T>(): int { { const T = (value: int): int => value; return T(3); } }
class Counter<T extends Signed> {
  private value: T;
  constructor() { this.value = T(0); }
  public function step(value: int): T { this.value += T(value); return this.value; }
  public function converted<U extends Signed>(): U { return U(this.value); }
}
`,
		"entry.km": `
import { Convert, Between, Round, TextOf, BytesOf, ArrayOf, Identity, Counter, Shadow, Builtin } from "./convert";
type Tiny = distinct int8;
class Leaf { constructor(public value: int) {} }
class Small extends Counter<int8> { constructor() { super(); } }
function Narrow(value: int): Tiny { return Convert<Tiny>(value); }
function Truncate(value: float64): int8 { return Between<int8>(value); }
function Rounded32(): float32 { return Round<float32>(); }
function Rounded64(): float64 { return Round<float64>(); }
function Shadowed(): int { return Shadow<string>(); }
function BuiltinName(value: int): int8 { return Builtin<int8>(value); }
function Text(value: int32): string { return TextOf<string>(value); }
function Bytes(value: string): byte[] { return BytesOf<byte[]>(value); }
function Array(value: int[]): [3]int { return ArrayOf<[3]int>(value); }
function Same(value: int): boolean { const leaf = new Leaf(value); return Identity(leaf) === leaf; }
function Count(value: int): int64 { const counter = new Small(); counter.step(value); counter.step(value); return counter.converted<int64>(); }
let calls = 0;
function observed(): int { calls++; return 130; }
function Once(): int { calls = 0; const result = Convert<int8>(observed()); return calls * 1000 + int(result); }
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "parameterconversion")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type Tiny int8
func Convert[T ~int8 | ~int64](value int) T { return T(value) }
func Between[T ~int8 | ~int64, U ~float32 | ~float64](value U) T { return T(value) }
func Round[T ~float32 | ~float64]() T { value := T(1.1); return value + T(1.2) }
func TextOf[T ~string](value int32) T { return T(value) }
func BytesOf[T ~[]byte](value string) T { return T(value) }
func ArrayOf[T ~[3]int](value []int) T { return T(value) }
func Identity[T any](value T) T { return T(value) }
func Narrow(value int) Tiny { return Convert[Tiny](value) }
func Truncate(value float64) int8 { return Between[int8](value) }
func Rounded32() float32 { return Round[float32]() }
func Rounded64() float64 { return Round[float64]() }
func Shadow[T any]() int { { T := func(value int) int { return value }; return T(3) } }
func Shadowed() int { return Shadow[string]() }
func Builtin[len ~int8 | ~int64](value int) len { return len(value) }
func BuiltinName(value int) int8 { return Builtin[int8](value) }
func Text(value int32) string { return TextOf[string](value) }
func Bytes(value string) []byte { return BytesOf[[]byte](value) }
func Array(value []int) [3]int { return ArrayOf[[3]int](value) }
func Same(value int) bool { leaf := &struct { value int }{value}; return Identity(leaf) == leaf }
type Counter[T ~int8 | ~int64] struct { value T }
func (c *Counter[T]) Step(value int) T { c.value += T(value); return c.value }
func Count(value int) int64 { counter := &Counter[int8]{value:int8(0)}; counter.Step(value); counter.Step(value); return int64(counter.value) }
func Once() int { calls := 0; observed := func() int { calls++; return 130 }; result := Convert[int8](observed()); return calls*1000 + int(result) }
`
	comparison := `package parameterconversion_test
import (
 "testing"
 "reflect"
 generated "type-parameter-conversion.test"
 reference "type-parameter-conversion.test/reference"
)
func panics(call func()) (result bool) { defer func() { result = recover() != nil }(); call(); return }
func TestConversions(t *testing.T) {
 if generated.Shadowed() != reference.Shadowed() { t.Error("lexical conversion target shadowing") }
 for _, value := range []int{-300,-129,-128,-1,0,1,127,128,255,300} {
  if int8(generated.Narrow(value)) != int8(reference.Narrow(value)) { t.Error("narrow conversion") }
  if generated.BuiltinName(value) != reference.BuiltinName(value) { t.Error("type parameter shadows builtin") }
  if generated.Count(value) != reference.Count(value) { t.Error("generic class conversion") }
  if generated.Same(value) != reference.Same(value) { t.Error("identity conversion") }
 }
 for _, value := range []float64{-100.75,-1.2,0,1.9,100.4} { if generated.Truncate(value) != reference.Truncate(value) { t.Error("parameter-to-parameter conversion") } }
 if generated.Rounded32() != reference.Rounded32() || generated.Rounded64() != reference.Rounded64() { t.Error("nonconstant floating conversion") }
 for _, value := range []int32{-1,0,65,0x266c,0x110000} { if generated.Text(value) != reference.Text(value) { t.Error("integer string conversion") } }
 for _, value := range []string{"", "hello", "温泉", "\xff"} { if !reflect.DeepEqual(generated.Bytes(value), reference.Bytes(value)) { t.Error("byte slice conversion") } }
 for _, values := range [][]int{nil,{}, {1,2}, {1,2,3}, {1,2,3,4}} {
  gp := panics(func() { generated.Array(values) }); rp := panics(func() { reference.Array(values) }); if gp != rp { t.Error("array conversion panic") }; if !gp && generated.Array(values) != reference.Array(values) { t.Error("array conversion") }
 }
 if generated.Once() != reference.Once() { t.Error("single evaluation") }
}
`
	runGeneratedGoDifferentialTest(t, root, "type-parameter-conversion.test", generated, reference, comparison)
}

func TestTypeParameterConversionRejectsUnrepresentableConstants(t *testing.T) {
	for _, expression := range []string{"1.5", "128.0", "1e100", "1 + 127", "-129"} {
		t.Run(expression, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.km")
			input := "constraint Small = ~int8 | ~int64; function invalid<T extends Small>(): T { return T(" + expression + "); }"
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, err := EmitGo([]string{path}, "invalid")
			if err == nil && len(diagnostics) == 0 {
				t.Fatal("accepted an unrepresentable type-parameter conversion")
			}
		})
	}
}
