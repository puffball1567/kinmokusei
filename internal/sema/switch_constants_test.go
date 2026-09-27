package sema

import (
	"strings"
	"testing"
)

func TestSwitchConstantContexts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"string expression duplicate", `function f(v:string):void{switch(v){case "a"+"b"{}case "ab"{}}}`, "duplicate value switch case"},
		{"string alias duplicate", `const a="a"+"b";const b=a;function f(v:string):void{switch(v){case b{}case "ab"{}}}`, "duplicate value switch case"},
		{"float expression duplicate", `function f(v:float):void{switch(v){case 0.5+0.25{}case 0.75{}}}`, "duplicate value switch case"},
		{"float32 rounding duplicate", `function f(v:float32):void{switch(v){case 16777216.0{}case 16777217.0{}}}`, "duplicate value switch case"},
		{"rounded typed alias", `const a:float32=16777217.;const b=a;function f(v:float32):void{switch(v){case b{}case 16777216.0{}}}`, "duplicate value switch case"},
		{"ordered constant duplicate", `function f(v:int):void{switch(v){case min(2,3){}case 2{}}}`, "duplicate value switch case"},
		{"constant len duplicate", `function f(v:int):void{switch(v){case len("ab"){}case 2{}}}`, "duplicate value switch case"},
		{"case list duplicate", `function f(v:string):void{switch(v){case "a"+"b","ab"{}}}`, "duplicate value switch case"},
		{"integer tag defaults", `function f():void{switch(1){case int64(1){}}}`, "as int value"},
		{"floating tag defaults", `function f():void{switch(1.0){case float32(1){}}}`, "as float64 value"},
		{"tag overflow", `function f():void{switch(9223372036854775808){default{}}}`, "overflows"},
		{"floating tag overflow", `function f():void{switch(1e400){default{}}}`, "overflows"},
		{"shift tag defaults", `function f(n:uint):void{switch(1.0<<n){default{}}}`, "must be integer"},
		{"case shift overflow", `function f(v:byte,n:uint):void{switch(v){case 300<<n{}}}`, "overflows"},
		{"distinct case values", `function f(v:float32):void{switch(v){case 16777216.0{}case 16777218.0{}}}`, ""},
		{"boolean first match", `function f(v:boolean):void{switch(v){case true{}case true{}}}`, ""},
		{"complex first match", `function f(v:complex128):void{switch(v){case 1i{}case 1i{}}}`, ""},
		{"runtime immutable", `function f(v:int):void{for(const i=1;i<2;){switch(v){case i{}case 1{}}break;}}`, ""},
		{"runtime duplicate expression", `function f(v:int,x:int):void{const y=x;switch(v){case y{}case y{}}}`, ""},
		{"generic runtime conversion", `constraint N=~int|~int64;function f<T extends N>(v:T):void{switch(v){case T(1){}case T(1){}}}`, ""},
		{"generic repeated literal", `constraint N=~int|~int64;function f<T extends N>(v:T):void{switch(v){case 1{}case 1{}}}`, "duplicate value switch case"},
		{"lexical constant capture", `const a="a";const b=a;function f(v:string):void{const a="b";switch(v){case b{}case a{}}}`, ""},
		{"untyped integer case to float", `function f(v:float32):void{switch(v){case 16777216{}case 16777217{}}}`, "duplicate value switch case"},
		{"negative zero", `function f(v:float):void{switch(v){case 0.0{}case -0.0{}}}`, "duplicate value switch case"},
		{"named constant duplicate", `type N=distinct int;const a=N(1);function f(v:N):void{switch(v){case a{}case 1{}}}`, "duplicate value switch case"},
		{"Go constant duplicate", `import go http from "net/http";function f(v:string):void{switch(v){case http.MethodGet{}case "G"+"ET"{}}}`, "duplicate value switch case"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}

func TestInterfaceSwitchConstantIdentity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, cases, want string }{
		{"different integer types", `case int(1){}case int64(1){}`, ""},
		{"different numeric types", `case 1{}case 1.0{}`, ""},
		{"named identity", `case N(1){}case 1{}`, ""},
		{"same type duplicate", `case 1{}case int(1){}`, "duplicate value switch case"},
		{"same named type duplicate", `case N(1){}case N(1){}`, "duplicate value switch case"},
		{"alias identity", `case byte(1){}case uint8(1){}`, "duplicate value switch case"},
		{"interface exact values", `case 9007199254740992.0{}case 9007199254740993.0{}`, ""},
		{"interface integer overflow", `case 9223372036854775808{}`, "overflows"},
		{"interface floating overflow", `case 1e400{}`, "overflows"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := `import go reflect from "reflect";type N=distinct int;function f():void{switch(reflect.ValueOf(1).Interface()){` + test.cases + `}}`
			got := strings.Join(checkSource(t, input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
