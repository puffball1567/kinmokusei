package sema

import (
	"strings"
	"testing"
)

func TestMultipleResultBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"count", `function f():(int,string){return f();} function g():(int,string,int){return f();}`, "multiple result count mismatch"},
		{"void", `function f():(void,int){return f();}`, "ordinary value types"},
		{"result", `function f():(Result<int>,int){return f();}`, "ordinary value types"},
		{"task", `function f():(Task<int>,int){return f();}`, "ordinary value types"},
		{"try", `function f():(int,string){try{return f();}finally{}}`, "try statements in multiple-result callables"},
		{"single value", `function f():(int,string){return f();} function g():int{return f();}`, "require destructuring"},
		{"nullable callback", `class Box {} function f():(Box | null,int){return f();} function use(cb:()=>(Box,int)):void{} function bad():void{use(f);}`, "cannot use"},
		{"nullable Result callback", `class Box {} function f():Result<Box | null>{return ok(null);} function use(cb:()=>(Box,error)):void{} function bad():void{use(f);}`, "cannot use"},
		{"class coercion", `class Base {} class Derived extends Base {} function f():(Derived,int){return f();} function g():(Base,int){return f();}`, "cannot use result 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(checkSource(t, tc.source), "\n"); !strings.Contains(got, tc.want) {
				t.Fatalf("wanted %q, got %s", tc.want, got)
			}
		})
	}
}

func TestSourceMultipleResultsAcceptMatchingCall(t *testing.T) {
	diagnostics := checkSource(t, `
function makePair(value: int): (int, string) { return makePair(value); }
function use(value: int): (int, string) { return makePair(value); }
`)
	if len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics: %v", diagnostics)
	}
}

func TestArrowMultipleResultsAcceptMatchingCall(t *testing.T) {
	diagnostics := checkSource(t, `
function makePair(value: int): (int, string) { return makePair(value); }
const pair = (value: int): (int, string) => makePair(value);
`)
	if len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics: %v", diagnostics)
	}
}

func TestSourceMultipleResultsRejectMismatchedCall(t *testing.T) {
	diagnostics := checkSource(t, `
function pair(value: int): (int, string) { return pair(value); }
function wrong(value: int): (string, int) { return pair(value); }
`)
	if len(diagnostics) == 0 {
		t.Fatal("expected multiple-result type mismatch diagnostic")
	}
}
