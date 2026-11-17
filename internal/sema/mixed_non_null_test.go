package sema

import (
	"strings"
	"testing"
)

func TestMixedReferenceBounds(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"function scalar":      `alias F=()=>int;constraint Mixed=F|int;function f<T extends Mixed>(v:T):T{return v;}`,
		"function raw pointer": `alias F=()=>int;constraint Mixed=F|*int;function f<T extends Mixed>(v:T):T{return v;}`,
		"class scalar":         `class C{}constraint Mixed=C|int;function f<T extends Mixed>(v:T):T{return v;}`,
		"rest":                 `alias F=()=>int;constraint Mixed=F|int;function f<T extends Mixed>(...v:T[]):int{return len(v);}`,
		"arrow":                `alias F=()=>int;constraint Mixed=F|int;function f<T extends Mixed>():(v:T)=>T{return(v)=>v;}`,
		"raw only":             `constraint Mixed=*int|int;function f<T extends Mixed>(v:T):T{return v;}`,
		"blank":                `class C{}constraint Mixed=C|*int;function f<T extends Mixed>(_:T):void{}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if ds := checkSource(t, input); len(ds) != 0 {
				t.Fatal(ds)
			}
		})
	}
	for name, input := range map[string]string{
		"ambiguous pointers":  `class C{}constraint Mixed=C|*int;function f<T extends Mixed>(v:T):T{return v;}`,
		"ambiguous callbacks": `import go context from "context";type F=distinct()=>void;constraint Mixed=F|context.CancelFunc;function f<T extends Mixed>(v:T):T{return v;}`,
		"ambiguous return":    `class C{}constraint Mixed=C|*int;function f<T extends Mixed>(read:()=>T):T{return read();}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ds := strings.Join(checkSource(t, input), "\n")
			if !strings.Contains(ds, "use a concrete type or separate uniform constraints") {
				t.Fatal(ds)
			}
		})
	}
}

func TestMixedBoundaryLoopRechecking(t *testing.T) {
	t.Parallel()
	input := `alias F=()=>int;constraint Mixed=F|int;function f<T extends Mixed>(read:()=>T,again:boolean):T{let value:T=read();while(again){value=read();}return value;}`
	if ds := checkSource(t, input); len(ds) != 0 {
		t.Fatal(ds)
	}
}

func TestMixedBoundaryRuntimeNameCollision(t *testing.T) {
	t.Parallel()
	input := `import go __kinmokuseiCheckNonNullKinds from "strconv";alias F=()=>int;constraint Mixed=F|int;function f<T extends Mixed>(v:T):void{_=__kinmokuseiCheckNonNullKinds.IntSize;}`
	if ds := strings.Join(checkSource(t, input), "\n"); !strings.Contains(ds, "reserved by the non-null runtime") {
		t.Fatal(ds)
	}
}
