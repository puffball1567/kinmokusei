package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericFunctionValuesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `import go errors from "errors";
export function Identity<T>(v:T):T{return v;}
export function Pair<T,U>(a:T,b:U):(T,U){return a,b;}
export function First<T>(...vs:T[]):T{return vs[0];}
export function Checked<T>(v:T,success:boolean):Result<T>{if(!success){return fail(errors.New("failure"));}return ok(v);}
export type Number=distinct int;
export struct Box<T>{public value:T;}
`,
		"entry.km": `import {Identity as Id,Pair,First,Checked,Number,Box} from "./models";
import go {Clone} from "slices";
const global=Id<int>;
export function Run(n:int):int[]{
 const angle=Id<int>;const bracket=Id[int];const callbacks=[angle,bracket];
 const pair=Pair<int,int>;const [left,right]=pair(n,n+1);
 const first=First<int>;const values=[n+2,n+3];
 const clone=Clone<int[],int>;const copied=clone(values);copied[0]+=10;
 const nominal=Id<Number>;const box=Id<Box<int>>;
 const identityCallback=Id<(v:int)=>int>;const callback=identityCallback((v:int):int=>v+4);
 const task=go angle(n+5);
 return [global(n),callbacks[0](n),callbacks[1](n+1),left,right,first(values...),values[0],copied[0],int(nominal(Number(n+6))),box(Box<int>{value:n+7}).value,callback(n),await task];
}
export function Text(v:string):string{const identity=Id<string>;return identity(v);}
export function ResultValue(n:int,success:boolean):Result<int>{const checked=Checked<int>;return checked(n,success);}
export function Outer<T>(v:T):T{const identity=Id<T>;return identity(v);}
export function OuterInt(n:int):int{return Outer(n);}
export function Shadow(n:int):int{const Id=[n,n+1];const index=1;const item=Id[index];return item;}
export function Deferred():int{let count=0;const identity=Id<(v:int)=>void>;const mark=identity((v:int):void=>{count+=v;});defer mark(2);mark(1);return count;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entry := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(entry, "functionvalues")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(entry, "functionvalues")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(mapped.GoSource, generated) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source map err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import("errors";"slices")
func identity[T any](v T)T{return v}
func pair[T,U any](a T,b U)(T,U){return a,b}
func first[T any](vs ...T)T{return vs[0]}
func checked[T any](v T,success bool)(T,error){if !success{var zero T;return zero,errors.New("failure")};return v,nil}
type number int
type box[T any]struct{value T}
var global=identity[int]
func Run(n int)[]int{angle:=identity[int];bracket:=identity[int];callbacks:=[]func(int)int{angle,bracket};p:=pair[int,int];left,right:=p(n,n+1);f:=first[int];values:=[]int{n+2,n+3};clone:=slices.Clone[[]int,int];copied:=clone(values);copied[0]+=10;nominal:=identity[number];b:=identity[box[int]];identityCallback:=identity[func(int)int];callback:=identityCallback(func(v int)int{return v+4});result:=make(chan int,1);go func(){result<-angle(n+5)}();return []int{global(n),callbacks[0](n),callbacks[1](n+1),left,right,f(values...),values[0],copied[0],int(nominal(number(n+6))),b(box[int]{n+7}).value,callback(n),<-result}}
func Text(v string)string{f:=identity[string];return f(v)}
func ResultValue(n int,success bool)(int,error){f:=checked[int];return f(n,success)}
func outer[T any](v T)T{f:=identity[T];return f(v)}
func OuterInt(n int)int{return outer(n)}
func Shadow(n int)int{identity:=[]int{n,n+1};index:=1;item:=identity[index];return item}
func Deferred()int{count:=0;identityCallback:=identity[func(int)];mark:=identityCallback(func(v int){count+=v});defer mark(2);mark(1);return count}
`
	comparison := `package functionvalues_test
import("reflect";"testing";g "generic-function-values.test";r "generic-function-values.test/reference")
func TestValues(t *testing.T){
 for _,n:=range []int{-10,0,17}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal("values",n,a,b)};if g.OuterInt(n)!=r.OuterInt(n)||g.Shadow(n)!=r.Shadow(n){t.Fatal("outer/shadow",n)};for _,success:=range []bool{false,true}{a,ae:=g.ResultValue(n,success);b,be:=r.ResultValue(n,success);if a!=b||(ae==nil)!=(be==nil){t.Fatal("result",a,ae,b,be)};if ae!=nil&&ae.Error()!=be.Error(){t.Fatal("error",ae,be)}}}
 for _,s:=range []string{"","hello","金木犀🍊"}{if g.Text(s)!=r.Text(s){t.Fatal("text",s)}}
 if g.Deferred()!=r.Deferred(){t.Fatal("defer")}
}
`
	runGeneratedGoDifferentialTest(t, root, "generic-function-values.test", generated, reference, comparison)
}

func TestGenericFunctionValueRejectsUnimportedTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{"models.km": `export function Identity<T>(v:T):T{return v;}`, "entry.km": `import {Identity as Id} from "./models";const f=Identity<int>;`} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "functionvalues")
	if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "undefined") {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
}
