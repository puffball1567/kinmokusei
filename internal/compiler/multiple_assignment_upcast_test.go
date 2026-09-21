package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultipleAssignmentUpcastsMatchGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "entry.km")
	input := `class Base{public virtual function read():int{return 1;}}
class Child extends Base{public override function read():int{return 2;}}
let calls=0;
function pair():(Child,int){calls++;return new Child(),calls;}
function Run():int{calls=0;let __assignmentValue0:Base=new Base();let n=0;[__assignmentValue0,n]=pair();const result=__assignmentValue0.read()*100+n;[__assignmentValue0,_]=pair();return result*10+calls;}
function maybe():(Child|null,int){return null,7;}
function Nullable():boolean{let value:Base|null=new Base();let n=0;[value,n]=maybe();return value==null&&n==7;}
function Loop():int{calls=0;let value:Base=new Base();let n=0;for(;n<3;[value,n]=pair()){}return value.read()*10+calls;}
function children():(Child,Child){return new Child(),new Child();}
function Repeated():int{let value:Base=new Base();[value,value]=children();return value.read();}
function Lookup():int{const m=makeMap<string,Child>();m["key"]=new Child();let value:Base=new Base();let found=false;[value,found]=m["key"];if(found){return value.read();}return 0;}
function Receive():int{const ch=goChannel<Child>(1);ch <- new Child();let value:Base=new Base();let found=false;[value,found]=<-ch;if(found){return value.read();}return 0;}
class Box<T>{constructor(public value:T){}}
class DerivedBox<T> extends Box<T>{constructor(value:T){super(value);}}
function boxes():(DerivedBox<int>,int){return new DerivedBox<int>(7),3;}
function genericBoxes<T>(value:T):(DerivedBox<T>,int){return new DerivedBox<T>(value),1;}
function assigned<T>(value:T):T{let box:Box<T>=new Box<T>(value);let n=0;[box,n]=genericBoxes(value);return box.value;}
function Generic():int{let box:Box<int>=new Box<int>(0);let n=0;[box,n]=boxes();return box.value+n+assigned(5);}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "assignments")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type base interface{read()int}
type parent struct{}
func(*parent)read()int{return 1}
type child struct{}
func(*child)read()int{return 2}
var calls int
func pair()(*child,int){calls++;return &child{},calls}
func Run()int{calls=0;var value base=&parent{};n:=0;v,x:=pair();value,n=v,x;result:=value.read()*100+n;v,_=pair();value=v;return result*10+calls}
func Nullable()bool{var value base=&parent{};var v *child; n:=7;if v==nil{value=nil}else{value=v};return value==nil&&n==7}
func Loop()int{calls=0;var value base=&parent{};n:=0;for n<3{v,x:=pair();value,n=v,x};return value.read()*10+calls}
func Repeated()int{var value base=&parent{};a,b:=&child{},&child{};value,value=a,b;return value.read()}
func Lookup()int{m:=map[string]*child{"key":{}};v,ok:=m["key"];var value base=v;if ok{return value.read()};return 0}
func Receive()int{ch:=make(chan *child,1);ch<-&child{};v,ok:=<-ch;var value base=v;if ok{return value.read()};return 0}
type box[T any]struct{value T}
type derivedBox[T any]struct{box[T]}
func assigned[T any](x T)T{v:=&derivedBox[T]{box[T]{x}};value:=&v.box;return value.value}
func Generic()int{value:=&box[int]{};v,n:=&derivedBox[int]{box[int]{7}},3;value=&v.box;return value.value+n+assigned(5)}
`
	comparison := `package assignments_test
import("testing";g "assignment-upcast.test";r "assignment-upcast.test/reference")
func TestAssignments(t *testing.T){for _,tc:=range []struct{name string;got,want func()int}{{"ordinary",g.Run,r.Run},{"loop",g.Loop,r.Loop},{"repeated",g.Repeated,r.Repeated},{"generic",g.Generic,r.Generic},{"lookup",g.Lookup,r.Lookup},{"receive",g.Receive,r.Receive}}{if got,want:=tc.got(),tc.want();got!=want{t.Fatalf("%s: %d != %d",tc.name,got,want)}};if g.Nullable()!=r.Nullable(){t.Fatal("nil upcast")}}
`
	runGeneratedGoDifferentialTest(t, root, "assignment-upcast.test", generated, reference, comparison)
}
