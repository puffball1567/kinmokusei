package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPresenceGuardsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `export const SUCCESS=true;
export const FAILURE=false;
export class Item{constructor(public n:int){}}
export class Base{}
export class Child extends Base{constructor(public n:int){super();}}
`,
		"entry.km": `import {SUCCESS as YES,FAILURE as NO,Item,Base,Child} from "./models";
function input(present:boolean,n:int):Map<int,Item>{const m=makeMap<int,Item>();if(present){m[0]=new Item(n);}return m;}
function stream(present:boolean,n:int):GoChannel<Item>{const ch=goChannel<Item>(1);if(present){ch<-new Item(n);}closeGoChannel(ch);return ch;}
export function Compare(present:boolean,n:int):int{const [value,ok]=input(present,n)[0];if((ok===YES)!==NO){return value.n;}return 0;}
export function Switch(present:boolean,n:int):int{const [value,ok]=input(present,n)[0];switch(ok){case NO{return 0;}}return value.n;}
export function DefaultFirst(present:boolean,n:int):int{const [value,ok]=input(present,n)[0];switch(!ok){default{return value.n;}case YES{return 0;}}}
export function Conditions(present:boolean,n:int):int{const [value,ok]=input(present,n)[0];switch(YES){case !ok{return 0;}default{return value.n;}}}
export function LaterLabel(present:boolean,n:int):int{const [value,ok]=input(present,n)[0];switch(ok){case NO{return 0;}case value.n>0{return value.n;}default{return value.n;}}}
export function Fallthrough(present:boolean,n:int):int{const [value,ok]=input(present,n)[0];switch(ok){case NO{return 0;}case YES{fallthrough;}default{return value.n;}}}
export function Receive(present:boolean,n:int):int{const [value,open]=<-stream(present,n);switch(!open){case YES{return 0;}default{return value.n;}}}
export function Selected(present:boolean,n:int):int{select{case const [value,open]=<-stream(present,n){switch(open===NO){case NO{return value.n;}default{return 0;}}}}}
export function Assertion(present:boolean,n:int):int{let base:Base=new Base();if(present){base=new Child(n);}const [value,ok]=base as? Child;switch(true){case ok===YES{return value.n;}default{return 0;}}}
export function Generic<T>(m:Map<int,T>,fallback:T):T{const [value,ok]=m[0];switch(ok){case NO{return fallback;}default{return value;}}}
export function GenericClass(present:boolean,n:int):int{return Generic(input(present,n),new Item(9)).n;}
export function Trace(present:boolean,n:int):int[]{let trace=0;const mark=(step:int):boolean=>{trace=trace*10+step;return false;};const [value,ok]=input(present,n)[0];switch(ok){case mark(1){return [0,trace];}case YES{return [value.n,trace];}default{return [0,trace];}}}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, diagnostics, err := EmitGo(paths, "presenceguards")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap(paths, "presenceguards")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
const YES=true
const NO=false
type Item struct{N int}
type Base struct{}
type Child struct{N int}
func input(p bool,n int)map[int]*Item{m:=map[int]*Item{};if p{m[0]=&Item{n}};return m}
func stream(p bool,n int)chan *Item{ch:=make(chan *Item,1);if p{ch<-&Item{n}};close(ch);return ch}
func Compare(p bool,n int)int{v,ok:=input(p,n)[0];if(ok==YES)!=NO{return v.N};return 0}
func Switch(p bool,n int)int{v,ok:=input(p,n)[0];switch ok{case NO:return 0};return v.N}
func DefaultFirst(p bool,n int)int{v,ok:=input(p,n)[0];switch !ok{default:return v.N;case YES:return 0}}
func Conditions(p bool,n int)int{v,ok:=input(p,n)[0];switch YES{case !ok:return 0;default:return v.N}}
func LaterLabel(p bool,n int)int{v,ok:=input(p,n)[0];switch ok{case NO:return 0;case v.N>0:return v.N;default:return v.N}}
func Fallthrough(p bool,n int)int{v,ok:=input(p,n)[0];switch ok{case NO:return 0;case YES:fallthrough;default:return v.N}}
func Receive(p bool,n int)int{v,open:=<-stream(p,n);switch !open{case YES:return 0;default:return v.N}}
func Selected(p bool,n int)int{select{case v,open:=<-stream(p,n):switch open==NO{case NO:return v.N;default:return 0}}}
func Assertion(p bool,n int)int{var base interface{}=&Base{};if p{base=&Child{n}};v,ok:=base.(*Child);switch true{case ok==YES:return v.N;default:return 0}}
func Generic[T any](m map[int]T,fallback T)T{v,ok:=m[0];switch ok{case NO:return fallback;default:return v}}
func GenericClass(p bool,n int)int{return Generic(input(p,n),&Item{9}).N}
func Trace(p bool,n int)[]int{trace:=0;mark:=func(step int)bool{trace=trace*10+step;return false};v,ok:=input(p,n)[0];switch ok{case mark(1):return []int{0,trace};case YES:return []int{v.N,trace};default:return []int{0,trace}}}
`
	comparison := `package presenceguards_test
import("reflect";"testing";g "presence-guards.test";r "presence-guards.test/reference")
func TestGuards(t *testing.T){for _,p:=range []bool{false,true}{for _,n:=range []int{-7,0,7}{
 for _,pair:=range [][2]func(bool,int)int{{g.Compare,r.Compare},{g.Switch,r.Switch},{g.DefaultFirst,r.DefaultFirst},{g.Conditions,r.Conditions},{g.LaterLabel,r.LaterLabel},{g.Fallthrough,r.Fallthrough},{g.Receive,r.Receive},{g.Selected,r.Selected},{g.Assertion,r.Assertion},{g.GenericClass,r.GenericClass}}{if got,want:=pair[0](p,n),pair[1](p,n);got!=want{t.Fatal(p,n,got,want)}}
 if got,want:=g.Trace(p,n),r.Trace(p,n);!reflect.DeepEqual(got,want){t.Fatal(p,n,got,want)}
 m:=map[int]int{};if p{m[0]=n};if got,want:=g.Generic(m,9),r.Generic(m,9);got!=want{t.Fatal(p,n,got,want)}
}}}
`
	runGeneratedGoDifferentialTest(t, root, "presence-guards.test", generated, reference, comparison)
}
