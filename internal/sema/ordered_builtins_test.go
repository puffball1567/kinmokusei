package sema

import (
	"strings"
	"testing"
)

func TestOrderedBuiltinConstants(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"narrow constant", `const a=min(255,256);const b=a;function f():byte{return b;}`, ""},
		{"exact floating", `const a=max(1e400,2e400);const b=a;function f():float{return b/1e400;}`, ""},
		{"unselected huge constant", `function f():byte{return min(1,1e400);}`, ""},
		{"kind promotion", `const a=max(1,2.5,2);function f():float{return a;}`, ""},
		{"floating narrow", `function f(v:float32):float32{return min(1.5,v,2.5);}`, ""},
		{"typed integer integral float", `function f(v:int):int{return max(1.,v);}`, ""},
		{"typed len remains int", `function f(v:[3]int):byte{return min(len(v),2);}`, "of type int"},
		{"string alias", `type Text=distinct string;const a=min("z","a");const b=a;function f():Text{return b;}`, ""},
		{"named string", `type Text=distinct string;function f(v:Text):Text{return min("z",v);}`, ""},
		{"named scalar", `type Count=distinct int8;function f(v:Count):Count{return max(1,v,127);}`, ""},
		{"generic", `constraint Ordered=~int|~string;function f<T extends Ordered>(a:T,b:T):T{return max(a,b);}`, ""},
		{"generic narrow constant", `constraint Small=~int8;function f<T extends Small>(a:T):T{return min(a,127);}`, ""},
		{"generic constant stays runtime", `constraint Small=~int8;function f<T extends Small>():void{const a=min(T(1),T(2));const p=&a;}`, ""},
		{"constant index", `function f(a:[3]int):int{return a[min(1.,2.)];}`, ""},
		{"constant nested len", `function f():int{const s=min("abc","z");return len(s);}`, ""},
		{"single argument", `function f():byte{return max(255);}`, ""},
		{"constant address", `function f():void{const a=min(1,2);const b=a;const p=&b;}`, "addressable"},
		{"runtime address", `function f(v:int):void{const a=min(1,v);const p=&a;}`, ""},
		{"mutable stays runtime", `function f():void{let v=1;const a=min(v,2);const p=&a;}`, ""},
		{"call stays runtime", `function value():int{return 1;}function f():void{const a=max(value(),2);const p=&a;}`, ""},
		{"overflow", `function f():byte{return max(255,256);}`, "overflows"},
		{"fractional", `function f():int{return min(1.5,2.5);}`, "truncated"},
		{"typed overflow even unselected", `function f(v:byte):byte{return min(v,256);}`, "overflows"},
		{"floating overflow even unselected", `function f(v:float):float{return min(1e400,v);}`, "overflows"},
		{"generic overflow", `constraint Small=~int8;function f<T extends Small>(v:T):T{return max(v,128);}`, "cannot convert"},
		{"named mismatch", `type A=distinct int;type B=distinct int;function f(a:A,b:B):A{return min(a,b);}`, "mismatched types"},
		{"typed string mismatch", `type Text=distinct string;function f(v:Text,s:string):Text{return max(v,s);}`, "mismatched types"},
		{"typed constants stay typed", `const a:int8=1;const b:int16=2;function f():int8{return min(a,b);}`, "mismatched types"},
		{"index bounds", `function f(a:[3]int):int{return a[max(2,3)];}`, "out of bounds"},
		{"negative size", `function f():void{const n=min(2,-1);const xs=makeSlice<int>(n);}`, "negative"},
		{"size order", `function f():void{const xs=makeSlice<int>(max(1,3),min(4,2));}`, "capacity cannot be smaller"},
		{"nonordered", `function f():complex128{return min(1i,2i);}`, "requires ordered operands"},
		{"unconstrained", `function f<T>(a:T,b:T):T{return max(a,b);}`, "requires ordered operands"},
		{"contextual shift", `function f(n:uint,v:byte):byte{return min(1<<n,v);}`, ""},
		{"contextual shift reversed", `function f(n:uint,v:byte):byte{return max(v,1<<n);}`, ""},
		{"nested contextual shift", `function f(n:uint,v:byte):byte{return min(+(1<<n)+2,v);}`, ""},
		{"shift left overflow", `function f(n:uint,v:byte):byte{return min(300<<n,v);}`, "overflows"},
		{"shift bad float context", `function f(n:uint,v:float):float{return min(1<<n,v);}`, "must be integer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
