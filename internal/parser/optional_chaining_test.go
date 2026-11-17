package parser

import "testing"

func TestParsesOptionalChains(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{`a?.b`, `a?.b.c`, `a?.b?.c`, `a?.(b)`, `a?.[i]`, `a?.b<T>(x)`, `a?.b[0]?.c()`, `(a?.b).c`, `a?.b!`, `a?.b??c`} {
		t.Run(expr, func(t *testing.T) {
			_, n := parseSource(t, `const value=`+expr+`;`)
			if n != 0 {
				t.Fatalf("%d diagnostics", n)
			}
		})
	}
}

func TestRejectsOptionalAssignmentTargets(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{`a?.b=1`, `a?.[0]=1`, `a?.b++`, `a?.b+=1`} {
		t.Run(expr, func(t *testing.T) {
			_, n := parseSource(t, `function f():void{`+expr+`;}`)
			if n == 0 {
				t.Fatal("optional assignment accepted")
			}
		})
	}
}
