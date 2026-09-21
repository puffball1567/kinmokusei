package sema

import (
	"strings"
	"testing"
)

func TestContainsNativeInterfaceInCallbackResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result Type
		want   bool
	}{
		{"scalar", Type{Kind: Int}, false},
		{"interface", Type{Kind: Interface}, true},
		{"scalar results", Type{Kind: MultiValue, Results: []Type{{Kind: Int}, {Kind: String}}}, false},
		{"interface result", Type{Kind: MultiValue, Results: []Type{{Kind: Int}, {Kind: Interface}}}, true},
		{"nested interface result", Type{Kind: MultiValue, Results: []Type{{Kind: Int}, {Kind: Array, Element: &Type{Kind: Interface}}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callback := Type{Kind: Function, Result: &tc.result}
			if got := containsNativeInterface(callback); got != tc.want {
				t.Fatalf("containsNativeInterface(callback) = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInferTypeArgumentsFromCallbackResults(t *testing.T) {
	const declarations = `function pair():(int,string){return 7,"ok";} function first<T,U>(f:()=>(T,U)):T{const [value,_]=f();return value;}`
	for _, expression := range []string{
		`first(pair)`,
		`first(()=>(pair()))`,
		`first(():(int,string)=>{return 7,"ok";})`,
		`first<int>(pair)`,
	} {
		t.Run(expression, func(t *testing.T) {
			if got := checkSource(t, declarations+`function use():int{return `+expression+`;}`); len(got) != 0 {
				t.Fatal(got)
			}
		})
	}
}

func TestCallbackResultInferenceBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"repeated parameter", `function pair():(int,string){return 1,"x";} function use<T>(f:()=>(T,T)):void{} function run():void{use(pair);}`, "result 2"},
		{"explicit conflict", `function pair():(string,int){return "x",1;} function use<T,U>(f:()=>(T,U)):void{} function run():void{use<int>(pair);}`, "cannot"},
		{"count", `function pair():(int,int,int){return 1,2,3;} function use<T>(f:()=>(T,T)):void{} function run():void{use(pair);}`, "result count mismatch"},
		{"constraint", `constraint Number=~int; function pair():(string,int){return "x",1;} function use<T extends Number>(f:()=>(T,int)):void{} function run():void{use(pair);}`, "does not satisfy"},
		{"dependent callback", `function use<T>(f:()=>(T,int),g:(v:T)=>string):string{const [value,_]=f();return g(value);} function run():string{return use(()=>{return "ok",1;},(value)=>value);}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, tc.source), "\n")
			if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("want %q, got %s", tc.want, got)
			}
		})
	}
}
