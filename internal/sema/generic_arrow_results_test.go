package sema

import (
	"strings"
	"testing"
)

func TestGenericClassArrowResultLists(t *testing.T) {
	for _, declaration := range []string{
		`class Box<T>{constructor(public value:T){}}`,
		`class Base<T>{constructor(public value:T){}} class Box<T> extends Base<T>{constructor(value:T){super(value);}}`,
	} {
		source := declaration + ` function use<T>(value:T):void{const pair=():(Box<T>,int)=>{return new Box<T>(value),1;};const [box,n]=pair();}`
		if got := checkSource(t, source); len(got) != 0 {
			t.Fatal(got)
		}
	}
}

func TestGenericClassArrowResultContracts(t *testing.T) {
	const declarations = `class Box<T>{constructor(public value:T){}} class Other<T>{constructor(public value:T){}}`
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"inferred", `const f=()=>{return new Box<T>(value),1;};const [box,n]=f();`, true},
		{"contextual", `const f:()=>(Box<T>,int)=()=>{return new Box<T>(value),1;};`, true},
		{"matching value", `const f=():(Box<T>,int)=>{return new Box<T>(value),1;};const g:()=>(Box<T>,int)=f;`, true},
		{"nullable", `const f=():(Box<T>|null,int)=>{return null,1;};const g:()=>(Box<T>|null,int)=f;`, true},
		{"nullable mismatch", `const f=():(Box<T>|null,int)=>{return null,1;};const g:()=>(Box<T>,int)=f;`, false},
		{"nominal mismatch", `const f=():(Box<T>,int)=>{return new Box<T>(value),1;};const g:()=>(Other<T>,int)=f;`, false},
		{"type argument mismatch", `const f=():(Box<T>,int)=>{return new Box<T>(value),1;};const g:()=>(Box<int>,int)=f;`, false},
		{"slot mismatch", `const f=():(Box<T>,int)=>{return new Box<T>(value),1;};const g:()=>(Box<T>,string)=f;`, false},
		{"arity mismatch", `const f=():(Box<T>,int)=>{return new Box<T>(value),1;};const g:()=>(Box<T>,int,int)=f;`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, declarations+`function use<T>(value:T):void{`+tc.body+`}`), "\n")
			if tc.valid && got != "" {
				t.Fatal(got)
			}
			if !tc.valid && !strings.Contains(got, "cannot use") {
				t.Fatalf("expected type rejection, got %s", got)
			}
		})
	}
}
