package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericClosersMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"types.km": `alias Items<E>=E[];
constraint Slice<E>=~E[];
alias Checked<E,S extends Slice<E>>=S;
class Box<T>{constructor(public value:T){}}
function identity<T>(value:T):T{return value;}
`,
		"entry.km": `import {Items,Slice,Checked,Box,identity} from "./types";
class Holder{public box:Box<Box<int>>=new Box<Box<int>>(new Box<int>(3));}
function Values(value:int):int{const items:Items<int>=[value,2];const checked:Checked<int,int[]>=items;let result=0;for(let box:Box<Box<int>>=new Box<Box<int>>(new Box<int>(0));box.value.value<2;box.value.value++){result+=checked[box.value.value];}return result+new Holder().box.value.value;}
function Nested(value:int):int{const box:Box<Box<Box<int>>>=new Box<Box<Box<int>>>(new Box<Box<int>>(new Box<int>(value)));return box.value.value.value;}
function Call(value:int):int{return identity<Box<int>>(new Box<int>(value)).value;}
function Compare(a:int,b:int,c:uint):boolean{return a<b>>c;}
function Index(a:int,b:int,c:uint):int{const flags=makeMap<boolean,int>();flags[true]=11;flags[false]=7;return flags[a<b>>c];}
function Shift(value:int,count:uint):int{value>>=count;return value;}
function Greater(a:int,b:int):boolean{return a>=b;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericclosers")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type Box[T any]struct{value T}
func identity[T any](value T)T{return value}
func Values(value int)int{items:=[]int{value,2};result:=0;for box:=(&Box[*Box[int]]{&Box[int]{0}});box.value.value<2;box.value.value++{result+=items[box.value.value]};return result+3}
func Nested(value int)int{box:=&Box[*Box[*Box[int]]]{&Box[*Box[int]]{&Box[int]{value}}};return box.value.value.value}
func Call(value int)int{return identity(&Box[int]{value}).value}
func Compare(a,b int,c uint)bool{return a<b>>c}
func Index(a,b int,c uint)int{flags:=map[bool]int{true:11,false:7};return flags[a<b>>c]}
func Shift(value int,count uint)int{value>>=count;return value}
func Greater(a,b int)bool{return a>=b}
`
	comparison := `package genericclosers_test
import("testing";generated "generic-closers.test";reference "generic-closers.test/reference")
func TestClosers(t *testing.T){for _,a:=range []int{-15,-1,0,1,16}{
 if generated.Values(a)!=reference.Values(a)||generated.Nested(a)!=reference.Nested(a)||generated.Call(a)!=reference.Call(a){t.Error("generic assignment or call")}
 for _,b:=range []int{-20,0,5,32}{for _,c:=range []uint{0,1,3,64}{
  if generated.Compare(a,b,c)!=reference.Compare(a,b,c)||generated.Index(a,b,c)!=reference.Index(a,b,c){t.Error("speculative shift/comparison")}
  if generated.Shift(a,c)!=reference.Shift(a,c)||generated.Greater(a,b)!=reference.Greater(a,b){t.Error("ordinary operators")}
 }}
}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-closers.test", generated, reference, comparison)
}
