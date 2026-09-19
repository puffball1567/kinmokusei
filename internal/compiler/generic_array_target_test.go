package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericArrayTargetsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module generic-array-target.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Array[E any] interface{~[0]E|~[2]E|~[3]E}
type Slice[E any] interface{~[]E}
type Pair [2]int
`,
		"bounds.km": `import go contracts from "generic-array-target.test/contracts";
export constraint Array<E>=contracts.Array<E>;
export constraint Slice<E>=contracts.Slice<E>;
export function convert<E,A extends Array<E>,S extends Slice<E>>(values:S):A{return copyArray[A](values);}
`,
		"exports.km": `export {Array,Slice,convert} from "./bounds";`,
		"entry.km": `import {Array,Slice,convert} from "./exports";
import go contracts from "generic-array-target.test/contracts";
type Pair=distinct [2]int;
type Triple=distinct [3]int;
type Numbers=distinct int[];
constraint Named=Pair|Triple;
function named<A extends Named>(values:int[]):A{return copyArray[A](values);}
class Copier<E>{public function copy<A extends Array<E>,S extends Slice<E>>(values:S):A{return copyArray[A](values);}}
class Item{constructor(public value:int){}}
alias Maybe=Item|null;
export function Scalars():int[]{const values=Numbers([1,2,3]);let pair=convert<int,Pair,Numbers>(values);const triple=convert<int,Triple,Numbers>(values);const external=convert<int,contracts.Pair,Numbers>(values);pair[0]=9;return [values[0],pair[0],triple[2],external[1],len(named<Pair>([4,5]))];}
export function Objects():int[]{const values:Maybe[]=[new Item(3),null];let pair=new Copier<Maybe>().copy<[2]Maybe,Maybe[]>(values);const first=pair[0];if(first!==null){first.value=7;}pair[1]=new Item(9);const original=values[0];let result=0;if(original!==null){result=original.value;}let flag=0;if(values[1]===null){flag=1;}return [result,flag];}
export function Zero():int{const values:int[]=nil;const result=convert<int,[0]int,int[]>(values);return len(result);}
function measured<A extends Array<int>>(values:int[]):int{const n=len(copyArray[A](values));const p=&n;return *p;}
export function Length(values:int[]):int{return measured<[2]int>(values);}
export function Short(values:int[]):[3]int{return convert<int,[3]int,int[]>(values);}
export function Evaluation():int[]{let calls=0;const next=():int[]=>{calls++;return [4,5];};const result=convert<int,Pair,int[]>(next());return [calls,result[0],result[1]];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "arraytargets")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "A(values)") {
		t.Fatalf("missing type parameter conversion:\n%s", generated)
	}
	reference := `package reference
type array[E any] interface{~[0]E|~[2]E|~[3]E}
type pair [2]int
type triple [3]int
type externalPair [2]int
type numbers []int
func convert[E any,A array[E],S ~[]E](values S)A{return A(values)}
func named[A interface{pair|triple}](values []int)A{return A(values)}
func Scalars()[]int{values:=numbers{1,2,3};p:=convert[int,pair](values);t:=convert[int,triple](values);external:=convert[int,externalPair](values);p[0]=9;return []int{values[0],p[0],t[2],external[1],len(named[pair]([]int{4,5}))}}
type item struct{value int}
func Objects()[]int{values:=[]*item{{3},nil};p:=convert[*item,[2]*item](values);first:=p[0];if first!=nil{first.value=7};p[1]=&item{9};original:=values[0];result:=0;if original!=nil{result=original.value};flag:=0;if values[1]==nil{flag=1};return []int{result,flag}}
func Zero()int{var values []int;result:=convert[int,[0]int](values);return len(result)}
func measured[A array[int]](values []int)int{n:=len(A(values));p:=&n;return *p}
func Length(values []int)int{return measured[[2]int](values)}
func Short(values []int)[3]int{return convert[int,[3]int](values)}
func Evaluation()[]int{calls:=0;next:=func()[]int{calls++;return []int{4,5}};result:=convert[int,pair](next());return []int{calls,result[0],result[1]}}
`
	comparison := `package arraytargets_test
import("testing";"reflect";g "generic-array-target.test";r "generic-array-target.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestTargets(t *testing.T){for _,pair:=range [][2][]int{{g.Scalars(),r.Scalars()},{g.Objects(),r.Objects()},{g.Evaluation(),r.Evaluation()}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Fatalf("got %v want %v",pair[0],pair[1])}};if g.Zero()!=r.Zero(){t.Fatal("zero")}}
func TestLengths(t *testing.T){for _,values:=range [][]int{nil,make([]int,0,8),make([]int,1,8),{1,2},{1,2,3}}{var got,want [3]int;gp:=panics(func(){got=g.Short(values)});rp:=panics(func(){want=r.Short(values)});if gp!=rp||gp!=(len(values)<3)||got!=want{t.Fatal("short conversion")};var gn,rn int;gp=panics(func(){gn=g.Length(values)});rp=panics(func(){rn=r.Length(values)});if gp!=rp||gp!=(len(values)<2)||gn!=rn{t.Fatal("runtime length")}}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-array-target.test", generated, reference, comparison)
}
