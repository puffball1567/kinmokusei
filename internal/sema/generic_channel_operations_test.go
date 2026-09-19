package sema

import (
	gotypes "go/types"
	"strings"
	"testing"
)

func TestGenericChannelOperations(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"send directions", `constraint C<E>=~GoChannel<E>|~GoSendChannel<E>;function put<E,T extends C<E>>(ch:T,value:E):void{ch<-value;}`, ""},
		{"receive directions", `constraint C<E>=~GoChannel<E>|~GoReceiveChannel<E>;function get<E,T extends C<E>>(ch:T):E{return <-ch;}`, ""},
		{"checked receive", `constraint C=~GoReceiveChannel<int>;function get<T extends C>(ch:T):boolean{const [value,ok]=<-ch;return ok;}`, ""},
		{"select", `constraint C=~GoChannel<int>;function pick<T extends C>(ch:T):int{select{case ch<-1{return 1;}case const [value,ok]=<-ch{return value;}default{return 0;}}}`, ""},
		{"intersection", `constraint C=~GoChannel<int>|~GoReceiveChannel<int>;constraint S=C&~GoChannel<int>;function put<T extends S>(ch:T):void{ch<-1;}`, ""},
		{"nullable oop", `class Item{public value:int=1;}constraint C=~GoChannel<Item|null>;function get<T extends C>(ch:T):int{const value=<-ch;if(value!==null){return value.value;}return 0;}`, ""},
		{"generic method", `constraint C<E>=~GoChannel<E>;class Box<E>{public function relay<T extends C<E>>(ch:T,value:E):E{ch<-value;return <-ch;}}`, ""},
		{"method nullable mismatch", `class Item{}constraint C<E>=~GoChannel<E>;class Box<E>{public function get<T extends C<E>>(ch:T):E{return <-ch;}}function bad(ch:GoChannel<Item|null>):Item{return new Box<Item>().get<GoChannel<Item|null>>(ch);}`, "nullable type information"},
		{"direction specialization", `constraint C=~GoSendChannel<int>;function put<T extends C>(ch:T):void{ch<-1;}function bad(ch:GoReceiveChannel<int>):void{put(ch);}`, "does not satisfy"},
		{"receive only send", `constraint C=~GoReceiveChannel<int>;function put<T extends C>(ch:T):void{ch<-1;}`, "send-capable"},
		{"send only receive", `constraint C=~GoSendChannel<int>;function get<T extends C>(ch:T):int{return <-ch;}`, "receive-capable"},
		{"different send elements", `constraint C=~GoChannel<int>|~GoSendChannel<string>;function put<T extends C>(ch:T):void{ch<-1;}`, "identical element"},
		{"different receive elements", `constraint C=~GoChannel<int>|~GoReceiveChannel<string>;function get<T extends C>(ch:T):int{return <-ch;}`, "identical element"},
		{"mixed nullability", `class Item{}constraint C=~GoChannel<Item>|~GoReceiveChannel<Item|null>;function get<T extends C>(ch:T):Item{return <-ch;}`, "nullability"},
		{"mixed send nullability", `class Item{}constraint C=~GoChannel<Item>|~GoSendChannel<Item|null>;function put<T extends C>(ch:T):void{ch<-new Item();}`, "nullability"},
		{"non channel", `constraint C=~GoChannel<int>|~int;function get<T extends C>(ch:T):int{return <-ch;}`, "receive-capable"},
		{"unrestricted send", `function put<T>(ch:T):void{ch<-1;}`, "send-capable"},
		{"unrestricted receive", `function get<T>(ch:T):int{return <-ch;}`, "receive-capable"},
		{"wrong value", `constraint C=~GoChannel<int>;function put<T extends C>(ch:T):void{ch<-"wrong";}`, "cannot use"},
		{"constant overflow", `constraint C=~GoChannel<byte>;function put<T extends C>(ch:T):void{ch<-256;}`, "cannot be represented"},
		{"nullable access", `class Item{public value:int=1;}constraint C=~GoChannel<Item|null>;function get<T extends C>(ch:T):int{const value=<-ch;return value.value;}`, "nullable"},
		{"null send", `class Item{}constraint C=~GoChannel<Item>;function put<T extends C>(ch:T):void{ch<-null;}`, "cannot use"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}

func TestImportedChannelDirectionSurvivesStorageAndSubstitution(t *testing.T) {
	t.Parallel()
	checker := &Checker{}
	for _, direction := range []gotypes.ChanDir{gotypes.SendRecv, gotypes.SendOnly, gotypes.RecvOnly} {
		original := gotypes.NewChan(direction, gotypes.Typ[gotypes.Int])
		value, err := kinmokuseiTypeFromGo(original)
		if err != nil {
			t.Fatal(err)
		}
		storage, ok := checker.goTypeForNativeStorage(value)
		if !ok || !gotypes.Identical(storage, original) {
			t.Fatalf("direction=%v storage=%v", direction, storage)
		}
		// A substituted native element may temporarily have no Go storage.
		parameter := gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, "E", nil), gotypes.NewInterfaceType(nil, nil).Complete())
		value.GoType = nil
		value.Element = &Type{Kind: TypeParameter, Name: "E", GoType: parameter}
		value = substituteNativeTypeParameters(value, nativeTypeBindings{parameter: builtins["int"]})
		if value.GoType == nil || !gotypes.Identical(value.GoType, original) {
			t.Fatalf("direction=%v substituted=%v", direction, value.GoType)
		}
	}
}
