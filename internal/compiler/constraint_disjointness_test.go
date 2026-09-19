package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParameterConstraintIntersectionsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module parameter-intersections.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Storage[E any] interface{~[]E|~[2]E|~map[string]E}
type Sequence[E any] interface{~[]E|~[3]E}
type Slice[E any] interface{Storage[E];Sequence[E]}
`,
		"bounds.km": `import go contracts from "parameter-intersections.test/contracts";
export constraint Slice<E>=contracts.Storage<E>&contracts.Sequence<E>;
export constraint Array<E>=contracts.Storage<E>&~[2]E;
export constraint Mapping<E>=contracts.Storage<E>&~Map<string,E>;
export type Named<E>=distinct E[];
type Other<E>=distinct E[];
constraint Names<E>=Named<E>|Other<E>;
export constraint Exact<E>=Names<E>&Named<E>;
`,
		"bridge.km": `export {Slice as Slices,Array,Mapping,Exact,Named} from "./bounds";`,
		"entry.km": `import {Slices,Array,Mapping,Exact,Named} from "./bridge";
import go contracts from "parameter-intersections.test/contracts";
function replace<E,S extends Slices<E>>(xs:S,value:E):S{xs[0]=value;return xs;}
function imported<E,S extends contracts.Slice<E>>(xs:S):E{return xs[0];}
function named<E,S extends Exact<E>>(xs:S):E{return xs[0];}
function array<E,A extends Array<E>>(xs:A,value:E):E{xs[0]=value;return xs[0];}
function mapping<E,M extends Mapping<E>>(xs:M,value:E):E{xs["key"]=value;return xs["key"];}
export function Values():int[]{const xs:Named<int>=Named<int>([3,4]);const changed=replace(xs,8);const pair:[2]int=[1,2];const value=array(pair,7);const map=makeMap<string,int>();const mapped=mapping(map,9);return [xs[0],changed[0],named(xs),imported<int,Named<int>>(xs),pair[0],value,mapped,map["key"]];}
class Item{constructor(public value:int){}}
alias Maybe=Item|null;
class First<E>{public function get<S extends Slices<E>>(xs:S):E{return xs[0];}}
export function Objects():int[]{const item=new Item(42);const xs:Maybe[]=[null];replace<Maybe,Maybe[]>(xs,item);const value=new First<Maybe>().get<Maybe[]>(xs);let identity=0;let result=0;if(value===item){identity=1;}if(value!==null){result=value.value;}replace<Maybe,Maybe[]>(xs,null);let empty=0;if(new First<Maybe>().get<Maybe[]>(xs)===null){empty=1;}return [result,identity,empty];}
export function Nil():void{const xs:int[]=nil;replace(xs,1);}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "intersections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type storage[E any] interface{~[]E|~[2]E|~map[string]E}
type sequence[E any] interface{~[]E|~[3]E}
type slices[E any] interface{storage[E];sequence[E]}
type arrays[E any] interface{storage[E];~[2]E}
type mapping[E any] interface{storage[E];~map[string]E}
type named[E any] []E
type other[E any] []E
type names[E any] interface{named[E]|other[E]}
type exact[E any] interface{names[E];named[E]}
func replace[E any,S slices[E]](xs S,value E)S{xs[0]=value;return xs}
func first[E any,S slices[E]](xs S)E{return xs[0]}
func exactFirst[E any,S exact[E]](xs S)E{return xs[0]}
func array[E any,A arrays[E]](xs A,value E)E{xs[0]=value;return xs[0]}
func set[E any,M mapping[E]](xs M,value E)E{xs["key"]=value;return xs["key"]}
func Values()[]int{xs:=named[int]{3,4};changed:=replace(xs,8);pair:=[2]int{1,2};value:=array(pair,7);m:=map[string]int{};mapped:=set(m,9);return []int{xs[0],changed[0],exactFirst(xs),first[int](xs),pair[0],value,mapped,m["key"]}}
type item struct{value int}
func Objects()[]int{object:=&item{42};xs:=[]*item{nil};replace(xs,object);value:=first[*item](xs);identity,result:=0,0;if value==object{identity=1};if value!=nil{result=value.value};replace(xs,nil);empty:=0;if first[*item](xs)==nil{empty=1};return []int{result,identity,empty}}
func Nil(){var xs []int;replace(xs,1)}
`
	comparison := `package intersections_test
import("testing";"reflect";g "parameter-intersections.test";r "parameter-intersections.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestIntersections(t *testing.T){for _,pair:=range [][2]func()[]int{{g.Values,r.Values},{g.Objects,r.Objects}}{if got,want:=pair[0](),pair[1]();!reflect.DeepEqual(got,want){t.Fatalf("got=%v want=%v",got,want)}};if !panics(g.Nil)||!panics(r.Nil){t.Fatal("nil indexing must panic")}}
`
	runGeneratedGoDifferentialTest(t, root, "parameter-intersections.test", generated, reference, comparison)
	for _, body := range []string{
		`function bad(xs:[2]int):void{use<int,[2]int>(xs);}`,
		`class Item{}alias Maybe=Item|null;function bad(xs:Maybe[]):void{use<Item,Maybe[]>(xs);}`,
	} {
		input := `import {Slices} from "./bridge";function use<E,S extends Slices<E>>(xs:S):void{}` + body
		path := filepath.Join(root, "bad.km")
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
		_, diagnostics, err := EmitGo([]string{path}, "bad")
		if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "constraint") {
			t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
		}
	}
}
