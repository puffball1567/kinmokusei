package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTypedForInitializerPreservesStorage(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function takes(v:int32):int32{return v;}function main():void{let total:int32=0;for(let i:int32=0;i<3;i++){total+=takes(i);}const _=total;}`,
		`function takes(v:uint8):uint8{return v;}function main():void{for(let i:uint8=0;i<3;i++){takes(i);}}`,
		`function takes(v:float32):float32{return v;}function main():void{for(let i:float32=0;i<3;i++){takes(i);}}`,
		`import go time from "time";function main():void{for(let i:time.Duration=0;i<3;i++){i.Seconds();}}`,
		`type Count=distinct int32;function takes(v:Count):Count{return v;}function main():void{for(let i:Count=0;i<3;i++){takes(i);}}`,
		`constraint Int32=~int32;function takes<T>(v:T):T{return v;}function use<T extends Int32>():void{for(let i:T=0;i<3;i++){takes<T>(i);}}`,
		`function main():void{for(let i:*int=nil;i===nil;){break;}}`,
		`function main():void{for(let i:int[]=nil;i===nil;){break;}}`,
		`function main():void{for(let i:interface{read():int;}=nil;i===nil;){break;}}`,
		`function main():void{for(let i:(v:int)=>int=nil;i===nil;){break;}}`,
		`struct S{public function read():int{return 1;}}function main():void{for(let i:interface{read():int;}=S{};i.read()<2;){break;}}`,
		`class Base{}class Child extends Base{}function main():void{for(let i:Base=new Child();false;){const _=i;}}`,
		`function main():void{for(let i:{right:int,left:int}={left:1,right:2};i.left<2;){break;}}`,
	} {
		t.Run(input, func(t *testing.T) {
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				t.Fatalf("accepted=%v err=%v", accepted, err)
			}
		})
	}
}

func TestTypedForInitializerMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `import go time from "time";
type Count=distinct int32;
constraint Int32=~int32;
alias Callback=(v:int32)=>int32;
alias Pointer=*int32;
function next():int32{return 1;}
function use<T extends Int32>():T{let total:T=0;for(let i:T=0;i<3;i++){total+=i;}return total;}
export function Run():int[]{
 let total:int32=0;let calls=0;
 const mark=():int32=>{calls++;return next();};
 for(let i:int32=mark();i<4;i++){total+=i;}
 let duration:time.Duration=0;for(let i:time.Duration=0;i<3;i++){duration+=i;}
 let narrow:Count=0;for(let i:Count=0;i<3;i++){narrow+=i;}
 let floats:float32=0;for(let i:float32=0;i<3;i++){floats+=i;}
 return [int(total),calls,int(duration),int(narrow),int(floats),int(use<int32>())];
}
export function Captures():int[]{
 let callbacks:Callback[]=[];let pointers:Pointer[]=[];
 for(let i:int32=0;i<3;i++){callbacks=append(callbacks,(v:int32):int32=>i+v);pointers=append(pointers,&i);}
 return [int(callbacks[0](0)),int(callbacks[1](0)),int(callbacks[2](0)),int(*pointers[0]),int(*pointers[1]),int(*pointers[2])];
}
export function Labels():int[]{
 let total:int32=0;let rounds=0;
 outer:for(let i:int32=0;i<5;i++){if(i==1){continue outer;}if(i==3){break outer;}total+=i;}
 restart:for(let i:int32=0;i<2;i++){rounds++;if(rounds==1){goto restart;}}
 let i:int32=10;for(let i:int32=0;i<2;i++){total+=i;}
 return [int(total),rounds,int(i)];
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "typedfor")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "typedfor")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(mapped.GoSource, generated) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source map err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
import "time"
type count int32
func next()int32{return 1}
func use[T ~int32]()T{var total T;for i:=T(0);i<3;i++{total+=i};return total}
func Run()[]int{var total int32;calls:=0;mark:=func()int32{calls++;return next()};for i:=mark();i<4;i++{total+=i};var duration time.Duration;for i:=time.Duration(0);i<3;i++{duration+=i};var narrow count;for i:=count(0);i<3;i++{narrow+=i};var floats float32;for i:=float32(0);i<3;i++{floats+=i};return []int{int(total),calls,int(duration),int(narrow),int(floats),int(use[int32]())}}
func Captures()[]int{callbacks:=[]func(int32)int32{};pointers:=[]*int32{};for i:=int32(0);i<3;i++{callbacks=append(callbacks,func(v int32)int32{return i+v});pointers=append(pointers,&i)};return []int{int(callbacks[0](0)),int(callbacks[1](0)),int(callbacks[2](0)),int(*pointers[0]),int(*pointers[1]),int(*pointers[2])}}
func Labels()[]int{var total int32;rounds:=0;outer:for i:=int32(0);i<5;i++{if i==1{continue outer};if i==3{break outer};total+=i};restart:for i:=int32(0);i<2;i++{rounds++;if rounds==1{goto restart}};i:=int32(10);for i:=int32(0);i<2;i++{total+=i};return []int{int(total),rounds,int(i)}}
`
	comparison := `package typedfor_test
import("reflect";"testing";g "typed-for.test";r "typed-for.test/reference")
func TestContracts(t *testing.T){
 if got,want:=g.Run(),r.Run();!reflect.DeepEqual(got,want){t.Fatal("types/order",got,want)}
 if got,want:=g.Captures(),r.Captures();!reflect.DeepEqual(got,want){t.Fatal("iteration identity",got,want)}
 if got,want:=g.Labels(),r.Labels();!reflect.DeepEqual(got,want){t.Fatal("labels/scope",got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "typed-for.test", generated, reference, comparison)
}
