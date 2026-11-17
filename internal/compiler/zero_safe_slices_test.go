package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInitializedSlicesMatchCheckedGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "slices.km")
	input := `export class User{constructor(public value:int){}}
export struct Row<T>{public value:T;}
alias Users=User[];
alias Pointer=*int;export type Pointers=distinct Pointer[];
alias MaybePointer=*int|null;export type MaybePointers=distinct MaybePointer[];
constraint MixedPointers=MaybePointers|Pointers;
alias Maybe=User|null;
constraint Slice<E>=~E[];
function users():Users{let xs=make<Users>(0,4);xs=append(xs,new User(1),new User(2));return xs;}
export function Bounds(low:int,high:uint64,max:uint64,full:boolean):int{const xs=users();if(full){return len(xs[low:high:max]);}return len(xs[low:high]);}
export function Omitted(low:int):int{return len(users()[low:]);}
export function Wide(low:uint64,high:uint64,max:uint64):int{return len(users()[low:high:max]);}
export function Shift(n:uint):int{return len(users()[0:1<<n]);}
export function Constants():int{return len(users()[0.0:1.0:3.0]);}
export function Aliases():int[]{const xs=users();const view=xs[0:1:3];view[0]=new User(9);return [xs[0].value,len(view),cap(view)];}
export function Empty():boolean{const xs:User[]=nil;return xs[:]===nil;}
export function Nullable():int{const xs=makeSlice<Maybe>(1,3);clear(xs);const extended=xs[:3];if(extended[2]===null){return len(extended);}return -1;}
export function Records(n:int):int{let xs=makeSlice<Row<User>>(0,3);xs=append(xs,Row<User>{value:new User(1)});return len(xs[:n]);}
export function RecordZero():boolean{const xs=makeSlice<Row<Maybe>>(2);clear(xs);return xs[0].value===null;}
export function Named(n:int):int{let seed=3;let xs=make<Pointers>(0,3);xs=append(xs,&seed);return len(xs[:n]);}
export function Fixed():int{const xs:[2]User=[new User(1),new User(2)];return xs[:][1].value;}
export function Generic<E,T extends Slice<E>>(xs:T,n:int):T{return xs[:n];}
export function Mixed<T extends MixedPointers>(xs:T,n:int):T{return xs[:n];}
export function Ordered(high:int,log:*int):int{const receiver=():Users=>{*log=*log*10+1;return users();};const bound=(digit:int,value:int):int=>{*log=*log*10+digit;return value;};return len(receiver()[bound(2,0):bound(3,high):bound(4,4)]);}
export function GuardInLen(n:int):int{const xs=makeSlice<User>(0,3);return len(copyArray<[0]User>(xs[:n]));}
export function GuardInCap(n:int):int{const xs=makeSlice<User>(0,3);return cap(viewArray<[0]User>(xs[:n]));}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "initialized")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit err=%v diagnostics=%v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "initialized")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("source maps err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type user struct{value int}
type users []*user
func values()users{xs:=make(users,0,4);return append(xs,&user{1},&user{2})}
func checked[S ~[]E,E any](xs S,low,high,max uint64,full bool)S{if low>high||high>max||max>uint64(cap(xs)){panic("bounds")};if high>uint64(len(xs)){panic("uninitialized")};if full{return xs[int(low):int(high):int(max)]};return xs[int(low):int(high)]}
func Bounds(low int,high,max uint64,full bool)int{xs:=values();if !full{max=uint64(cap(xs))};return len(checked(xs,uint64(low),high,max,full))}
func Omitted(low int)int{xs:=values();return len(checked(xs,uint64(low),uint64(len(xs)),uint64(cap(xs)),false))}
func Wide(low,high,max uint64)int{return len(checked(values(),low,high,max,true))}
func Shift(n uint)int{var high int=1<<n;return Bounds(0,uint64(high),4,false)}
func Constants()int{return Bounds(0,1,3,true)}
func Aliases()[]int{xs:=values();view:=checked(xs,0,1,3,true);view[0]=&user{9};return []int{xs[0].value,len(view),cap(view)}}
func Empty()bool{var xs users;return checked(xs,0,0,0,false)==nil}
func Nullable()int{xs:=make([]*user,1,3);clear(xs);return len(xs[:3])}
type row[T any]struct{value T}
func Records(n int)int{xs:=make([]row[*user],0,3);xs=append(xs,row[*user]{&user{1}});return len(checked(xs,0,uint64(n),3,false))}
func RecordZero()bool{xs:=make([]row[*user],2);clear(xs);return xs[0].value==nil}
func Named(n int)int{seed:=3;xs:=make([]*int,0,3);xs=append(xs,&seed);return len(checked(xs,0,uint64(n),3,false))}
func Fixed()int{xs:=[2]*user{{1},{2}};return xs[:][1].value}
func Generic[E any,T ~[]E](xs T,n int)T{return checked(xs,0,uint64(n),uint64(cap(xs)),false)}
func Ordered(high int,log *int)int{*log=*log*10+1;xs:=values();*log=*log*10+2;low:=0;*log=*log*10+3;h:=high;*log=*log*10+4;max:=4;return len(checked(xs,uint64(low),uint64(h),uint64(max),true))}
func GuardInLen(n int)int{xs:=make(users,0,3);return len([0]*user(checked(xs,0,uint64(n),3,false)))}
func GuardInCap(n int)int{xs:=make(users,0,3);return cap((*[0]*user)(checked(xs,0,uint64(n),3,false)))}
`
	comparison := `package initialized_test
import("fmt";"reflect";"runtime";"strings";"testing";g "initialized-slices.test";r "initialized-slices.test/reference")
func capture(f func()int)(value int,panicked bool,message string){defer func(){if p:=recover();p!=nil{panicked=true;message=fmt.Sprint(p)}}();value=f();return}
func compare(t *testing.T,gf,rf func()int){t.Helper();gv,gp,_:=capture(gf);rv,rp,_:=capture(rf);if gv!=rv||gp!=rp{t.Fatalf("generated=%d,%v reference=%d,%v",gv,gp,rv,rp)}}
func TestBounds(t *testing.T){for _,lo:=range []int{-1,0,1,2,3}{for _,hi:=range []uint64{0,1,2,3,4,^uint64(0)}{for _,max:=range []uint64{0,1,2,3,4,^uint64(0)}{for _,full:=range []bool{false,true}{compare(t,func()int{return g.Bounds(lo,hi,max,full)},func()int{return r.Bounds(lo,hi,max,full)})}}};compare(t,func()int{return g.Omitted(lo)},func()int{return r.Omitted(lo)})};for _,n:=range []uint{0,1,2,8,31,32,63,64}{compare(t,func()int{return g.Shift(n)},func()int{return r.Shift(n)})};for _,n:=range []uint64{0,1,2,^uint64(0)}{compare(t,func()int{return g.Wide(n,2,4)},func()int{return r.Wide(n,2,4)})}}
func TestStorage(t *testing.T){if !reflect.DeepEqual(g.Aliases(),r.Aliases())||g.Empty()!=r.Empty()||g.Nullable()!=r.Nullable()||g.RecordZero()!=r.RecordZero()||g.Fixed()!=r.Fixed()||g.Constants()!=r.Constants(){t.Fatal("storage")};for _,n:=range []int{-1,0,1,2,3}{compare(t,func()int{return g.Records(n)},func()int{return r.Records(n)});compare(t,func()int{return g.Named(n)},func()int{return r.Named(n)});xs:=make([]int,2,4);compare(t,func()int{return len(g.Generic(xs,n))},func()int{return len(r.Generic(xs,n))})}}
func runtimeBounds(f func()int)(ok bool){defer func(){_,ok=recover().(runtime.Error)}();f();return}
func TestNativeBoundsError(t *testing.T){if !runtimeBounds(func()int{return g.Bounds(-1,1,4,false)})||!runtimeBounds(func()int{return g.Bounds(0,5,4,false)}){t.Fatal("native Go bounds error changed")}}
func TestMixed(t *testing.T){xs:=make(g.Pointers,1,3);for _,n:=range []int{0,1,2,3}{_,panicked,_:=capture(func()int{return len(g.Mixed(xs,n))});if panicked!=(n>len(xs)){t.Fatal("mixed nullability",n,panicked)}}}
func TestOrder(t *testing.T){for _,n:=range []int{1,3,9}{gl,rl:=0,0;compare(t,func()int{return g.Ordered(n,&gl)},func()int{return r.Ordered(n,&rl)});if gl!=1234||gl!=rl{t.Fatal("order",gl,rl)}}}
func TestChecksInsideArrayLength(t *testing.T){for _,pair:=range [][2]func(int)int{{g.GuardInLen,r.GuardInLen},{g.GuardInCap,r.GuardInCap}}{compare(t,func()int{return pair[0](0)},func()int{return pair[1](0)});_,panic,message:=capture(func()int{return pair[0](1)});if !panic||!strings.Contains(message,"uninitialized elements at slices.km:"){t.Fatal("missing source failure",panic,message)}}}
`
	runGeneratedGoDifferentialTest(t, root, "initialized-slices.test", generated, reference, comparison)
}
