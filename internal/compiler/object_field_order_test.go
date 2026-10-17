package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestObjectFieldOrderMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "objects.km")
	input := `let trace: string = "";
let calls: int = 0;
function mark(tag: string, value: int): int { trace = trace + tag; calls = calls + 1; return value; }
function reset(): void { trace = ""; calls = 0; }
export function State(): (string, int) { return trace, calls; }
export function Literal(): (string, int) {
 reset();
 const o = { zeta: mark("z", 1), alpha: mark("a", 2), mid: mark("m", 3) };
 return trace, calls * 1000 + o.zeta * 100 + o.alpha * 10 + o.mid;
}
export function Nested(): (string, int) {
 reset();
 const o = { zeta: { z: mark("Z", 1), a: mark("A", 2) }, alpha: mark("a", 3) };
 return trace, calls * 1000 + o.zeta.z * 100 + o.zeta.a * 10 + o.alpha;
}
alias Shape = { alpha: int8, zeta: int8 };
function narrow(tag: string, value: int8): int8 { mark(tag, int(value)); return value; }
function makeShape(): Shape { return { zeta: narrow("rZ", 3), alpha: narrow("rA", 4) }; }
function takeShape(o: Shape): int { return int(o.zeta) * 10 + int(o.alpha); }
export function Contextual(): (string, int) {
 reset();
 let o: Shape = { zeta: narrow("Z", 1), alpha: narrow("A", 2) };
 const first = takeShape(o);
 o = makeShape();
 const second = takeShape(o);
 const third = takeShape({ zeta: narrow("pZ", 5), alpha: narrow("pA", 6) });
 return trace, first * 10000 + second * 100 + third;
}
function generic<T>(value: T): { alpha: T, zeta: T } { return { zeta: value, alpha: value }; }
export function Identity(): int {
 let first = { zeta: 1, alpha: 2 };
 const second = { alpha: 3, zeta: 4 };
 first = second;
 const other = generic<int>(5);
 const keyword = { type: 6, package: 7 };
 const empty = {};
 const copy = empty;
 const _ = copy;
 return first.zeta * 100 + first.alpha * 10 + other.zeta + keyword.type + keyword.package;
}
export function Receives(): int {
 const ch = goChannel<int>(2);
 ch <- 1;
 ch <- 2;
 const o = { zeta: <-ch, alpha: <-ch };
 return o.zeta * 10 + o.alpha;
}
function explode(): int { trace = trace + "!"; calls = calls + 1; const xs: int[] = []; return xs[0]; }
export function Stop(): void {
 reset();
 const o = { zeta: mark("z", 1), alpha: explode(), mid: mark("m", 3) };
 const _ = o;
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "objects")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "objects")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) || len(mapped.SourceMap.Mappings) == 0 {
		t.Fatalf("mapped emit: %v %v", err, diagnostics)
	}
	reference := `package reference
var trace string
var calls int
func mark(tag string, value int) int { trace += tag; calls++; return value }
func reset() { trace = ""; calls = 0 }
func State() (string, int) { return trace, calls }
func Literal() (string, int) {
 reset()
 z := mark("z", 1); a := mark("a", 2); m := mark("m", 3)
 return trace, calls * 1000 + z * 100 + a * 10 + m
}
func Nested() (string, int) {
 reset()
 z := mark("Z", 1); a := mark("A", 2); outer := mark("a", 3)
 return trace, calls * 1000 + z * 100 + a * 10 + outer
}
func Contextual() (string, int) {
 reset()
 z := mark("Z", 1); a := mark("A", 2); first := z*10+a
 z = mark("rZ", 3); a = mark("rA", 4); second := z*10+a
 z = mark("pZ", 5); a = mark("pA", 6); third := z*10+a
 return trace, first * 10000 + second * 100 + third
}
func Identity() int { return 4*100 + 3*10 + 5 + 6 + 7 }
func Receives() int { ch:=make(chan int,2); ch<-1; ch<-2; z:=<-ch; a:=<-ch; return z*10+a }
func explode() int { trace += "!"; calls++; xs := []int{}; return xs[0] }
func Stop() { reset(); mark("z", 1); explode(); mark("m", 3) }
`
	tests := `package objects
import ("testing"; "objectorder/reference")
func TestObjects(t *testing.T) {
 for name, pair := range map[string][2]func()(string,int){
  "literal": {Literal, reference.Literal},
  "nested": {Nested, reference.Nested},
  "contextual": {Contextual, reference.Contextual},
 } {
  t.Run(name, func(t *testing.T) {
   gotTrace, gotValue := pair[0](); wantTrace, wantValue := pair[1]()
   if gotTrace != wantTrace || gotValue != wantValue { t.Fatalf("got (%q,%d), want (%q,%d)", gotTrace,gotValue,wantTrace,wantValue) }
  })
 }
 if got,want:=Identity(),reference.Identity();got!=want{t.Fatalf("identity=%d,want=%d",got,want)}
 if got,want:=Receives(),reference.Receives();got!=want{t.Fatalf("receives=%d,want=%d",got,want)}
}
func panics(f func()) (yes bool) { defer func(){yes=recover()!=nil}(); f(); return }
func TestStop(t *testing.T) {
 gotPanic,wantPanic:=panics(Stop),panics(reference.Stop)
 gotTrace,gotCalls:=State(); wantTrace,wantCalls:=reference.State()
 if gotPanic!=wantPanic || !gotPanic || gotTrace!=wantTrace || gotCalls!=wantCalls {
  t.Fatalf("got (%v,%q,%d), want (%v,%q,%d)",gotPanic,gotTrace,gotCalls,wantPanic,wantTrace,wantCalls)
 }
}
`
	runGeneratedGoDifferentialTest(t, root, "objectorder", generated, reference, tests)
}
