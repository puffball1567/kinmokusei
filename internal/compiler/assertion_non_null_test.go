package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAssertionNonNullMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `import {Callback,Handler} from "./models";
import go http from "net/http";
export function Probe(input:interface{}):int{const [callback,ok]=input as? Callback;if(!ok){return -1;}return callback();}
export function Force(input:interface{}):Callback{return input as! Callback;}
export function ProbeNamed(input:interface{}):int{const [callback,ok]=input as? Handler;if(!ok){return -1;}return callback();}
export function ForceNamed(input:interface{}):Handler{return input as! Handler;}
constraint Callbacks=()=>int;
export function ProbeGeneric<T extends Callbacks>(input:interface{}):boolean{const [callback,ok]=input as? T;return ok;}
export function ForceGeneric<T extends Callbacks>(input:interface{}):T{return input as! T;}
constraint DefinedCallbacks=Handler;
export function ProbeDefinedGeneric<T extends DefinedCallbacks>(input:interface{}):boolean{const [callback,ok]=input as? T;return ok;}
export function ForceDefinedGeneric<T extends DefinedCallbacks>(input:interface{}):T{return input as! T;}
export function ProbeNullable(input:interface{}):boolean{const [callback,ok]=input as? Callback|null;return ok;}
export function ForceNullable(input:interface{}):Callback|null{return input as! Callback|null;}
export function ProbeRaw(input:interface{}):boolean{const [callback,ok]=input as? http.HandlerFunc;return ok;}
export function ForceRaw(input:interface{}):http.HandlerFunc{return input as! http.HandlerFunc;}
export function Ordered(input:interface{},visit:(step:int)=>void):int{const load=():interface{}=>{visit(1);return input;};const [callback,ok]=load() as? Callback;visit(2);if(!ok){return -1;}return callback();}
export function Finally(input:interface{},visit:()=>void):Callback{try{return input as! Callback;}finally{visit();}}
alias Once=[1]Callback;
export function FixedLen(input:interface{}):int{const n=len(copyArray[Once]([input as! Callback]));const address=&n;return *address;}
export function FixedCap(input:interface{}):int{return cap(copyArray[Once]([input as! Callback]));}
`
	path := filepath.Join(root, "entry.km")
	if err := os.WriteFile(filepath.Join(root, "models.km"), []byte("export alias Callback=()=>int;\nexport type Handler=distinct ()=>int;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "assertionnonnull")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "assertionnonnull")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import "net/http"
type Handler func()int
func Probe(input interface{})int{callback,ok:=input.(func()int);if !ok||callback==nil{return -1};return callback()}
func Force(input interface{})func()int{callback:=input.(func()int);if callback==nil{panic("nil callback")};return callback}
func ProbeNamed(input interface{})int{callback,ok:=input.(Handler);if !ok||callback==nil{return -1};return callback()}
func ForceNamed(input interface{})Handler{callback:=input.(Handler);if callback==nil{panic("nil callback")};return callback}
func ProbeGeneric[T ~func()int](input interface{})bool{callback,ok:=input.(T);return ok&&callback!=nil}
func ForceGeneric[T ~func()int](input interface{})T{callback:=input.(T);if callback==nil{panic("nil callback")};return callback}
func ProbeNullable(input interface{})bool{_,ok:=input.(func()int);return ok}
func ForceNullable(input interface{})func()int{return input.(func()int)}
func ProbeRaw(input interface{})bool{_,ok:=input.(http.HandlerFunc);return ok}
func ForceRaw(input interface{})http.HandlerFunc{return input.(http.HandlerFunc)}
func Ordered(input interface{},visit func(int))int{load:=func()interface{}{visit(1);return input};callback,ok:=load().(func()int);ok=ok&&callback!=nil;visit(2);if !ok{return -1};return callback()}
func Finally(input interface{},visit func())func()int{defer visit();return Force(input)}
func FixedLen(input interface{})int{return len([1]func()int{Force(input)})}
func FixedCap(input interface{})int{return cap([1]func()int{Force(input)})}
`
	comparison := `package assertionnonnull_test
import("fmt";"net/http";"reflect";"testing";g "assertion-non-null.test";r "assertion-non-null.test/reference")
func capture(f func())(failure any){defer func(){failure=recover()}();f();return nil}
func TestAssertions(t *testing.T){for _,input:=range []any{nil,func()int(nil),func()int{return 7},42}{
 var got,want int
 gp:=capture(func(){got=g.Probe(input)});rp:=capture(func(){want=r.Probe(input)})
 if(gp==nil)!=(rp==nil)||got!=want{t.Fatal(input,got,want,gp,rp)}
 gp=capture(func(){g.Force(input)});rp=capture(func(){r.Force(input)})
 if(gp==nil)!=(rp==nil){t.Fatal(input,gp,rp)}
 if callback,ok:=input.(func()int);ok{if callback==nil{if fmt.Sprint(gp)!="non-null type assertion failed at entry.km:4:58"{t.Fatal(gp)}}else{if got,want:=g.Force(input)(),r.Force(input)();got!=want{t.Fatal(got,want)}}}
 if got,want:=g.ProbeNullable(input),r.ProbeNullable(input);got!=want{t.Fatal(got,want)}
 gp=capture(func(){g.ForceNullable(input)});rp=capture(func(){r.ForceNullable(input)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)}
 if got,want:=g.ProbeGeneric[func()int](input),r.ProbeGeneric[func()int](input);got!=want{t.Fatal(got,want)}
 for _,pair:=range [][2]func(any)int{{g.FixedLen,r.FixedLen},{g.FixedCap,r.FixedCap}}{
  gp=capture(func(){got=pair[0](input)});rp=capture(func(){want=pair[1](input)});if(gp==nil)!=(rp==nil)||gp==nil&&got!=want{t.Fatal(got,want,gp,rp)}
 }
 gp=capture(func(){g.ForceGeneric[func()int](input)});rp=capture(func(){r.ForceGeneric[func()int](input)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)}
 var gs,rs []int;got=g.Ordered(input,func(n int){gs=append(gs,n)});want=r.Ordered(input,func(n int){rs=append(rs,n)});if got!=want||!reflect.DeepEqual(gs,rs){t.Fatal(got,want,gs,rs)}
 gs,rs=nil,nil;gp=capture(func(){g.Finally(input,func(){gs=append(gs,1)})});rp=capture(func(){r.Finally(input,func(){rs=append(rs,1)})});if(gp==nil)!=(rp==nil)||!reflect.DeepEqual(gs,rs){t.Fatal(gp,rp,gs,rs)}
}}
func TestNamed(t *testing.T){for state:=0;state<3;state++{var gi,ri any;switch state{case 0:gi,ri=g.Handler(nil),r.Handler(nil);case 1:gi,ri=g.Handler(func()int{return 8}),r.Handler(func()int{return 8});case 2:gi,ri=func()int{return 8},func()int{return 8}}
 if got,want:=g.ProbeNamed(gi),r.ProbeNamed(ri);got!=want{t.Fatal(got,want)}
 gp:=capture(func(){g.ForceNamed(gi)});rp:=capture(func(){r.ForceNamed(ri)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)}
 if got,want:=g.ProbeDefinedGeneric[g.Handler](gi),r.ProbeGeneric[r.Handler](ri);got!=want{t.Fatal(got,want)}
 gp=capture(func(){g.ForceDefinedGeneric[g.Handler](gi)});rp=capture(func(){r.ForceGeneric[r.Handler](ri)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)}
}}
func TestRaw(t *testing.T){for _,input:=range []any{nil,http.HandlerFunc(nil),http.HandlerFunc(func(http.ResponseWriter,*http.Request){}),3}{
 if got,want:=g.ProbeRaw(input),r.ProbeRaw(input);got!=want{t.Fatal(got,want)}
 gp:=capture(func(){g.ForceRaw(input)});rp:=capture(func(){r.ForceRaw(input)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)}
}}
`
	runGeneratedGoDifferentialTest(t, root, "assertion-non-null.test", generated, reference, comparison)
}
