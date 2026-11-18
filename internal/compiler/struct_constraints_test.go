package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStructConstraintsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"models.km": `import go fmt from "fmt";
export constraint Boxes<E>=Box<E>;
export constraint AliasedBoxes<E>=Wrapped<E>;
export constraint Arrays<E>=~[2]Box<E>;
export constraint Slices<E>=~Box<E>[];
export constraint Rows=Plain|Other;
export constraint ComparableRow=comparable&Plain;
export constraint Printable=Plain&fmt.Stringer;
export constraint Links=Link;
export alias Wrapped<E>=Box<E>;
export struct Box<E>{public value:E;}
export struct Plain{public n:int;public function String():bstring{return b"plain";}}
export struct Other{public n:int;}
export struct Link{public next:*Link;public n:int;}
export class User{constructor(public n:int){}}
export function keep<E,S extends Boxes<E>>(v:S):S{return v;}
export function aliasKeep<E,S extends AliasedBoxes<E>>(v:S):S{return v;}
export function first<E,S extends Arrays<E>>(v:S):Box<E>{return v[0];}
export function head<E,S extends Slices<E>>(v:S):Box<E>{return v[0];}
export function keepRow<S extends Rows>(v:S):S{return v;}
export function equal<S extends ComparableRow>(a:S,b:S):boolean{return a===b;}
export function printed<S extends Printable>(v:S):bstring{return v.String();}
export function keepLink<S extends Links>(v:S):S{return v;}
export struct Host<E>{public function keep<S extends Boxes<E>>(v:S):S{return v;}}
export struct Stored<S extends Boxes<int>>{public value:S;}
export class Store<S extends Boxes<int>>{constructor(public value:S){}public function read():S{return this.value;}}
`,
		"entry.km": `import {Box,Plain,Other,Link,User,keep as copy,aliasKeep,first,head,keepRow,equal,printed,keepLink,Host,Stored,Store} from "./models";
export function Run(n:int):int[]{
 const original=Box<int>{value:n};let result=copy(original);result.value+=1;
 const callback:(v:Box<int>)=>Box<int>=copy;
 const values:[2]Box<int>=[original,Box<int>{value:n+2}];
 const items=[original,Box<int>{value:n+3}];
 return [original.value,result.value,callback(original).value,aliasKeep(original).value,first(values).value,head(items).value,keepRow(Other{n:n+4}).n,Host<int>{}.keep(original).value,Stored<Box<int>>{value:original}.value.value,new Store<Box<int>>(original).read().value];
}
export function Equal(a:int,b:int):boolean{return equal(Plain{n:a},Plain{n:b});}
export function Printed(n:int):bstring{return printed(Plain{n:n});}
export function Cycle(n:int):int{let v=Link{next:nil,n:n};v.next=&v;const result=keepLink(v);v.n+=1;return result.next.n;}
export function Nullable(present:boolean,n:int):int{let user:User|null=null;if(present){user=new User(n);}const original=Box<User|null>{value:user};const result=copy(original);const value=result.value;if(value===null){return -1;}return value.n;}
export function Read(v:Box<User>):int{const result=copy(v);return result.value.n;}
export function Same(v:Box<User>):boolean{const result=copy(v);return result.value===v.value;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{filepath.Join(root, "entry.km")}
	generated, ds, err := EmitGo(paths, "structconstraints")
	if err != nil || len(ds) != 0 {
		t.Fatalf("emit: %v %v", err, ds)
	}
	mapped, ds, err := EmitGoWithSourceMap(paths, "structconstraints")
	if err != nil || len(ds) != 0 || !bytes.Equal(generated, mapped.GoSource) {
		t.Fatalf("mapped: %v %v", err, ds)
	}
	reference := `package reference
import "fmt"
type Box[E any]struct{Value E}
type Boxes[E any]interface{Box[E]}
type AliasedBoxes[E any]interface{Box[E]}
type Arrays[E any]interface{~[2]Box[E]}
type Slices[E any]interface{~[]Box[E]}
type Plain struct{N int}
func(Plain)String()string{return "plain"}
type Other struct{N int}
type Rows interface{Plain|Other}
type ComparableRow interface{comparable;Plain}
type Printable interface{Plain;fmt.Stringer}
type Link struct{Next *Link;N int}
type Links interface{Link}
func keep[E any,S Boxes[E]](v S)S{return v}
func aliasKeep[E any,S AliasedBoxes[E]](v S)S{return v}
func first[E any,S Arrays[E]](v S)Box[E]{return v[0]}
func head[E any,S Slices[E]](v S)Box[E]{return v[0]}
func keepRow[S Rows](v S)S{return v}
func equal[S ComparableRow](a,b S)bool{return a==b}
func printed[S Printable](v S)string{return v.String()}
func keepLink[S Links](v S)S{return v}
type Host[E any]struct{}
func hostKeep[E any,S Boxes[E]](_ Host[E],v S)S{return v}
type Stored[S Boxes[int]]struct{Value S}
type Store[S Boxes[int]]struct{value S}
func(s *Store[S])read()S{return s.value}
func Run(n int)[]int{original:=Box[int]{n};result:=keep(original);result.Value++;var callback func(Box[int])Box[int]=keep;values:=[2]Box[int]{original,{n+2}};items:=[]Box[int]{original,{n+3}};return []int{original.Value,result.Value,callback(original).Value,aliasKeep(original).Value,first(values).Value,head(items).Value,keepRow(Other{n+4}).N,hostKeep(Host[int]{},original).Value,Stored[Box[int]]{original}.Value.Value,(&Store[Box[int]]{original}).read().Value}}
func Equal(a,b int)bool{return equal(Plain{a},Plain{b})}
func Printed(n int)string{return printed(Plain{n})}
func Cycle(n int)int{v:=Link{N:n};v.Next=&v;result:=keepLink(v);v.N++;return result.Next.N}
type User struct{N int}
func Nullable(present bool,n int)int{var user *User;if present{user=&User{n}};result:=keep(Box[*User]{user});if result.Value==nil{return -1};return result.Value.N}
func Read(v Box[*User])int{result:=keep(v);if result.Value==nil{panic("non-null")};return result.Value.N}
func Same(v Box[*User])bool{return keep(v).Value==v.Value}
`
	comparison := `package structconstraints_test
import("fmt";"reflect";"strings";"testing";g "struct-constraints.test";r "struct-constraints.test/reference")
func capture(f func())(value any){defer func(){value=recover()}();f();return nil}
func TestValues(t *testing.T){
 for _,n:=range []int{-9,0,17}{if a,b:=g.Run(n),r.Run(n);!reflect.DeepEqual(a,b){t.Fatal(n,a,b)};if g.Printed(n)!=r.Printed(n)||g.Cycle(n)!=r.Cycle(n){t.Fatal("method or cycle",n)};for _,present:=range []bool{false,true}{if g.Nullable(present,n)!=r.Nullable(present,n){t.Fatal("nullable",present,n)}};for _,m:=range []int{n,n+1}{if g.Equal(n,m)!=r.Equal(n,m){t.Fatal("equality",n,m)}}}
 a,b:=g.Box[*g.User]{Value:g.NewUser(7)},r.Box[*r.User]{Value:&r.User{N:7}};if g.Read(a)!=r.Read(b)||g.Same(a)!=r.Same(b){t.Fatal("native identity")}
 got:=capture(func(){g.Read(g.Box[*g.User]{})});want:=capture(func(){r.Read(r.Box[*r.User]{})});if got==nil||want==nil||!strings.Contains(fmt.Sprint(got),"entry.km:"){t.Fatal("Go-owned invalid reference",got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "struct-constraints.test", generated, reference, comparison)
}
