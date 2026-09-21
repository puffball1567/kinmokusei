package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectionMultipleResultsMatchesGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `let calls=0;
function appendInputs():(int[],int,int){calls++;return [1],2,3;}
function copyInputs(dst:int[]):(int[],int[]){calls++;return dst,[4,5];}
function textInputs(dst:byte[]):(byte[],string){calls++;return dst,"ok";}
function deleteInputs(m:Map<string,int>):(Map<string,int>,string){calls++;return m,"key";}
function schedule(m:Map<string,int>):void{defer delete(deleteInputs(m));m["during"]=calls+len(m)*10;}
function Deferred():int{calls=0;const m=makeMap<string,int>();m["key"]=1;schedule(m);return m["during"]*100+len(m);}
function appended<T>(values:T[],item:T):T[]{const inputs=():(T[],T)=>{return values,item;};return append(inputs());}
function Generic():int{return appended([6],7)[1];}
function numbers():(int,int,int){calls++;return 8,-2,4;}
function parts():(float,float){calls++;return 1.5,-2.5;}
constraint Ordered=~int|~string;
function low<T extends Ordered>(a:T,b:T):T{const pair=():(T,T)=>{return a,b;};return min(pair());}
function Numeric():complex128{calls=0;const lo=min(numbers());const hi=max(numbers());const z=complex(parts());return z+complex(float(lo+hi+low(5,3)),float(calls));}
function words():(string,string){return "z","a";}
function Text():string{return min(words())+low("b","a");}
function narrowParts():(float32,float32){return 1.5,-2.5;}
function Narrow():complex64{return complex(narrowParts());}
function Extrema(a:float,b:float):(float,float){const pair=():(float,float)=>{return a,b;};return min(pair()),max(pair());}
function Run():int{
 calls=0;const a=append(appendInputs());const b:int[]=[0,0];
 const n=copy(copyInputs(b));const text:byte[]=[0,0];const size=copy(textInputs(text));
 const m=makeMap<string,int>();m["key"]=1;delete(deleteInputs(m));
 return a[2]+b[1]+n+size+len(m)+int(text[0])+calls*1000;
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "collections")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
var calls int
func appendInputs()([]int,int,int){calls++;return []int{1},2,3}
func copyInputs(dst []int)([]int,[]int){calls++;return dst,[]int{4,5}}
func textInputs(dst []byte)([]byte,string){calls++;return dst,"ok"}
func deleteInputs(m map[string]int)(map[string]int,string){calls++;return m,"key"}
func schedule(m map[string]int){dm,dk:=deleteInputs(m);defer delete(dm,dk);m["during"]=calls+len(m)*10}
func Deferred()int{calls=0;m:=map[string]int{"key":1};schedule(m);return m["during"]*100+len(m)}
func appended[T any](values []T,item T)[]T{return append(values,item)}
func Generic()int{return appended([]int{6},7)[1]}
func numbers()(int,int,int){calls++;return 8,-2,4}
func parts()(float64,float64){calls++;return 1.5,-2.5}
func low[T interface{~int|~string}](a,b T)T{return min(a,b)}
func Numeric()complex128{calls=0;a,b,c:=numbers();lo:=min(a,b,c);d,e,f:=numbers();hi:=max(d,e,f);r,i:=parts();z:=complex(r,i);return z+complex(float64(lo+hi+low(5,3)),float64(calls))}
func Text()string{return min("z","a")+low("b","a")}
func Narrow()complex64{return complex(float32(1.5),float32(-2.5))}
func Extrema(a,b float64)(float64,float64){return min(a,b),max(a,b)}
// Bind explicitly: Go 1.26+ vet panics on tuple arguments to collection built-ins.
func Run()int{calls=0;dst,x,y:=appendInputs();a:=append(dst,x,y);b:=[]int{0,0};cd,cs:=copyInputs(b);n:=copy(cd,cs);text:=[]byte{0,0};td,ts:=textInputs(text);size:=copy(td,ts);m:=map[string]int{"key":1};dm,dk:=deleteInputs(m);delete(dm,dk);return a[2]+b[1]+n+size+len(m)+int(text[0])+calls*1000}
`
	comparison := `package collections_test
import("testing";"math";g "collection-results.test";r "collection-results.test/reference")
func TestRun(t *testing.T){if got,want:=g.Run(),r.Run();got!=want{t.Fatalf("got %d want %d",got,want)};if got,want:=g.Deferred(),r.Deferred();got!=want{t.Fatalf("deferred capture got %d want %d",got,want)};if got,want:=g.Generic(),r.Generic();got!=want{t.Fatalf("generic result got %d want %d",got,want)}}
func TestNumeric(t *testing.T){if got,want:=g.Numeric(),r.Numeric();got!=want{t.Fatalf("numeric %v != %v",got,want)};if g.Text()!=r.Text()||g.Narrow()!=r.Narrow(){t.Fatal("string/generic or complex64")};same:=func(a,b float64)bool{return (math.IsNaN(a)&&math.IsNaN(b))||(a==b&&math.Signbit(a)==math.Signbit(b))};for _,p:=range [][2]float64{{0,math.Copysign(0,-1)},{math.NaN(),1},{1,math.NaN()},{math.Inf(1),math.Inf(-1)}}{a,b:=g.Extrema(p[0],p[1]);x,y:=r.Extrema(p[0],p[1]);if !same(a,x)||!same(b,y){t.Fatalf("extrema %v",p)}}}
`
	runGeneratedGoDifferentialTest(t, root, "collection-results.test", generated, reference, comparison)
}
