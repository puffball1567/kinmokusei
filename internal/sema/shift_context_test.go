package sema

import (
	"strings"
	"testing"
)

func TestShiftContexts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"narrow result", `function f(n:uint):byte{return 1<<n;}`, ""},
		{"narrow overflow", `function f(n:uint):byte{return 300<<n;}`, "overflows"},
		{"negative unsigned", `function f(n:uint):byte{return -1<<n;}`, "overflows"},
		{"floating result", `function f(n:uint):float{return 1<<n;}`, "must be integer"},
		{"nested narrow", `function f(n:uint):byte{return (1<<n)+2;}`, ""},
		{"nested overflow", `function f(n:uint):byte{return (300<<n)+2;}`, "overflows"},
		{"typed peer", `function f(n:uint,x:byte):byte{return (1<<n)+x;}`, ""},
		{"typed peer overflow", `function f(n:uint,x:byte):byte{return (300<<n)+x;}`, "overflows"},
		{"float literal shift", `function f(n:uint):int{return 1.0<<n;}`, ""},
		{"float literal size", `function f(n:uint):int[]{return make[int[]](1.0<<n);}`, ""},
		{"runtime still typed", `function f(n:uint):byte{const x=1<<n;return x;}`, "cannot use"},
		{"unsigned right shift", `function f(n:uint):byte{return 255>>n;}`, ""},
		{"signed overflow", `function f(n:uint):int8{return 128>>n;}`, "overflows"},
		{"conversion", `function f(n:uint):byte{return byte(1<<n);}`, ""},
		{"conversion overflow", `function f(n:uint):byte{return byte(300<<n);}`, "overflows"},
		{"conversion float", `function f(n:uint):float{return float(1<<n);}`, "must be integer"},
		{"conversion string", `function f(n:uint):string{return string(1<<n);}`, "must be integer"},
		{"typed conversion still allowed", `function f(n:uint,x:int):byte{return byte(x<<n);}`, ""},
		{"named conversion", `type B=distinct byte;function f(n:uint):B{return B(1<<n);}`, ""},
		{"named conversion overflow", `type B=distinct byte;function f(n:uint):B{return B(300<<n);}`, "overflows"},
		{"generic result", `constraint B=~byte|~uint16;function f<T extends B>(n:uint):T{return 1<<n;}`, ""},
		{"generic overflow", `constraint B=~byte|~uint16;function f<T extends B>(n:uint):T{return 300<<n;}`, "cannot convert"},
		{"argument", `function take(x:byte):void{}function f(n:uint):void{take(1<<n);}`, ""},
		{"argument overflow", `function take(x:byte):void{}function f(n:uint):void{take(300<<n);}`, "overflows"},
		{"array element", `function f(n:uint):byte[]{return [1<<n];}`, ""},
		{"array overflow", `function f(n:uint):byte[]{return [300<<n];}`, "overflows"},
		{"index", `function f(n:uint,x:int[]):int{return x[1.0<<n];}`, ""},
		{"slice bounds", `function f(n:uint,x:int[]):int[]{return x[1.0<<n:2.0<<n];}`, ""},
		{"channel capacity", `function f(n:uint):GoChannel<int>{return goChannel<int>(1.0<<n);}`, ""},
		{"default float", `function f(n:uint):void{const x=1.0<<n;}`, "must be integer"},
		{"default arrow", `function f(n:uint):void{const run=()=>1.0<<n;}`, "must be integer"},
		{"comparison typed peer", `function f(n:uint,x:byte):boolean{return (1.0<<n)==x;}`, ""},
		{"comparison overflow", `function f(n:uint,x:byte):boolean{return (300<<n)==x;}`, "overflows"},
		{"comparison float peer", `function f(n:uint):boolean{return (1<<n)==1.0;}`, "must be integer"},
		{"float runtime left", `function f(n:uint,x:float):int{return int(x<<n);}`, "requires integer operands"},
		{"interface default", `import go fmt from "fmt";function f(n:uint):void{fmt.Print(1<<n);}`, ""},
		{"interface floating default", `import go fmt from "fmt";function f(n:uint):void{fmt.Print(1.0<<n);}`, "must be integer"},
		{"index overflow", `function f(n:uint,x:int[]):int{return x[9223372036854775808<<n];}`, "index must be an integer"},
		{"allocation overflow", `function f(n:uint):int[]{return make[int[]](9223372036854775808<<n);}`, "size must be an integer"},
		{"generic explicit argument", `function take<T>(value:T):T{return value;}function f(n:uint):byte{return take<byte>(1<<n);}`, ""},
		{"generic argument overflow", `function take<T>(value:T):T{return value;}function f(n:uint):byte{return take<byte>(300<<n);}`, "overflows"},
		{"typed constant overflow", `function f(n:uint):void{const value:byte=300<<n;}`, "overflows"},
		{"map key overflow", `function f(n:uint,m:Map<byte,int>):int{return m[300<<n];}`, "overflows"},
		{"generic typed peer", `function take<T>(a:T,b:T):T{return a;}function f(n:uint,x:byte):byte{return take(1.0<<n,x);}`, ""},
		{"generic default float", `function take<T>(a:T):T{return a;}function f(n:uint):void{const x=take(1.0<<n);}`, "must be integer"},
		{"Go generic explicit", `import go cmp from "cmp";function f(n:uint):int{return cmp.Compare<byte>(1<<n,1);}`, ""},
		{"Go generic overflow", `import go cmp from "cmp";function f(n:uint):int{return cmp.Compare<byte>(300<<n,1);}`, "overflows"},
		{"Go generic inferred", `import go cmp from "cmp";function f(n:uint,x:byte):int{return cmp.Compare(1.0<<n,x);}`, ""},
		{"Go generic default float", `import go cmp from "cmp";function f(n:uint):int{return cmp.Compare(1.0<<n,1);}`, "must be integer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
