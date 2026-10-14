package sema

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestIncompleteNativeTypeChecks(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"self map key", `type A=distinct Map<A,int>;`, "cannot be used as a Map key"},
		{"array map key", `type A=distinct Map<[2]A,int>;`, "cannot be used as a Map key"},
		{"pointer map", `type A=distinct *Map<A,int>;`, ""},
		{"channel map", `type A=distinct GoChannel<Map<A,int>>;`, ""},
		{"struct array key", `type K=distinct [1]S;struct S{public m:Map<K,int>;}`, "cannot be used as a Map key"},
		{"generic self map key", `type A<T>=distinct Map<A<T>,T>;function f(v:A<int>):void{}`, "cannot be used as a Map key"},
		{"recursive constrained channel", `type Box<T extends comparable>=distinct GoChannel<T>;type A=distinct *Box<A>;function f(v:A):A{return v;}`, ""},
		{"recursive constrained struct", `struct Box<T extends comparable>{public v:int;}type A=distinct *Box<A>;function f(v:A):A{return v;}`, ""},
		{"recursive constrained struct rejects array", `struct Box<T extends comparable>{public v:int;}type A=distinct Map<int,*Box<A>>;`, "does not satisfy"},
		{"recursive pointer key", `type A=distinct Map<*A,int>;function f(v:A):int{return len(v);}`, ""},
		{"generic recursive pointer key", `type A<T>=distinct Map<*A<T>,T>;function f(v:A<int>):int{return len(v);}`, ""},
		{"ordinary recursive slice", `type A=distinct A[];function f(v:Map<A,int>):void{}`, "cannot be used as a Map key"},
		{"forward comparable struct", `struct Table{public m:Map<Key,int>;}struct Key{public id:int;}function f(v:Table):int{return len(v.m);}`, ""},
		{"recursive storage retains verified fields", `import go reflect from "reflect";struct Box<T extends comparable>{public text:string;public v:T;}type A=distinct *Box<A>;function f(v:A):void{reflect.ValueOf(v);}`, "cannot use"},
		{"invalid value cycle with constraint", `struct S{public v:S;}struct Box<T extends comparable>{public v:T;}function f(v:Box<S>):void{}`, "does not satisfy"},
		{"recursive imported generic", `import go unique from "unique";type A=distinct *unique.Handle<A>;`, "unresolved source storage"},
		{"finite nested comparable struct", `struct Box<T>{public v:T;}function f(v:Map<Box<Box<int>>,int>):void{}`, ""},
		{"failed recursive declaration", `type A<T>=distinct Map<A<T>,Missing>;`, "unknown type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if failure := recover(); failure != nil {
					t.Fatalf("checker panicked: %v\n%s", failure, debug.Stack())
				}
			}()
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
