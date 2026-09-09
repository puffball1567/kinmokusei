package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceGenericConstraintsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"types.km": `
constraint Rows<S extends Slice<E>, E> = ~S[];
constraint Slice<E> = Storage<E>;
constraint Storage<T> = ~T[];
constraint Lookup<K extends comparable,V> = ~Map<K,V>;
constraint Sequence<E> = (yield:(value:E)=>boolean)=>void;
constraint Receive<E> = Both<E>|ReadOnly<E>;
constraint Both<T> = GoChannel<T>;
constraint ReadOnly<T> = GoReceiveChannel<T>;
type Numbers = distinct int[];
alias Values<E,S extends Slice<E>> = S;
type Nested<E,S extends Slice<E>> = distinct S[];
function elements<S extends Slice<E>,E>(values:S):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}
function flatten<E,S extends Slice<E>,R extends Rows<S,E>>(rows:R):E[]{let result:E[]=[];for(const row of rows){for(const value of row){result=append(result,value);}}return result;}
function mapped<M extends Lookup<K,V>,K extends comparable,V>(values:M):V[]{let result:V[]=[];for(const [key,value] of values){result=append(result,value);}return result;}
function streamed<E,S extends Sequence<E>>(sequence:S):E[]{let result:E[]=[];for(const value of sequence){result=append(result,value);break;}return result;}
function drained<E,C extends Receive<E>>(channel:C):E[]{let result:E[]=[];for(const value of channel){result=append(result,value);}return result;}
function observed<S>(calls:*int,values:S):S{*calls+=1;return values;}
class Collector<E>{public function collect<S extends Slice<E>>(values:S):E[]{return elements(values);}}
class IntCollector extends Collector<int>{}
class Stored<E,S extends Slice<E>>{constructor(private values:S){}public function read():E[]{return elements(this.values);}}
struct Record<E,S extends Slice<E>>{public values:S;public function read():E[]{return elements(this.values);}}
interface Reader<E,S extends Slice<E>>{function read():S;}
class IntReader implements Reader<int,int[]>{constructor(private values:int[]){}public function read():int[]{return this.values;}}
function throughInterface<E,S extends Slice<E>>(reader:Reader<E,S>):E[]{return elements(reader.read());}
class Leaf{constructor(public value:int){}public virtual function read():int{return this.value;}}
class Twice extends Leaf{constructor(value:int){super(value);}public override function read():int{return this.value*2;}}
alias MaybeLeaf=Leaf|null;
constraint MaybeLeaves=~MaybeLeaf[];
function nullableTotal<S extends MaybeLeaves>(values:S):int{let total=0;for(const leaf of values){if(leaf!==null){total+=leaf.read();}}return total;}
function nullableObjects(flag:boolean,value:int):int{const input:MaybeLeaf[]=[null,new Twice(value)];const copied=elements(input);const stored=new Collector<MaybeLeaf>().collect(input);let selected=copied[0];if(flag){selected=copied[1];}if(selected===null){return -1;}return selected.read()+len(stored)+nullableTotal(input);}
function objects(value:int):int{const leaf:Leaf=new Twice(value);const input:Leaf[]=[leaf,new Leaf(value+1)];const result=new Collector<Leaf>().collect(input);let identity=0;if(result[0]===leaf){identity=1;}return result[0].read()+result[1].read()+identity;}
`,
		"pairs.km": `constraint Slice<E> = ~[2]E; function pair<E,A extends Slice<E>>(values:A):E[]{let result:E[]=[];for(const value of values){result=append(result,value);}return result;}`,
		"entry.km": `
import {Slice, Numbers, Values, Nested, elements, flatten, mapped, streamed, drained, observed, Collector, IntCollector, Stored, Record, IntReader, throughInterface, objects, nullableObjects} from "./types";
import {pair} from "./pairs";
constraint PublicSlice<E> = Slice<E>;
function Slices(values:int[]):int[][]{let calls=0;const result=elements(observed(&calls,values));return [result,elements<Numbers,int>(Numbers(values)),new Collector<int>().collect(values),new IntCollector().collect(values),new Stored<int,int[]>(values).read(),Record<int,int[]>{values:values}.read(),throughInterface(new IntReader(values)),[calls]];}
function Strings(values:string[]):string[]{return new Collector<string>().collect(values);}
function Flatten(values:int[][]):int[]{return flatten(Nested<int,int[]>(values));}
function Mapped(values:Map<string,int>):int[]{return mapped(values);}
function Stream(values:int[]):int[]{return streamed((yield:(value:int)=>boolean):void=>{for(const value of values){if(!yield(value)){return;}}});}
function Drain(values:GoReceiveChannel<int>):int[]{return drained(values);}
function Pair(values:[2]string):string[]{return pair(values);}
function Aliased(values:Values<int,int[]>):int[]{return elements(values);}
function Objects(value:int):int{return objects(value);}
function NullableObjects(flag:boolean,value:int):int{return nullableObjects(flag,value);}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "sourcebounds")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	if !strings.Contains(string(generated), "type PublicSlice[E any] interface") {
		t.Fatalf("missing parameterized constraint:\n%s", generated)
	}
	reference := `package reference
type Slice[E any] interface{~[]E}
type Rows[S Slice[E],E any] interface{~[]S}
type Numbers []int
type Nested[E any,S Slice[E]] []S
func elements[S Slice[E],E any](values S)[]E{result:=[]E{};for _,value:=range values{result=append(result,value)};return result}
func observed[S any](calls *int,values S)S{*calls++;return values}
type Collector[E any]struct{}
func collect[E any,S Slice[E]](_ *Collector[E],values S)[]E{return elements(values)}
type Stored[E any,S Slice[E]]struct{values S}
func(s *Stored[E,S])read()[]E{return elements(s.values)}
type Record[E any,S Slice[E]]struct{values S}
func(r Record[E,S])read()[]E{return elements(r.values)}
type Reader[E any,S Slice[E]]interface{read()S}
type IntReader struct{values []int}
func(r *IntReader)read()[]int{return r.values}
func throughInterface[E any,S Slice[E]](reader Reader[E,S])[]E{return elements(reader.read())}
func Slices(values []int)[][]int{calls:=0;result:=elements(observed(&calls,values));return [][]int{result,elements[Numbers,int](Numbers(values)),collect(&Collector[int]{},values),collect(&Collector[int]{},values),(&Stored[int,[]int]{values}).read(),(Record[int,[]int]{values}).read(),throughInterface[int,[]int](&IntReader{values}),{calls}}}
func Strings(values []string)[]string{return collect(&Collector[string]{},values)}
func flatten[E any,S Slice[E],R Rows[S,E]](rows R)[]E{result:=[]E{};for _,row:=range rows{for _,value:=range row{result=append(result,value)}};return result}
func Flatten(values [][]int)[]int{return flatten(Nested[int,[]int](values))}
func mapped[M ~map[K]V,K comparable,V any](values M)[]V{result:=[]V{};for _,value:=range values{result=append(result,value)};return result}
func Mapped(values map[string]int)[]int{return mapped(values)}
func streamed[E any,S ~func(func(E)bool)](sequence S)[]E{result:=[]E{};for value:=range sequence{result=append(result,value);break};return result}
func Stream(values []int)[]int{return streamed(func(yield func(int)bool){for _,value:=range values{if !yield(value){return}}})}
func drained[E any,C interface{chan E|<-chan E}](channel C)[]E{result:=[]E{};for value:=range channel{result=append(result,value)};return result}
func Drain(values <-chan int)[]int{return drained(values)}
func pair[E any,A ~[2]E](values A)[]E{result:=[]E{};for _,value:=range values{result=append(result,value)};return result}
func Pair(values [2]string)[]string{return pair(values)}
func Aliased(values []int)[]int{return elements(values)}
type Leaf struct{value int}
func(l *Leaf)read()int{return l.value}
type Twice struct{Leaf}
func(l *Twice)read()int{return l.value*2}
func Objects(value int)int{leaf:=&Twice{Leaf{value}};input:=[]interface{read()int}{leaf,&Leaf{value+1}};result:=collect(&Collector[interface{read()int}]{},input);identity:=0;if result[0]==leaf{identity=1};return result[0].read()+result[1].read()+identity}
func NullableObjects(flag bool,value int)int{input:=[]interface{read()int}{nil,&Twice{Leaf{value}}};copied:=elements(input);stored:=collect(&Collector[interface{read()int}]{},input);selected:=copied[0];if flag{selected=copied[1]};if selected==nil{return -1};total:=0;for _,leaf:=range input{if leaf!=nil{total+=leaf.read()}};return selected.read()+len(stored)+total}
`
	comparison := `package sourcebounds_test
import("reflect";"sort";"testing";generated "source-generic-bounds.test";reference "source-generic-bounds.test/reference")
func external[S generated.PublicSlice[int]](values S)int{total:=0;for _,value:=range values{total+=value};return total}
func closed(values []int)<-chan int{channel:=make(chan int,len(values));for _,value:=range values{channel<-value};close(channel);return channel}
func TestBehavior(t *testing.T){
 for _,values:=range [][]int{nil,{}, {0},{-3,4,5}}{
  if got,want:=generated.Slices(values),reference.Slices(values);!reflect.DeepEqual(got,want){t.Errorf("slices=%v want=%v",got,want)}
  for _,pair:=range [][2][]int{{generated.Stream(values),reference.Stream(values)},{generated.Drain(closed(values)),reference.Drain(closed(values))},{generated.Aliased(values),reference.Aliased(values)}}{if !reflect.DeepEqual(pair[0],pair[1]){t.Errorf("got=%v want=%v",pair[0],pair[1])}}
  want:=0;for _,value:=range values{want+=value};if got:=external(values);got!=want{t.Errorf("public constraint=%v want=%v",got,want)}
 }
 for _,values:=range [][]string{nil,{}, {"hello","金木犀"}}{if got,want:=generated.Strings(values),reference.Strings(values);!reflect.DeepEqual(got,want){t.Errorf("strings=%v want=%v",got,want)}}
 for _,values:=range [][][]int{nil,{}, {nil,{}}, {{1,2},{3}}}{if got,want:=generated.Flatten(values),reference.Flatten(values);!reflect.DeepEqual(got,want){t.Errorf("flatten=%v want=%v",got,want)}}
 for _,values:=range []map[string]int{nil,{}, {"a":1,"b":2}}{got,want:=generated.Mapped(values),reference.Mapped(values);sort.Ints(got);sort.Ints(want);if !reflect.DeepEqual(got,want){t.Errorf("map=%v want=%v",got,want)}}
 for _,values:=range [][2]string{{},{"one","温泉"}}{if got,want:=generated.Pair(values),reference.Pair(values);!reflect.DeepEqual(got,want){t.Errorf("pair=%v want=%v",got,want)}}
 for _,value:=range []int{-3,0,7}{if got,want:=generated.Objects(value),reference.Objects(value);got!=want{t.Errorf("objects=%v want=%v",got,want)}}
 for _,flag:=range []bool{false,true}{for _,value:=range []int{-3,0,7}{if got,want:=generated.NullableObjects(flag,value),reference.NullableObjects(flag,value);got!=want{t.Errorf("nullable=%v want=%v",got,want)}}}
}
`
	runGeneratedGoDifferentialTest(t, root, "source-generic-bounds.test", generated, reference, comparison)
}
