package sema

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestChannelTypeRefWithoutCachedGoStorage(t *testing.T) {
	for _, name := range []string{"GoChannel", "GoSendChannel", "GoReceiveChannel"} {
		t.Run(name, func(t *testing.T) {
			item := Type{Kind: Class, Name: "Item"}
			ref := typeRefFromType(Type{Kind: GoChannel, Name: name, Element: &item}, source.Span{})
			if ref.Name != name || len(ref.GenericArguments) != 1 || ref.GenericArguments[0].Name != "Item" {
				t.Fatalf("channel shape not preserved: %+v", ref)
			}
		})
	}
}

func TestClassChannelOperations(t *testing.T) {
	const declarations = `class Item{public value:int=1;}class Other{}alias Maybe=Item|null;
function both<T>(ch:GoChannel<T>):GoChannel<T>{return ch;}
function receiving<T>(ch:GoReceiveChannel<T>):GoReceiveChannel<T>{return ch;}`
	for _, tc := range []struct{ name, input, want string }{
		{"send", `function f(ch:GoChannel<Item>,value:Item):void{ch<-value;}`, ""},
		{"send only", `function f(ch:GoSendChannel<Item>,value:Item):void{ch<-value;closeGoChannel(ch);}`, ""},
		{"close", `function f(ch:GoChannel<Item>):void{closeGoChannel(ch);}`, ""},
		{"select send", `function f(ch:GoSendChannel<Item>,value:Item):void{select{case ch<-value{} default{}}}`, ""},
		{"nullable element", `function f(ch:GoChannel<Maybe>):void{ch<-null;closeGoChannel(ch);}`, ""},
		{"receive only send", `function f(ch:GoReceiveChannel<Item>,value:Item):void{ch<-value;}`, "cannot send to receive-only"},
		{"receive only close", `function f(ch:GoReceiveChannel<Item>):void{closeGoChannel(ch);}`, "cannot close receive-only"},
		{"wrong element", `function f(ch:GoChannel<Item>,value:Other):void{ch<-value;}`, "cannot use"},
		{"null element", `function f(ch:GoChannel<Item>):void{ch<-null;}`, "cannot use"},
		{"nullable channel", `function f(ch:GoChannel<Item>|null,value:Item):void{ch<-value;}`, "nullable channel"},
		{"nullable close", `function f(ch:GoChannel<Item>|null):void{closeGoChannel(ch);}`, "nullable channel"},
		{"generic send close", `function f(ch:GoChannel<Item>,value:Item):void{both(ch)<-value;closeGoChannel(both(ch));}`, ""},
		{"generic nullable element", `function f(ch:GoChannel<Maybe>):void{both(ch)<-null;closeGoChannel(both(ch));}`, ""},
		{"generic wrong element", `function f(ch:GoChannel<Item>,value:Other):void{both(ch)<-value;}`, "cannot use"},
		{"generic null element", `function f(ch:GoChannel<Item>):void{both(ch)<-null;}`, "cannot use"},
		{"generic receive only send", `function f(ch:GoReceiveChannel<Item>,value:Item):void{receiving(ch)<-value;}`, "cannot send to receive-only"},
		{"generic receive only close", `function f(ch:GoReceiveChannel<Item>):void{closeGoChannel(receiving(ch));}`, "cannot close receive-only"},
		{"narrow channel element", `function f(ch:GoChannel<Maybe>):GoChannel<Item>{return both(ch);}`, "cannot use"},
		{"widen writable channel element", `function f(ch:GoChannel<Item>):GoChannel<Maybe>{return both(ch);}`, "cannot use"},
		{"reverse direction", `function f(ch:GoReceiveChannel<Item>):GoChannel<Item>{return receiving(ch);}`, "cannot use"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, declarations+tc.input), "\n")
			if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("want %q, got %s", tc.want, got)
			}
		})
	}
}
