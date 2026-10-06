package sema

import (
	gotypes "go/types"
	"strings"
	"testing"
)

func TestConstantArrayTypeLengths(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function size(value:[1+2]int):[6/2]int { return value; }`,
		`alias Row = [N]int; const N = M + 1; const M = 2; function use(value:Row):[3]int { return value; }`,
		`const N = len("湯"); function use(value:[N]byte):[3]byte { return value; }`,
		`const N = max(1,min(3,4)); function use(value:[N]int):[3]int { return value; }`,
		`function use():int { const n=2; const a:[n+1]int=[1,2,3]; return len(a); }`,
		`function use(value:[2.0+1i-1i]int):[3-1]int { return value; }`,
		`import go sha256 from "crypto/sha256"; function use(value:[sha256.Size]byte):[32]byte {return value;}`,
		`import go { Size } from "crypto/sha256"; function use(value:[Size]byte):[32]byte {return value;}`,
		`struct Row<T> { public items:[2+1]T; } function use(value:Row<int>):[3]int {return value.items;}`,
		`function use():int { let a:[2]int=[1,2]; const b:[len(a)]int=[3,4]; return b[1]; }`,
		`const B:[2]int=[1,2]; const A:[len(B)]int=[3,4]; function use():int {return A[1];}`,
		`enum Count { Zero, N=3 } alias Row=[Count.N]int; function use(a:Row):[3]int {return a;}`,
		`alias Row=[Size.N]int; class Size { public static const N:int=3; } function use(a:Row):[3]int {return a;}`,
		`class Row { public items:[Row.N]int=[1,2,3]; public static const N:int=3; }`,
		`alias Row=[Child.N]int; class Child extends Size {} class Size { public static const N:int=3; } function use(a:Row):[3]int {return a;}`,
		`function use(a:[(1<<100)-(1<<100)+3]int):[3]int {return a;}`,
		`alias Row=[len(seed)]int; const seed:[3]int=makeSeed(); const row:Row=[4,5,6]; function makeSeed():[len(row)]int{return [1,2,3];}`,
		`alias Row=[len(seed)]int; let seed:[3]int=makeSeed(); function makeSeed():[3]int{return [1,2,3];}`,
		`alias Row=[cap(seed)]int; const seed:*[3]int=nil;`,
		`const N:int=3; function use(a:[N]int):[3]int {return a;}`,
		`const N=3; constraint Fixed=~[N]int; function use<T extends Fixed>(a:T):T {return a;}`,
		`class Limits { public static const N:int=3; } constraint Fixed=~[Limits.N]int; function use<T extends Fixed>(a:T):T {return a;}`,
		`function use(n:[2]int):int {const values:[len(n)]int=[1,2]; return len(values);}`,
		`alias Row=[Limits.N]int; class Limits {public static const N:int=len(seed);} const seed:[3]int=makeSeed(); function makeSeed():[3]int{return [1,2,3];}`,
		`alias Row=[Limits.N]int; class Limits {public static const N:int=cap(Limits.values);public static values:[3]int=makeSeed();} function makeSeed():[3]int{return [1,2,3];}`,
		`class Row { public items:[Row.N]int=[1,2,3]; private static const N:int=3; public function get(a:[Row.N]int):[Row.N]int{return a;} }`,
		`alias Row=[Child.N]int; class Child extends Base {public static const N:int=Child.hidden;} class Base {protected static const hidden:int=3;}`,
	} {
		t.Run(input, func(t *testing.T) {
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("%v", diagnostics)
			}
		})
	}
}

func TestRejectsNonconstantArrayTypeLengths(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`function bad(a:[-1]int):void {}`, "must not be negative"},
		{`function bad(a:[1.5]int):void {}`, "integer constant"},
		{`const N:float=2; function bad(a:[N]int):void {}`, "integer constant"},
		{`function bad(a:[9223372036854775808]int):void {}`, "out of range for int"},
		{`function bad(a:[1<<63]int):void {}`, "out of range for int"},
		{`function bad(a:[true]int):void {}`, "integer constant"},
		{`function bad(a:["2"]int):void {}`, "integer constant"},
		{`function bad(a:[1/0]int):void {}`, "zero"},
		{`function bad(n:int):void { const a:[n]int=[]; }`, "integer constant"},
		{`function bad():void {let n=2; const a:[n]int=[1,2];}`, "integer constant"},
		{`function value():int{return 2;} const N=value(); function bad(a:[N]int):void {}`, "integer constant"},
		{`const N=len(A); const A:[N]int=[];`, "cycle"},
		{`class Size { public static const N:int=Size.N; } alias Row=[Size.N]int;`, "cycle"},
		{`enum Size { N=Size.N } alias Row=[Size.N]int;`, "cycle"},
		{`class Size { private static const N:int=3; } alias Row=[Size.N]int;`, "private"},
		{`class Size { protected static const N:int=3; } alias Row=[Size.N]int;`, "protected"},
		{`function use(n:[2]int):[len(n)]int {return n;}`, "undefined name"},
	} {
		t.Run(test.input, func(t *testing.T) {
			diagnostics := checkSource(t, test.input)
			for _, message := range diagnostics {
				if strings.Contains(message, test.want) {
					return
				}
			}
			t.Fatalf("%v; want %q", diagnostics, test.want)
		})
	}
}

func TestArrayTypeLengthUsesSelectedTarget(t *testing.T) {
	t.Parallel()
	for _, arch := range []string{"386", "amd64", "arm", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			policy := GoInteropPolicy{Sizes: gotypes.SizesFor("gc", arch)}
			diagnostics := checkSourceWithPolicy(t, `alias Wide=[1<<31]byte;`, policy)
			if (len(diagnostics) == 0) != (arch == "amd64" || arch == "arm64") {
				t.Fatalf("%s: %v", arch, diagnostics)
			}
		})
	}
}
