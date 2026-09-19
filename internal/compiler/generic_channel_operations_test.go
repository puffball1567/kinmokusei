package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericChannelOperationsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module generic-channel-operations.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Send[E any] interface{~chan E|~chan<- E}
type Receive[E any] interface{~chan E|~<-chan E}
type Mixed interface{~chan int|~chan string}
type Empty interface{~chan int;~chan string}
`,
		"bounds.km": `export constraint Send<E>=~GoChannel<E>|~GoSendChannel<E>;
export constraint Receive<E>=~GoChannel<E>|~GoReceiveChannel<E>;
export function put<E,C extends Send<E>>(ch:C,value:E):void{ch<-value;}
export function get<E,C extends Receive<E>>(ch:C):E{return <-ch;}`,
		"entry.km": `import {Send,Receive,put,get} from "./bounds";
import go contracts from "generic-channel-operations.test/contracts";
type Named=distinct GoChannel<int>;
function importedPut<E,C extends contracts.Send<E>>(ch:C,value:E):void{ch<-value;}
function importedGet<E,C extends contracts.Receive<E>>(ch:C):E{return <-ch;}
function offer<C extends Send<int>>(ch:C):boolean{select{case ch<-23{return true;}default{return false;}}}
function poll<C extends Receive<int>>(ch:C):int{select{case const [value,open]=<-ch{if(open){return value;}return -1;}default{return -2;}}}
function assigned<C extends Receive<int>>(ch:C):int{let value=0;let open=false;select{case [value,open]=<-ch{if(open){return value;}}default{return -2;}}return -1;}
function evaluated<C extends Send<int>>(ch:C,count:*int):void{const channel=():C=>{*count+=1;return ch;};const value=():int=>{*count+=10;return 31;};select{case channel()<-value(){}default{}}}
export function Buffered():int[]{const ch=Named(goChannel<int>(2));put(ch,7);const send:GoSendChannel<int>=ch;importedPut(send,9);const receive:GoReceiveChannel<int>=ch;const a=get<int,GoReceiveChannel<int>>(receive);const b=importedGet<int,Named>(ch);closeGoChannel(ch);const zero=get<int,Named>(ch);return [a,b,zero];}
export function Selection():int[]{const ch=goChannel<int>(1);let calls=0;evaluated(ch,&calls);const a=poll(ch);offer(ch);const b=assigned(ch);closeGoChannel(ch);const closed=poll(ch);const absent:GoChannel<int>=nil;const idle=poll(absent);let flag=0;if(offer(absent)){flag=1;}evaluated(absent,&calls);return [a,b,closed,idle,flag,calls];}
export function Concurrent():int{const ch=goChannel<int>();go put(ch,42);return get<int,GoChannel<int>>(ch);}
class Item{constructor(public value:int){}}
class Child extends Item{constructor(){super(29);}}
constraint Items=~GoChannel<Item>|~GoSendChannel<Item>;
function offerChild<C extends Items>(ch:C):void{select{case ch<-new Child(){}default{}}}
function putChild<C extends Items>(ch:C):void{ch<-new Child();}
export function Upcast():int[]{const ch=goChannel<Item>(3);offerChild(ch);putChild(ch);select{case ch<-new Child(){}default{}}const a=<-ch;const b=<-ch;const c=<-ch;return [a.value,b.value,c.value];}
constraint Objects=~GoChannel<Item|null>|~GoReceiveChannel<Item|null>;
function objectValue<C extends Objects>(ch:C):int{const [value,open]=<-ch;if(value!==null){return value.value;}return 0;}
constraint Both<E>=contracts.Send<E>&contracts.Receive<E>;
class Relay<E>{public function roundTrip<C extends Both<E>>(ch:C,value:E):E{ch<-value;return <-ch;}}
export function OOP():int[]{const ch=goChannel<Item|null>(1);const item=new Item(17);importedPut<Item|null,GoChannel<Item|null>>(ch,item);const a=objectValue(ch);const value=new Relay<Item|null>().roundTrip<GoChannel<Item|null>>(ch,item);let identity=0;if(value===item){identity=1;}closeGoChannel(ch);return [a,identity,objectValue(ch)];}
export function ClosedSend():void{const ch=goChannel<int>();closeGoChannel(ch);put(ch,1);}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericchannels")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type send[E any] interface{~chan E|~chan<- E}
type receive[E any] interface{~chan E|~<-chan E}
type both[E any] interface{send[E];receive[E]}
type named chan int
func put[E any,C send[E]](ch C,value E){ch<-value}
func get[E any,C receive[E]](ch C)E{return <-ch}
func offer[C send[int]](ch C)bool{select{case ch<-23:return true;default:return false}}
func poll[C receive[int]](ch C)int{select{case value,open:=<-ch:if open{return value};return -1;default:return -2}}
func assigned[C receive[int]](ch C)int{value,open:=0,false;select{case value,open=<-ch:if open{return value};default:return -2};return -1}
func evaluated[C send[int]](ch C,count *int){channel:=func()C{*count+=1;return ch};value:=func()int{*count+=10;return 31};select{case channel()<-value():default:}}
func Buffered()[]int{ch:=make(named,2);put(ch,7);var output chan<- int=ch;put(output,9);var input <-chan int=ch;a:=get[int](input);b:=get[int](ch);close(ch);return []int{a,b,get[int](ch)}}
func Selection()[]int{ch:=make(chan int,1);calls:=0;evaluated(ch,&calls);a:=poll(ch);offer(ch);b:=assigned(ch);close(ch);closed:=poll(ch);var absent chan int;idle:=poll(absent);flag:=0;if offer(absent){flag=1};evaluated(absent,&calls);return []int{a,b,closed,idle,flag,calls}}
func Concurrent()int{ch:=make(chan int);go put(ch,42);return get[int](ch)}
type item struct{value int}
func objectValue[C receive[*item]](ch C)int{value,_:=<-ch;if value!=nil{return value.value};return 0}
func roundTrip[E any,C both[E]](ch C,value E)E{ch<-value;return <-ch}
func OOP()[]int{ch:=make(chan *item,1);item:=&item{17};put(ch,item);a:=objectValue(ch);value:=roundTrip(ch,item);identity:=0;if value==item{identity=1};close(ch);return []int{a,identity,objectValue(ch)}}
type child struct{item}
func offerChild[C send[*item]](ch C){value:=&child{item{29}};select{case ch<-&value.item:default:}}
func putChild[C send[*item]](ch C){value:=&child{item{29}};ch<-&value.item}
func Upcast()[]int{ch:=make(chan *item,3);offerChild(ch);putChild(ch);value:=&child{item{29}};select{case ch<-&value.item:default:};a,b,c:=<-ch,<-ch,<-ch;return []int{a.value,b.value,c.value}}
func ClosedSend(){ch:=make(chan int);close(ch);put(ch,1)}
`
	comparison := `package genericchannels_test
import("testing";"reflect";g "generic-channel-operations.test";r "generic-channel-operations.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestChannels(t *testing.T){for _,pair:=range [][2]func()[]int{{g.Buffered,r.Buffered},{g.Selection,r.Selection},{g.OOP,r.OOP},{g.Upcast,r.Upcast}}{if got,want:=pair[0](),pair[1]();!reflect.DeepEqual(got,want){t.Fatalf("got=%v want=%v",got,want)}};if g.Concurrent()!=r.Concurrent(){t.Fatal("concurrent")};if !panics(g.ClosedSend)||!panics(r.ClosedSend){t.Fatal("closed send must panic")}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-channel-operations.test", generated, reference, comparison)
	for _, bound := range []string{"Mixed", "Empty"} {
		for _, operation := range []string{"ch<-1;", "const value=<-ch;"} {
			input := `import go contracts from "generic-channel-operations.test/contracts";function bad<C extends contracts.` + bound + `>(ch:C):void{` + operation + `}`
			path := filepath.Join(root, "bad.km")
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diagnostics, err := EmitGo([]string{path}, "bad")
			if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "identical element") {
				t.Fatalf("%s %s: err=%v diagnostics=%v", bound, operation, err, diagnostics)
			}
		}
	}
}
