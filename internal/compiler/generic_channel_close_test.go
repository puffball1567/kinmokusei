package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericChannelCloseMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module generic-channel-close.test\n\ngo 1.23\n",
		"contracts/contracts.go": `package contracts
type Writable interface{~chan int|~chan<- string}
type Bad interface{~chan int|~<-chan int}
type Empty interface{~chan int;~chan string}
`,
		"bounds.km": `export constraint Writable=~GoChannel<int>|~GoSendChannel<int>|~GoSendChannel<string>;
export function finish<C extends Writable>(channel:C):void{closeGoChannel(channel);}`,
		"bridge.km": `export {Writable,finish as stop} from "./bounds";`,
		"entry.km": `import {Writable,stop} from "./bridge";
import go contracts from "generic-channel-close.test/contracts";
type Named=distinct GoChannel<int>;
constraint Channels<E>=~GoChannel<E>|~GoSendChannel<E>;
class Closer<E>{public function finish<C extends Channels<E>>(channel:C):void{closeGoChannel(channel);}}
class Item{constructor(public value:int){}}
function imported<C extends contracts.Writable>(channel:C):void{closeGoChannel(channel);}
function later<C extends Writable>(channel:C):void{defer closeGoChannel(channel);}
function evaluated<C extends Writable>(channel:C,count:*int):void{const next=():C=>{*count+=1;return channel;};closeGoChannel(next());}
export function Buffered():int[]{const channel=Named(goChannel<int>(2));channel<-7;channel<-9;let calls=0;evaluated(channel,&calls);const [a,first]=<-channel;const [b,second]=<-channel;const [zero,open]=<-channel;let flags=0;if(first){flags+=1;}if(second){flags+=2;}if(open){flags+=4;}return [a,b,zero,flags,calls,len(channel),cap(channel)];}
export function Text():string{const channel=goChannel<string>(1);channel<-"hello";const send:GoSendChannel<string>=channel;stop(send);const value=<-channel;return value;}
export function Imported():boolean{const channel=goChannel<int>();imported(channel);const [value,open]=<-channel;return open;}
export function Deferred():boolean{const channel=goChannel<int>();later(channel);const [value,open]=<-channel;return open;}
export function Concurrent():int{const channel=goChannel<int>();go stop(channel);return <-channel;}
export function OOP():int[]{const item=new Item(42);const channel=goChannel<Item|null>(1);channel<-item;new Closer<Item|null>().finish<GoChannel<Item|null>>(channel);const [value,present]=<-channel;const [zero,open]=<-channel;let result=0;if(value!==null){result=value.value;}let flags=0;if(present){flags+=1;}if(open){flags+=2;}if(zero===null){flags+=4;}return [result,flags];}
export function Nil():void{const channel:Named=nil;stop(channel);}
export function Twice():void{const channel=goChannel<int>();stop(channel);stop(channel);}
export function SendClosed():void{const channel=goChannel<int>(1);stop(channel);channel<-1;}
`,
	}
	for name, input := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "genericclose")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	reference := `package reference
type writable interface{~chan int|~chan<- int|~chan<- string}
type importedWritable interface{~chan int|~chan<- string}
type named chan int
func stop[C writable](channel C){close(channel)}
func imported[C importedWritable](channel C){close(channel)}
func later[C writable](channel C){defer close(channel)}
func evaluated[C writable](channel C,count *int){next:=func()C{*count+=1;return channel};close(next())}
type channels[E any] interface{~chan E|~chan<- E}
type item struct{value int}
func finish[E any,C channels[E]](channel C){close(channel)}
func Buffered()[]int{channel:=make(named,2);channel<-7;channel<-9;calls:=0;evaluated(channel,&calls);a,first:=<-channel;b,second:=<-channel;zero,open:=<-channel;flags:=0;if first{flags+=1};if second{flags+=2};if open{flags+=4};return []int{a,b,zero,flags,calls,len(channel),cap(channel)}}
func Text()string{channel:=make(chan string,1);channel<-"hello";var send chan<- string=channel;stop(send);return <-channel}
func Imported()bool{channel:=make(chan int);imported(channel);_,open:=<-channel;return open}
func Deferred()bool{channel:=make(chan int);later(channel);_,open:=<-channel;return open}
func Concurrent()int{channel:=make(chan int);go stop(channel);return <-channel}
func OOP()[]int{value:=&item{42};channel:=make(chan *item,1);channel<-value;finish[*item](channel);got,present:=<-channel;zero,open:=<-channel;result:=0;if got!=nil{result=got.value};flags:=0;if present{flags+=1};if open{flags+=2};if zero==nil{flags+=4};return []int{result,flags}}
func Nil(){var channel named;stop(channel)}
func Twice(){channel:=make(chan int);stop(channel);stop(channel)}
func SendClosed(){channel:=make(chan int,1);stop(channel);channel<-1}
`
	comparison := `package genericclose_test
import("testing";"reflect";g "generic-channel-close.test";r "generic-channel-close.test/reference")
func panics(f func())(yes bool){defer func(){yes=recover()!=nil}();f();return}
func TestClose(t *testing.T){if !reflect.DeepEqual(g.Buffered(),r.Buffered())||!reflect.DeepEqual(g.OOP(),r.OOP()){t.Fatal("drain/identity")};if g.Text()!=r.Text()||g.Imported()!=r.Imported()||g.Deferred()!=r.Deferred()||g.Concurrent()!=r.Concurrent(){t.Fatal("close behavior")};for _,pair:=range [][2]func(){{g.Nil,r.Nil},{g.Twice,r.Twice},{g.SendClosed,r.SendClosed}}{if panics(pair[0])!=panics(pair[1]){t.Fatal("panic behavior")}}}
`
	runGeneratedGoDifferentialTest(t, root, "generic-channel-close.test", generated, reference, comparison)
	for _, bound := range []string{"Bad", "Empty"} {
		input := `import go contracts from "generic-channel-close.test/contracts";function bad<C extends contracts.` + bound + `>(channel:C):void{closeGoChannel(channel);}`
		path := filepath.Join(root, "bad.km")
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
		_, diagnostics, err := EmitGo([]string{path}, "bad")
		if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "send-capable") {
			t.Fatalf("%s: err=%v diagnostics=%v", bound, err, diagnostics)
		}
	}
}
