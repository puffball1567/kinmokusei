package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNullableTypeParameterRejectedBeforeGoGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	input := `function orElse<T>(x:T|null,d:T):T{if(x===null){return d;}return x;}`
	if err := os.WriteFile(entry, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFiles([]string{entry})
	if err != nil || len(result.Diagnostics) == 0 {
		t.Fatalf("check err=%v diagnostics=%v", err, result.Diagnostics)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "nullableparameters")
	if err != nil || len(generated) != 0 || len(diagnostics) == 0 {
		t.Fatalf("emit err=%v diagnostics=%v bytes=%d", err, diagnostics, len(generated))
	}
	if diagnostics[0].Span.Path != entry || !strings.Contains(diagnostics[0].Message, "type T cannot be nullable because it has no nil representation") {
		t.Fatalf("missing source diagnostic: %v", diagnostics)
	}
}

func TestNullableTypeParameterConstraintsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	input := `alias Callback=()=>int;
constraint Reference=~*int|~int[]|~Map<int,int>|~GoChannel<int>|Callback;
export function Missing<T extends Reference>(value:T|null):boolean{return value===null;}
constraint Pointer=~*int;
export function OrElse<T extends Pointer>(value:T|null,fallback:T):T{if(value===null){return fallback;}return value;}
export function Pointee<T>(value:*T|null):boolean{return value===null;}
export function Slice<T>(value:T[]|null):boolean{return value===null;}
`
	if err := os.WriteFile(entry, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "nullableparameters")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type reference interface{~*int|~[]int|~map[int]int|~chan int|func()int}
func Missing[T reference](value T)bool{return value==nil}
func OrElse[T ~*int](value,fallback T)T{if value==nil{return fallback};return value}
func Pointee[T any](value *T)bool{return value==nil}
func Slice[T any](value []T)bool{return value==nil}
`
	comparison := `package nullableparameters_test
import("testing";g "nullable-parameters.test";r "nullable-parameters.test/reference")
func TestNilRepresentations(t *testing.T){
 n:=7
 for _,v:=range []*int{nil,&n}{if g.Missing(v)!=r.Missing(v)||g.OrElse(v,&n)!=r.OrElse(v,&n)||g.Pointee(v)!=r.Pointee(v){t.Fatal("pointer")}}
 for _,v:=range [][]int{nil,{}, {1}}{if g.Missing(v)!=r.Missing(v)||g.Slice(v)!=r.Slice(v){t.Fatal("slice")}}
 for _,v:=range []map[int]int{nil,{}, {1:2}}{if g.Missing(v)!=r.Missing(v){t.Fatal("map")}}
 for _,v:=range []chan int{nil,make(chan int)}{if g.Missing(v)!=r.Missing(v){t.Fatal("channel")}}
 for _,v:=range []func()int{nil,func()int{return 7}}{if g.Missing(v)!=r.Missing(v){t.Fatal("callback")}}
}
`
	runGeneratedGoDifferentialTest(t, root, "nullable-parameters.test", generated, reference, comparison)
}
