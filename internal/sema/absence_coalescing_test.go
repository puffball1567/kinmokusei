package sema

import (
	"strings"
	"testing"
)

func TestAbsenceCoalescingTypes(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"map integer":                 `function f(m:Map<int,int>):int{return m[0]??9;}`,
		"receive integer":             `function f(ch:GoReceiveChannel<int>):int{return (<-ch)??9;}`,
		"map record":                  `class C{}struct Row{public c:C;}function f(m:Map<int,Row>):Row{return m[0]??Row{c:new C()};}`,
		"receive record":              `class C{}struct Row{public c:C;}function f(ch:GoChannel<Row>,fallback:Row):Row{return (<-ch)??fallback;}`,
		"array payload":               `class C{}alias Pair=[2]C;function f(m:Map<int,Pair>,fallback:Pair):Pair{return m[0]??fallback;}`,
		"generic value payload":       `constraint N=~int|~int64;function f<T extends N>(m:Map<int,T>,fallback:T):T{return m[0]??fallback;}`,
		"generic receive":             `constraint N=~int|~int64;function f<T extends N>(ch:GoChannel<T>,fallback:T):T{return (<-ch)??fallback;}`,
		"nested value defaults":       `function f(a:Map<int,int>,b:Map<int,int>):int{return a[0]??(b[0]??9);}`,
		"nilable payload":             `class C{}function f(m:Map<int,C>):C{return m[0]??new C();}`,
		"contextual shift":            `function f(m:Map<int,byte>,n:uint):byte{return m[0]??(1<<n);}`,
		"ordinary sequence reference": `class C{}function f(xs:C[]):C{return xs[0]??new C();}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := checkSource(t, input); len(got) != 0 {
				t.Fatalf("rejected: %v", got)
			}
		})
	}
}

func TestRejectsInvalidAbsenceCoalescing(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ input, want string }{
		"unknown map payload":       {`function f<T>(m:Map<int,T>,fallback:T):T{return m[0]??fallback;}`, "uniform nil or value"},
		"unknown receive payload":   {`function f<T>(ch:GoChannel<T>,fallback:T):T{return (<-ch)??fallback;}`, "uniform nil or value"},
		"mixed map payload":         {`constraint C=~int|~*int;function f<T extends C>(m:Map<int,T>,fallback:T):T{return m[0]??fallback;}`, "uniform nil or value"},
		"wrong fallback":            {`function f(m:Map<int,int>):int{return m[0]??true;}`, "cannot use"},
		"null fallback":             {`function f(m:Map<int,int>):int{return m[0]??null;}`, "cannot use"},
		"sequence scalar":           {`function f(xs:int[]):int{return xs[0]??9;}`, "left operand"},
		"value chain needs nesting": {`function f(a:Map<int,int>,b:Map<int,int>):int{return a[0]??b[0]??9;}`, "left operand"},
		"nullable map":              {`function f(m:Map<int,int>|null):int{return m[0]??9;}`, "before indexing"},
		"nullable channel":          {`function f(ch:GoChannel<int>|null):int{return (<-ch)??9;}`, "nullable"},
		"send-only channel":         {`function f(ch:GoSendChannel<int>):int{return (<-ch)??9;}`, "send-only"},
		"shift overflow":            {`function f(m:Map<int,byte>,n:uint):byte{return m[0]??(300<<n);}`, "300"},
		"conditional task":          {`function work():int{return 1;}function f(m:Map<int,int>):int{const task=go work();return m[0]??await task;}`, "Task"},
		"conditional mutation":      {`class C{public n:int=1;}class H{constructor(public v:C|null){}}function reset(h:H):int{h.v=null;return 0;}function f(h:H,m:Map<int,int>):int{if(h.v===null){return 0;}const n=m[0]??reset(h);return h.v.n;}`, "nullable"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(checkSource(t, test.input), "\n")
			if !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%q want=%q", got, test.want)
			}
		})
	}
}
