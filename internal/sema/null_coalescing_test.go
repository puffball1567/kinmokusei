package sema

import (
	"strings"
	"testing"
)

func TestNullCoalescingTypesAndFlow(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"class":                          `class C{public n:int=1;}function f(v:C|null):int{return (v??new C()).n;}`,
		"contextual upcast":              `class C{}class D extends C{}function f(v:D|null):C{return v??new C();}`,
		"nullable fallback":              `class C{}function f(v:C|null):C|null{return v??null;}`,
		"two nullable":                   `class C{}function f(v:C|null,w:C|null):C|null{return v??w;}`,
		"raw Go pointer":                 `import go flag from "flag";function f(v:*flag.Flag):*flag.Flag{return v??newFlag();}function newFlag():*flag.Flag{let value=flag.Flag{};return &value;}`,
		"pointer proof":                  `function f(v:*int|null):int{let n=3;const p:*int|null=v??&n;return *p;}`,
		"function":                       `alias F=()=>int;function f(v:F|null):int{return (v??(()=>3))();}`,
		"contextual empty slice":         `function f(v:byte[]|null):byte[]{return v??[];}`,
		"contextual slice elements":      `function f(v:byte[]|null,n:uint):byte[]{return v??[1<<n];}`,
		"generic pointer":                `constraint P=~*int;function f<T extends P>(v:T|null,w:T):T{return v??w;}`,
		"scalars after null":             `function f(n:uint):byte{return null??(1<<n);}`,
		"literal context":                `class C{}const value:C|null=null??null;`,
		"conditional effect keeps proof": `class C{public n:int=1;}function f(value:C|null):int{const required=value??new C();return required.n;}`,
		"constant null await":            `class C{}function work():C{return new C();}function f():C{const absent:C|null=null;const task=go work();return absent??await task;}`,
		"literal null await":             `class C{}function work():C{return new C();}function f():C{const task=go work();return null??await task;}`,
		"skipped await stays pending":    `class C{}function work():C{return new C();}function f():C{const task=go work();const value=new C()??await task;detach task;return value;}`,
		"skipped mutation keeps proof":   `class C{public n:int=1;}class H{constructor(public v:C|null){}}function reset(h:H):C{h.v=null;return new C();}function f(h:H):int{if(h.v===null){return 0;}const chosen=new C()??reset(h);return h.v.n;}`,
		"immutable alias shadowing":      `class C{}function work():C{return new C();}function f():C{const first:C|null=null;const alias=first;const task=go work();{const first=new C();return alias??await task;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("coalescing rejected: %v", diagnostics)
			}
		})
	}
}

func TestRejectsInvalidNullCoalescing(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ input, want string }{
		"scalar left":                 {`const n=1??2;`, "left operand"},
		"mismatched fallback":         {`class C{}class D{}function f(v:C|null):C{return v??new D();}`, "cannot use"},
		"nullable leak":               {`class C{}function f(v:C|null):C{return v??null;}`, "cannot use"},
		"raw nil native":              {`class C{}function f(v:C|null):C|null{return v??nil;}`, "cannot use nil"},
		"no concrete type":            {`const v=null??null;`, "cannot infer"},
		"void fallback":               {`class C{}function nothing():void{}function f(v:C|null):void{const x=v??nothing();}`, "ordinary value"},
		"unconsumed conditional task": {`class C{}function work():C{return new C();}function f(v:C|null):C{const task=go work();return v??await task;}`, "Task"},
		"original not narrowed":       {`class C{public n:int=1;}function f(v:C|null):int{const chosen=v??new C();return v.n;}`, "nullable"},
		"conditional mutation":        {`class C{public n:int=1;}class H{constructor(public v:C|null){}}function reset(h:H):C{h.v=null;return new C();}function f(h:H,p:C|null):int{if(h.v===null){return 0;}const chosen=p??reset(h);return h.v.n;}`, "nullable"},
		"shift representability":      {`function f(n:uint):byte{return null??(300<<n);}`, "300"},
		"default floating shift":      {`function f(n:uint):void{const v=null??(1.0<<n);}`, "shift"},
		"not a constant array length": {`function f(p:*[3]int):void{const xs:[len(p??p)]int=[1,2,3];}`, "constant"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			diagnostics := strings.Join(checkSource(t, test.input), "\n")
			if !strings.Contains(diagnostics, test.want) {
				t.Fatalf("diagnostics=%q want=%q", diagnostics, test.want)
			}
		})
	}
}
