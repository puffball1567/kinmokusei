package sema

import (
	"strings"
	"testing"
)

func TestPresenceGuardForms(t *testing.T) {
	t.Parallel()
	prelude := `class C{public n:int=7;}const YES=true;const NO=false;`
	for name, body := range map[string]string{
		"constant equality":    `if(ok===YES){return value.n;}return 0;`,
		"constant inequality":  `if(NO!==ok){return value.n;}return 0;`,
		"nested comparison":    `if((ok===YES)!==NO){return value.n;}return 0;`,
		"tag true":             `switch(ok){case true{return value.n;}default{return 0;}}`,
		"tag false exit":       `switch(ok){case false{return 0;}}return value.n;`,
		"default first":        `switch(ok){default{return value.n;}case NO{return 0;}}`,
		"negated tag":          `switch(!ok){case false{return value.n;}default{return 0;}}`,
		"comparison tag":       `switch(ok===NO){case NO{return value.n;}default{return 0;}}`,
		"condition case":       `switch(YES){case ok{return value.n;}default{return 0;}}`,
		"condition false case": `switch(NO){case !ok{return value.n;}default{return 0;}}`,
		"condition default":    `switch(true){case !ok{return 0;}default{return value.n;}}`,
		"short circuit":        `return (ok===NO)||value.n>0;`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result := "int"
			if name == "short circuit" {
				result = "boolean"
			}
			input := prelude + `function f(m:Map<int,C>):` + result + `{const [value,ok]=m[0];` + body + `}`
			if got := checkSource(t, input); len(got) != 0 {
				t.Fatalf("rejected: %v", got)
			}
		})
	}
}

func TestRejectsUnprovenPresenceGuards(t *testing.T) {
	t.Parallel()
	prelude := `class C{public n:int=7;}const YES=true;const NO=false;`
	for name, body := range map[string]string{
		"false case":               `switch(ok){case false{return value.n;}default{return 0;}}`,
		"true default":             `switch(ok){case true{return 0;}default{return value.n;}}`,
		"grouped outcomes":         `switch(ok){case true,false{return value.n;}}return 0;`,
		"fallthrough from failure": `switch(ok){case false{fallthrough;}case true{return value.n;}}return 0;`,
		"copied flag":              `const copy=ok;switch(copy){case true{return value.n;}}return 0;`,
		"reset flag":               `ok=true;switch(ok){case true{return value.n;}}return 0;`,
		"shadowed flag":            `{const ok=true;switch(ok){case true{return value.n;}}}return 0;`,
		"mutable expected truth":   `let truth=true;if(ok===truth){return value.n;}return 0;`,
		"mixed guard group":        `switch(true){case ok,YES{return value.n;}}return 0;`,
		"wrong negated outcome":    `switch(!ok){case YES{return value.n;}default{return 0;}}`,
		"escaping guard":           `const pointer=&ok;switch(ok){case YES{return value.n;}}return 0;`,
		"changed pair in label":    `const reset=():boolean=>{[value,ok]=m[1];return false;};switch(ok){case reset(){return 0;}case true{return value.n;}default{return 0;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			input := prelude + `function f(m:Map<int,C>):int{let [value,ok]=m[0];` + body + `}`
			if got := strings.Join(checkSource(t, input), "\n"); !strings.Contains(got, "may be absent") {
				t.Fatalf("unsafe guard accepted: %s", got)
			}
		})
	}
}

func TestPresenceSwitchProducerKinds(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"generic map":     `function f<T>(m:Map<int,T>,fallback:T):T{const [v,ok]=m[0];switch(ok){case false{return fallback;}default{return v;}}}`,
		"generic receive": `function f<T>(ch:GoChannel<T>,fallback:T):T{const [v,open]=<-ch;switch(!open){case true{return fallback;}default{return v;}}}`,
		"class assertion": `class B{}class D extends B{public n:int=7;}function f(b:B):int{const [v,ok]=b as? D;switch(true){case !ok{return 0;}default{return v.n;}}}`,
		"select receive":  `class C{public n:int=7;}function f(ch:GoChannel<C>):int{select{case const [v,open]=<-ch{switch(open){case true{return v.n;}default{return 0;}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := checkSource(t, input); len(got) != 0 {
				t.Fatalf("rejected: %v", got)
			}
		})
	}
}
