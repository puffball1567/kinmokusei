package sema

import (
	"strings"
	"testing"
)

func TestGenericChannelClose(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"bidirectional", `constraint C=~GoChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, ""},
		{"send only", `constraint C=~GoSendChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, ""},
		{"mixed directions", `constraint C=~GoChannel<int>|~GoSendChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, ""},
		{"different elements", `constraint C=~GoChannel<int>|~GoSendChannel<string>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, ""},
		{"dependent element", `constraint C<E>=~GoChannel<E>|~GoSendChannel<E>;function finish<E,T extends C<E>>(ch:T):void{closeGoChannel(ch);}`, ""},
		{"oop elements", `class Item{}constraint C=~GoChannel<Item>|~GoSendChannel<Item|null>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, ""},
		{"generic method", `constraint C<E>=~GoChannel<E>|~GoSendChannel<E>;class Box<E>{public function finish<T extends C<E>>(ch:T):void{closeGoChannel(ch);}}`, ""},
		{"intersection", `constraint C=~GoChannel<int>|~GoReceiveChannel<int>;constraint S=C&~GoChannel<int>;function finish<T extends S>(ch:T):void{closeGoChannel(ch);}`, ""},
		{"receive only", `constraint C=~GoReceiveChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, "send-capable"},
		{"mixed receive", `constraint C=~GoChannel<int>|~GoReceiveChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, "send-capable"},
		{"non channel", `constraint C=~GoChannel<int>|~int;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}`, "send-capable"},
		{"unrestricted", `function finish<T>(ch:T):void{closeGoChannel(ch);}`, "send-capable"},
		{"bad specialization", `constraint C=~GoSendChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel(ch);}function bad(ch:GoReceiveChannel<int>):void{finish(ch);}`, "does not satisfy"},
		{"arity", `constraint C=~GoChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel(ch,ch);}`, "expects one"},
		{"type arguments", `constraint C=~GoChannel<int>;function finish<T extends C>(ch:T):void{closeGoChannel<T>(ch);}`, "does not accept type arguments"},
		{"nullable concrete", `function finish(ch:GoChannel<int>|null):void{closeGoChannel(ch);}`, "must be checked against null"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
