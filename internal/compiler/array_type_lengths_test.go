package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArrayTypeLengthsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, directory := range []string{"limits", "state"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"go.mod": "module array-type-lengths.test\n\ngo 1.23\n",
		"state/state.go": `package state
var Count int`,
		"limits/limits.go": `package limits
import "array-type-lengths.test/state"
const Width=3
func init(){state.Count++}`,
		"sizes.km": `export const Width=3;
export struct Box<T>{public items:[Width]T;}
export function row():[Width]int{return [4,5,6];}`,
		"bridge.km": `export {Width as Count,Box,row} from "./sizes";`,
		"entry.km": `import {Count as Width,Box,row} from "./bridge";
import go {Size as DigestSize} from "crypto/sha256";
import go sha256 from "crypto/sha256";
import go md5 from "crypto/md5";
import go limits from "array-type-lengths.test/limits";
import go state from "array-type-lengths.test/state";
alias MD5=[md5.Size]byte;
alias External=[limits.Width]int;
const typedWidth:int=3;
alias Row=[Width]int;
enum Sizes {Zero,Three=3}
class Limits {
 public data:[Limits.hidden]int=[1,2,3];
 public static const count:int=Limits.hidden;
 private static const hidden:int=3;
 protected static const protectedCount:int=3;
 public static const storedCount:int=cap(Limits.stored);
 public static stored:[3]int=makeSeed();
 public function echo(value:[Limits.hidden]int):[Limits.hidden]int{return value;}
}
class Child extends Limits {public static const inheritedCount:int=Child.protectedCount;}
alias ClassRow=[Child.inheritedCount]int;
const seed:[3]int=makeSeed();
const initialized:[len(seed)]int=[4,5,6];
function makeSeed():[len(initialized)]int{return [1,2,3];}
export function Roundtrip(value:Row):[typedWidth]int{return value;}
export function Digest(value:byte[]):[DigestSize]byte{return sha256.Sum256(value);}
export function Imported():[Width]int{return row();}
export function OnlyLength(value:MD5):[16]byte{return value;}
export function Initialization(value:External):int{return state.Count+value[0];}
export function Shadow():int[]{const Width=2;const a:[Width]int=[7,8];const f=(b:[Width]int):[Width]int=>b;const copy=copyArray[[Width]int]([1,2]);const view=viewArray[[Width]int]([3,4]);return [len(f(a)),copy[1],view[0]];}
export function Generic<T>(value:Box<T>):[Width]T{return value.items;}
export function Objects():int[]{const a:[Sizes.Three]int=[1,2,3];const b:ClassRow=a;const c=new Child();const result:[Limits.storedCount]int=c.echo(a);return [len(b),len(c.data),seed[0],initialized[1],result[1]];}
export function Ignored(index:int):[3]int{const matrix:[1][Width]int=[[1,2,3]];const a:[len(matrix[index])]int=[4,5,6];const nilArray:*[3]int=nil;const b:[cap(nilArray)]int=a;return b;}
export function Copied():int[]{let original:[max(2,min(3,4))]int=[1,2,3];let duplicate:[len("湯")]int=original;duplicate[0]=9;return [original[0],duplicate[0],len(duplicate)];}
export function Lookup(values:Map<[Width]byte,int>,key:[Width]byte):int{return values[key];}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "arraytypelengths")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{"[3]int", "[32]byte", "[2]int", "[3]T", "map[[3]byte]int"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("missing %q:\n%s", want, generated)
		}
	}
	for _, path := range []string{"crypto/md5", "array-type-lengths.test/limits"} {
		if !strings.Contains(string(generated), `_ "`+path+`"`) {
			t.Fatalf("folded type length lost package initialization for %s:\n%s", path, generated)
		}
	}
	reference := `package reference
import "crypto/sha256"
const width=3
type Box[T any] struct{Items [width]T}
type sizes int
const three sizes=3
const count int=3
type limits struct{data [count]int}
type child struct{limits}
var seed [3]int=makeSeed()
var initialized [len(seed)]int=[3]int{4,5,6}
func makeSeed()[len(initialized)]int{return [3]int{1,2,3}}
func Roundtrip(value [width]int)[1+2]int{return value}
func Digest(value []byte)[sha256.Size]byte{return sha256.Sum256(value)}
func Imported()[width]int{return [3]int{4,5,6}}
func OnlyLength(value [16]byte)[16]byte{return value}
func Initialization(value [3]int)int{return 1+value[0]}
func Shadow()[]int{const width=2;a:=[width]int{7,8};f:=func(b [width]int)[width]int{return b};copy:=[width]int([]int{1,2});view:=(*[width]int)([]int{3,4});return []int{len(f(a)),copy[1],view[0]}}
func Generic[T any](value Box[T])[width]T{return value.Items}
func Objects()[]int{a:=[three]int{1,2,3};b:=[count]int(a);c:=child{limits{data:[count]int{1,2,3}}};result:=a;return []int{len(b),len(c.data),seed[0],initialized[1],result[1]}}
func Ignored(index int)[3]int{matrix:=[1][width]int{{1,2,3}};a:=[len(matrix[index])]int{4,5,6};var nilArray *[3]int;b:=[cap(nilArray)]int(a);return b}
func Copied()[]int{original:=[max(2,min(3,4))]int{1,2,3};duplicate:=[len("湯")]int(original);duplicate[0]=9;return []int{original[0],duplicate[0],len(duplicate)}}
func Lookup(values map[[width]byte]int,key [width]byte)int{return values[key]}
`
	comparison := `package arraytypelengths
import (
 "reflect"
 "testing"
 reference "array-type-lengths.test/reference"
)
func TestLengths(t *testing.T){
 for _,input:=range [][3]int{{},{1,2,3},{-5,7,10}}{if got,want:=Roundtrip(input),reference.Roundtrip(input);got!=want{t.Fatalf("roundtrip=%v Go=%v",got,want)}}
 for _,input:=range [][]byte{nil,{},[]byte("温泉"),{0,255}}{if got,want:=Digest(input),reference.Digest(input);got!=want{t.Fatalf("digest=%x Go=%x",got,want)}}
 if got,want:=Imported(),reference.Imported();got!=want{t.Fatalf("import=%v Go=%v",got,want)}
 if got,want:=OnlyLength([16]byte{1,2}),reference.OnlyLength([16]byte{1,2});got!=want{t.Fatalf("length-only import=%v Go=%v",got,want)}
 if got,want:=Initialization([3]int{5}),reference.Initialization([3]int{5});got!=want{t.Fatalf("package initialization=%v Go=%v",got,want)}
 for _,test:=range []struct{name string;got,want []int}{{"shadow",Shadow(),reference.Shadow()},{"objects",Objects(),reference.Objects()},{"copy",Copied(),reference.Copied()}}{if !reflect.DeepEqual(test.got,test.want){t.Fatalf("%s=%v Go=%v",test.name,test.got,test.want)}}
 if got,want:=Generic(Box[int]{Items:[3]int{7,8,9}}),reference.Generic(reference.Box[int]{Items:[3]int{7,8,9}});got!=want{t.Fatalf("generic=%v Go=%v",got,want)}
 for _,index:=range []int{-1,0,100}{if got,want:=Ignored(index),reference.Ignored(index);got!=want{t.Fatalf("ignored(%d)=%v Go=%v",index,got,want)}}
 for _,key:=range [][3]byte{{1,2,3},{}}{values:=map[[3]byte]int{{1,2,3}:9};if got,want:=Lookup(values,key),reference.Lookup(values,key);got!=want{t.Fatalf("map=%d Go=%d",got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "array-type-lengths.test", generated, reference, comparison)
}
