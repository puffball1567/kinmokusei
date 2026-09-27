package sema

import (
	"strings"
	"testing"
)

func TestNamedCollectionElementContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input string }{
		{"slice range", `type S<T>=distinct T[];function f(v:S<Maybe>):*int{for(const item of v){return item;}return nil;}`},
		{"array range", `type S<T>=distinct [2]T;function f(v:S<Maybe>):*int{for(const item of v){return item;}return nil;}`},
		{"array pointer range", `type S<T>=distinct [2]T;function f(v:*S<Maybe>):*int{for(const item of v){return item;}return nil;}`},
		{"map key range", `type S<T>=distinct Map<T,int>;function f(v:S<Maybe>):*int{for(const [key,item] of v){return key;}return nil;}`},
		{"map value range", `type S<T>=distinct Map<int,T>;function f(v:S<Maybe>):*int{for(const item of v){return item;}return nil;}`},
		{"channel range", `type S<T>=distinct GoChannel<T>;function f(v:S<Maybe>):*int{for(const item of v){return item;}return nil;}`},
		{"channel receive", `type S<T>=distinct GoChannel<T>;function f(v:S<Maybe>):*int{return <-v;}`},
		{"channel checked receive", `type S<T>=distinct GoChannel<T>;function f(v:S<Maybe>):*int{const [item,open]=<-v;return item;}`},
		{"select receive", `type S<T>=distinct GoChannel<T>;function f(v:S<Maybe>):*int{select{case const [item,open]=<-v{return item;}default{return nil;}}}`},
		{"iterator range", `type S<T>=distinct (emit:(item:T)=>boolean)=>void;function f(v:S<Maybe>):*int{for(const item of v){return item;}return nil;}`},
		{"nested defined range", `type S<T>=distinct T[];type W<T>=distinct S<T>;function f(v:W<Maybe>):*int{for(const item of v){return item;}return nil;}`},
		{"named array pointer index", `type A<T>=distinct [2]T;type P<T>=distinct *A<T>;function f(v:P<Maybe>):*int{return v[0];}`},
		{"named array pointer reslice", `type A<T>=distinct [2]T;type P<T>=distinct *A<T>;function f(v:P<Maybe>):*int{return v[:][0];}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := `alias Maybe=*int|null;` + test.input
			if diagnostics := checkSource(t, input); !strings.Contains(strings.Join(diagnostics, "\n"), "cannot use") {
				t.Fatalf("missing element contract diagnostic: %v", diagnostics)
			}
			if diagnostics := checkSource(t, strings.ReplaceAll(input, "*int|null", "*int")); len(diagnostics) != 0 {
				t.Fatalf("matching element contract rejected: %v", diagnostics)
			}
		})
	}
}

func TestNamedCollectionOperationRestrictions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`type C<T>=distinct GoReceiveChannel<T>;function f(v:C<Maybe>):void{v<-null;}`, "receive-only"},
		{`type C<T>=distinct GoSendChannel<T>;function f(v:C<Maybe>):Maybe{return <-v;}`, "send-only"},
		{`type C<T>=distinct GoSendChannel<T>;function f(v:C<Maybe>):void{for(const item of v){}}`, "send-only"},
		{`type C<T>=distinct GoChannel<T>;function f(v:C<Maybe>|null):void{v<-null;}`, "nullable channel"},
		{`type C<T>=distinct GoChannel<T>;function f(v:C<Maybe>|null):Maybe{return <-v;}`, "nullable channel"},
		{`type I<T>=distinct (emit:(item:T)=>boolean)=>void;function f(v:I<Maybe>|null):void{for(const item of v){}}`, "nullable iterator"},
		{`type M<T>=distinct Map<T,int>;function f(v:M<*int>):void{delete(v,null);}`, "cannot use null"},
		{`type C<T>=distinct GoChannel<T>;function f(v:C<*int>):void{v<-null;}`, "cannot use null"},
	} {
		t.Run(test.input, func(t *testing.T) {
			diagnostics := strings.Join(checkSource(t, `alias Maybe=*int|null;`+test.input), "\n")
			if !strings.Contains(diagnostics, test.want) {
				t.Fatalf("diagnostics=%s want=%s", diagnostics, test.want)
			}
		})
	}
}

func TestNamedCollectionNullableOperations(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`type S<T>=distinct GoChannel<T>;function f(v:S<Maybe>):void{v<-null;}`,
		`type S<T>=distinct GoChannel<T>;function f(v:S<Maybe>):void{select{case v<-null{}default{}}}`,
		`type S<T>=distinct GoChannel<T>;function f(v:S<Maybe>):int{const item=<-v;if(item!==null){return *item;}return 0;}`,
		`type S<T>=distinct Map<T,int>;function f(v:S<Maybe>):void{delete(v,null);}`,
		`type S<T>=distinct Map<T,int>;function pair(v:S<Maybe>):(S<Maybe>,Maybe){return v,null;}function f(v:S<Maybe>):void{delete(pair(v));}`,
	} {
		t.Run(input, func(t *testing.T) {
			if diagnostics := checkSource(t, `alias Maybe=*int|null;`+input); len(diagnostics) != 0 {
				t.Fatalf("valid nullable operation rejected: %v", diagnostics)
			}
		})
	}
}
