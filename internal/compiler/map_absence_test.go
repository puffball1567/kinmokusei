package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMapAbsenceMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := `export interface Reader{function read():int;}
export class User implements Reader{constructor(public n:int){}public function read():int{return this.n;}}
export struct Row{public user:User;}
type Table<T>=distinct Map<int,T>;
constraint Tables<E>=~Map<int,E>;
export function GenericRead<T>(values:Map<int,T>,key:int,fallback:T):T{const [value,ok]=values[key];if(!ok){return fallback;}return value;}
function constrained<E,T extends Tables<E>>(values:T,key:int,fallback:E):E{const [value,ok]=values[key];if(ok){return value;}return fallback;}
export function Run(present:boolean):int[]{
 const values=makeMap<int,User>();
 if(present){values[0]=new User(7);}
 const value=values[0];let direct=0;if(value!==null){direct=value.n;}
 const chosen=values[0]??new User(9);
 const [checked,ok]=values[0];let checkedN=0;if(ok){delete(values,0);checkedN=checked.n;}
 if(present){values[0]=new User(7);}
 const reader:Reader|null=values[0];let interfaceN=0;if(reader!==null){interfaceN=reader.read();}
 const rows=makeMap<int,Row>();if(present){rows[0]=Row{user:new User(4)};}
 const [row,found]=rows[0];let rowN=0;if(found){rowN=row.user.n;}
 const generic=constrained<User,Map<int,User>>(values,0,new User(6));
 return [direct,chosen.n,checkedN,interfaceN,rowN,generic.n];
}
export function NullablePayload(present:boolean):boolean[]{
 const values=makeMap<int,User|null>();if(present){values[0]=null;}
 const [value,found]=values[0];return [value===null,found];
}
export function PairJoin(values:Map<int,User>,pick:boolean):int{
 let [value,ok]=values[0];if(pick){[value,ok]=values[1];}if(!ok){return 0;}return value.n;
}
export function PointerValue(values:Map<int,*int>):int{
 const [value,ok]=values[0];if(!ok){return 0;}if(value===nil){return 1;}return *value;
}
export function InterfaceValue(values:Map<int,error>):error|null{return values[0];}
export function Assert(values:Map<int,User>):int{return values[0]!.n;}
export function Ordered(present:boolean):int[]{
 let calls=0;const values=makeMap<int,User>();if(present){values[0]=new User(3);}
 const object=():Map<int,User>=>{calls=calls*10+1;return values;};
 const key=():int=>{calls=calls*10+2;return 0;};
 const fallback=():User=>{calls=calls*10+3;return new User(8);};
 const value=object()[key()]??fallback();return [value.n,calls];
}
`
	entry := filepath.Join(root, "entry.km")
	if err := os.WriteFile(entry, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "mapabsence")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{entry}, "mapabsence")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
type Reader interface{Read()int}
type User struct{N int}
func(u *User)Read()int{return u.N}
type Row struct{User *User}
func GenericRead[T any](values map[int]T,key int,fallback T)T{v,ok:=values[key];if !ok{return fallback};return v}
func Run(p bool)[]int{values:=map[int]*User{};if p{values[0]=&User{7}};v:=values[0];direct:=0;if v!=nil{direct=v.N};chosen:=v;if chosen==nil{chosen=&User{9}};checked,ok:=values[0];checkedN:=0;if ok{delete(values,0);checkedN=checked.N};if p{values[0]=&User{7}};v=values[0];var reader Reader;if v!=nil{reader=v};interfaceN:=0;if reader!=nil{interfaceN=reader.Read()};rows:=map[int]Row{};if p{rows[0]=Row{&User{4}}};row,found:=rows[0];rowN:=0;if found{rowN=row.User.N};generic:=GenericRead(values,0,&User{6});return []int{direct,chosen.N,checkedN,interfaceN,rowN,generic.N}}
func NullablePayload(p bool)[]bool{values:=map[int]*User{};if p{values[0]=nil};v,ok:=values[0];return []bool{v==nil,ok}}
func PairJoin(values map[int]*User,p bool)int{v,ok:=values[0];if p{v,ok=values[1]};if !ok{return 0};return v.N}
func PointerValue(values map[int]*int)int{v,ok:=values[0];if !ok{return 0};if v==nil{return 1};return *v}
func InterfaceValue(values map[int]error)error{return values[0]}
func Assert(values map[int]*User)int{v:=values[0];if v==nil{panic("missing")};return v.N}
func Ordered(p bool)[]int{calls:=0;values:=map[int]*User{};if p{values[0]=&User{3}};object:=func()map[int]*User{calls=calls*10+1;return values};key:=func()int{calls=calls*10+2;return 0};fallback:=func()*User{calls=calls*10+3;return &User{8}};m:=object();k:=key();v:=m[k];if v==nil{v=fallback()};return []int{v.N,calls}}
`
	comparison := `package mapabsence_test
import("errors";"fmt";"reflect";"strings";"testing";g "map-absence.test";r "map-absence.test/reference")
func capture(f func())(failure any){defer func(){failure=recover()}();f();return nil}
type typedError struct{}
func(*typedError)Error()string{return "typed nil"}
func TestMaps(t *testing.T){
 for _,p:=range []bool{false,true}{if got,want:=g.Run(p),r.Run(p);!reflect.DeepEqual(got,want){t.Fatal(p,got,want)};if got,want:=g.NullablePayload(p),r.NullablePayload(p);!reflect.DeepEqual(got,want){t.Fatal(p,got,want)};if got,want:=g.Ordered(p),r.Ordered(p);!reflect.DeepEqual(got,want){t.Fatal(p,got,want)}}
 for _,key:=range []int{0,1,2}{if got,want:=g.GenericRead(map[int]int{0:7},key,9),r.GenericRead(map[int]int{0:7},key,9);got!=want{t.Fatal(got,want)}}
 for _,p:=range []bool{false,true}{for _,q:=range []bool{false,true}{gm:=map[int]*g.User{};rm:=map[int]*r.User{};if p{gm[0]=g.NewUser(3);rm[0]=&r.User{N:3}};if q{gm[1]=g.NewUser(7);rm[1]=&r.User{N:7}};for _,pick:=range []bool{false,true}{if got,want:=g.PairJoin(gm,pick),r.PairJoin(rm,pick);got!=want{t.Fatal(got,want)}};gp:=capture(func(){g.Assert(gm)});rp:=capture(func(){r.Assert(rm)});if(gp==nil)!=(rp==nil){t.Fatal(gp,rp)};if gp!=nil&&!strings.Contains(fmt.Sprint(gp),"entry.km:"){t.Fatal(gp)}}}
 n:=8;for _,values:=range []map[int]*int{nil,{}, {0:nil},{0:&n}}{if got,want:=g.PointerValue(values),r.PointerValue(values);got!=want{t.Fatal(got,want)}}
 var typedNil *typedError
 for _,values:=range []map[int]error{nil,{}, {0:nil},{0:typedNil},{0:errors.New("error")}}{got,want:=g.InterfaceValue(values),r.InterfaceValue(values);if(got==nil)!=(want==nil){t.Fatal(got,want)};if got!=nil&&got.Error()!=want.Error(){t.Fatal(got,want)}}
}
`
	runGeneratedGoDifferentialTest(t, root, "map-absence.test", generated, reference, comparison)
}
