package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedConstraintCompositionMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module imported-constraints.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Integer interface{~int|~int64}
type Nested interface{Integer;~int}
type Overlap interface{Nested|int}
type Slice[E any] interface{~[]E}
type Pair[E any] interface{~[2]E}
type Lookup[K comparable,V any] interface{~map[K]V}
type Key interface{comparable}
type Filtered interface{Key;~int|~[]int}
type Any = any
`,
		"bounds.km": `import go { Ordered } from "cmp";
import go c from "imported-constraints.test/contracts";
import go fmt from "fmt";
constraint Integer=Ordered&c.Nested;
constraint Named=Integer&fmt.Stringer;
constraint Items<E>=c.Slice<E>;
constraint Pair<E>=c.Pair<E>&comparable;
constraint Lookup<K extends comparable,V>=c.Lookup<K,V>;
constraint Key=c.Key;
export {Integer as Number,Named,Items,Pair,Lookup,Key};`,
		"bridge.km": `export {Number as Small,Named,Items,Pair,Lookup,Key} from "./bounds";`,
		"entry.km": `import {Small,Named,Items,Pair,Lookup,Key} from "./bridge";
import go c from "imported-constraints.test/contracts";
import go strconv from "strconv";
constraint Overlap=c.Overlap;
constraint Choice=Small|~string;
constraint Filtered=c.Filtered;
type Score=distinct int;
public function string(this:Score):string{return strconv.Itoa(int(this));}
function double<T extends Small>(x:T):T{return x*2;}
function format<T extends Named>(x:T):string{const show=(x+x).String;return show();}
function keep<T extends Choice>(x:T):T{return x;}
function overlap<T extends Overlap>(x:T):T{return x*3;}
function filtered<T extends Filtered>(x:T):T{return x*4;}
function copied<S extends Items<E>,E>(xs:S):E[]{let out:E[]=[];for(const x of xs){out=append(out,x);}return out;}
function equal<T extends Key>(a:T,b:T):boolean{return a===b;}
function pairEqual<E extends comparable,T extends Pair<E>>(a:T,b:T):boolean{return a===b;}
function total<M extends Lookup<K,int>,K extends comparable>(xs:M):int{let result=0;for(const [key,value] of xs){result+=value;}return result;}
class Holder<E>{public function copy<S extends Items<E>>(xs:S):E[]{return copied(xs);}}
class Leaf{constructor(public value:int){}}
alias Maybe=Leaf|null;
function Twice(x:int):int{return double(x);}
function Format(x:int):string{return format(Score(x));}
function Text(x:string):string{return keep(x);}
function Overlapped(x:int):int{return overlap(x);}
function Filter(x:int):int{return filtered(x);}
function Copy(xs:int[]):int[]{return new Holder<int>().copy(xs);}
function Equal(a:string,b:string):boolean{return equal(a,b);}
function Dynamic(a:c.Any,b:c.Any):boolean{return equal<c.Any>(a,b);}
function Pairs(a:[2]int,b:[2]int):boolean{return pairEqual(a,b);}
function Total(xs:Map<string,int>):int{return total(xs);}
function Nullable(x:int):int{const xs:Maybe[]=[new Leaf(x),null];const result=copied(xs);let value=0;for(const leaf of result){if(leaf!==null){value+=leaf.value;}else{value+=1;}}return value;}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "importedconstraints")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{`"cmp"`, ".Ordered", "c.Nested", "c.Slice[E]", "comparable", "c.Overlap"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	reference := `package reference
import("cmp";"fmt";"strconv")
type Number interface{cmp.Ordered;~int}
type Named interface{Number;fmt.Stringer}
type Score int
func(s Score)String()string{return strconv.Itoa(int(s))}
func double[T Number](x T)T{return x*2}
func format[T Named](x T)string{show:=(x+x).String;return show()}
func Twice(x int)int{return double(x)}
func Format(x int)string{return format(Score(x))}
func Text(x string)string{return x}
func Overlapped(x int)int{return x*3}
func Filter(x int)int{return x*4}
type Items[E any]interface{~[]E}
func copied[S Items[E],E any](xs S)[]E{out:=[]E{};for _,x:=range xs{out=append(out,x)};return out}
func Copy(xs []int)[]int{return copied(xs)}
func equal[T comparable](a,b T)bool{return a==b}
func Equal(a,b string)bool{return equal(a,b)}
func Dynamic(a,b any)bool{return equal[any](a,b)}
type Pair[E any]interface{~[2]E;comparable}
func pairEqual[E comparable,T Pair[E]](a,b T)bool{return a==b}
func Pairs(a,b [2]int)bool{return pairEqual(a,b)}
func Total(xs map[string]int)int{total:=0;for _,x:=range xs{total+=x};return total}
type Leaf struct{value int}
func Nullable(x int)int{xs:=[]*Leaf{{x},nil};result:=copied(xs);value:=0;for _,leaf:=range result{if leaf!=nil{value+=leaf.value}else{value++}};return value}
`
	comparison := `package importedconstraints_test
import("reflect";"testing";g "imported-constraints.test";r "imported-constraints.test/reference")
func TestNumbers(t *testing.T){for _,x:=range []int{-8,0,21}{if g.Twice(x)!=r.Twice(x)||g.Format(x)!=r.Format(x)||g.Overlapped(x)!=r.Overlapped(x)||g.Filter(x)!=r.Filter(x)||g.Nullable(x)!=r.Nullable(x){t.Fatalf("numeric mismatch: %d",x)}}}
func TestCollections(t *testing.T){for _,xs:=range [][]int{nil,{}, {1,-4,9}}{if !reflect.DeepEqual(g.Copy(xs),r.Copy(xs)){t.Fatalf("copy mismatch: %v",xs)}};for _,xs:=range []map[string]int{nil,{}, {"a":2,"b":5}}{if g.Total(xs)!=r.Total(xs){t.Fatal("map mismatch")}};for _,a:=range [][2]int{{1,2},{2,1}}{for _,b:=range [][2]int{{1,2},{2,1}}{if g.Pairs(a,b)!=r.Pairs(a,b){t.Fatal("pair mismatch")}}}}
func TestEquality(t *testing.T){for _,a:=range []string{"","alpha","beta"}{if g.Text(a)!=r.Text(a){t.Fatal("text mismatch")};for _,b:=range []string{"","alpha","beta"}{if g.Equal(a,b)!=r.Equal(a,b)||g.Dynamic(a,b)!=r.Dynamic(a,b){t.Fatal("equality mismatch")}}}}
func panics(f func())(result bool){defer func(){result=recover()!=nil}();f();return}
func TestDynamicEqualityPanic(t *testing.T){if !panics(func(){g.Dynamic([]int{1},[]int{1})})||!panics(func(){r.Dynamic([]int{1},[]int{1})}){t.Fatal("interface equality must retain Go panic behavior")}}
`
	runGeneratedGoDifferentialTest(t, root, "imported-constraints.test", generated, reference, comparison)
}

func TestImportedConstraintLinkedDiagnostics(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"excluded type", `function keep<T extends Number>(x:T):T{return x;} function bad(x:string):string{return keep(x);}`, "does not satisfy"},
		{"empty", `constraint Bad=Number&~boolean;`, "no common types"},
		{"union comparable", `constraint Bad=Key|~int;`, "without method or comparable requirements"},
		{"union overlap", `constraint Bad=Number|~int;`, "overlaps an earlier term"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for name, contents := range map[string]string{
				"bounds.km": `import go cmp from "cmp"; constraint Integer=cmp.Ordered&~int; constraint Key=comparable; export {Integer as Number,Key};`,
				"bridge.km": `export {Number,Key} from "./bounds";`,
				"entry.km":  `import {Number,Key} from "./bridge";` + test.source,
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "invalidconstraints")
			found := false
			for _, diagnostic := range diagnostics {
				found = found || strings.Contains(diagnostic.Message, test.want)
			}
			if err != nil || !found || len(generated) != 0 {
				t.Fatalf("err=%v diagnostics=%v generated=%s", err, diagnostics, generated)
			}
		})
	}
}
