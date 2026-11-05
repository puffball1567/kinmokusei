package sema

import (
	"strings"
	"testing"
)

func TestMapAbsenceAndCheckedPresence(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"nullable read":              `class C{public n:int=1;}function f(m:Map<int,C>):int{const v=m[0];if(v===null){return 0;}return v.n;}`,
		"coalescing":                 `class C{public n:int=1;}function f(m:Map<int,C>):int{return (m[0]??new C()).n;}`,
		"assertion":                  `class C{public n:int=1;}function f(m:Map<int,C>):int{return m[0]!.n;}`,
		"writes preserve type":       `class C{}function f(m:Map<int,C>):void{m[0]=new C();}`,
		"nested maps":                `class C{}function f(m:Map<int,Map<int,C>>):void{m[0][0]=new C();}`,
		"checked class":              `class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];if(ok){return v.n;}return 0;}`,
		"checked struct":             `class C{public n:int=1;}struct R{public v:C;}function f(m:Map<int,R>):int{const [v,ok]=m[0];if(!ok){return 0;}return v.v.n;}`,
		"checked generic":            `function f<T>(m:Map<int,T>,fallback:T):T{const [v,ok]=m[0];if(ok){return v;}return fallback;}`,
		"checked mutable":            `class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];[v,ok]=m[1];if(ok){return v.n;}return 0;}`,
		"guard true equality":        `class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];if(ok===true){return v.n;}return 0;}`,
		"guard false equality":       `class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];if(false!==ok){return v.n;}return 0;}`,
		"short circuit":              `class C{public n:int=1;}function f(m:Map<int,C>):boolean{const [v,ok]=m[0];return ok&&v.n>0;}`,
		"short circuit negative":     `class C{public n:int=1;}function f(m:Map<int,C>):boolean{const [v,ok]=m[0];return !ok||v.n>0;}`,
		"snapshot survives deletion": `class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];if(!ok){return 0;}delete(m,0);return v.n;}`,
		"nullable payload":           `class C{public n:int=1;}function f(m:Map<int,C|null>):int{const [v,ok]=m[0];if(v!==null){return v.n;}return 0;}`,
		"safe zeros":                 `function f(m:Map<int,int>):int{const [v,ok]=m[0];return m[1]+v;}`,
		"reinitialized payload":      `class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];v=new C();return v.n;}`,
		"branch pairs":               `class C{public n:int=1;}function f(m:Map<int,C>,b:boolean):int{let [v,ok]=m[0];if(b){[v,ok]=m[1];}if(ok){return v.n;}return 0;}`,
		"constant capture":           `class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];const run=():int=>{if(ok){return v.n;}return 0;};return run();}`,
		"safe capture snapshot":      `class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];if(!ok){return 0;}const snapshot=v;const run=():int=>snapshot.n;return run();}`,
		"checked callback":           `alias F=()=>int;function f(m:Map<int,F>):int{const [v,ok]=m[0];if(ok){return v();}return 0;}`,
		"checked receive":            `class C{public n:int=1;}function f(ch:GoChannel<C>):int{const [v,ok]=<-ch;if(ok){return v.n;}return 0;}`,
		"checked downcast":           `class B{}class D extends B{public n:int=1;}function f(b:B):int{const [v,ok]=b as? D;if(ok){return v.n;}return 0;}`,
		"bounded pointer read":       `constraint P=~*int;function f<T extends P>(m:Map<int,T>,fallback:T):T{return m[0]??fallback;}`,
		"named pointer read":         `type Links=distinct Map<int,*int>;function f(v:Links):*int|null{return v[0];}`,
		"safe scalar struct":         `struct Row{public n:int;}function f(m:Map<int,Row>):int{return m[0].n;}`,
		"scalar bound":               `constraint Scalar=~int|~string;function f<T extends Scalar>(m:Map<int,T>):T{return m[0];}`,
		"loop condition":             `class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];let n=0;while(ok){n+=v.n;[v,ok]=m[1];}return n;}`,
		"whole assertion discard":    `class B{}class D extends B{}function f(b:B):void{const _=b as? D;}`,
		"whole map discard":          `class C{}struct Row{public v:C;}function f<T>(m:Map<int,T>,rows:Map<int,Row>):void{const _=m[0];const _=rows[0];}`,
		"initialize missing branch":  `class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];if(!ok){v=new C();}return v.n;}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("rejected: %v", diagnostics)
			}
		})
	}
}

func TestRejectsMissingMapValuesAndUnverifiedPayloads(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ input, want string }{
		"mixed value nullability":         {`alias F=()=>int;alias Maybe=F|null;type A=distinct Map<int,F>;type B=distinct Map<int,Maybe>;constraint Maps=A|B;function f<T extends Maps>(m:T):int{const [v,ok]=m[0];if(ok){return v();}return 0;}`, "every constraint alternative"},
		"mixed key nullability":           {`alias Maybe=*int|null;type A=distinct Map<*int,int>;type B=distinct Map<Maybe,int>;constraint Maps=A|B;function f<T extends Maps>(m:T,key:*int):int{return m[key];}`, "every constraint alternative"},
		"direct class":                    {`class C{public n:int=1;}function f(m:Map<int,C>):int{return m[0].n;}`, "nullable"},
		"class return":                    {`class C{}function f(m:Map<int,C>):C{return m[0];}`, "cannot use"},
		"unsafe struct read":              {`class C{}struct R{public v:C;}function f(m:Map<int,R>):R{return m[0];}`, "no safe zero"},
		"unknown generic read":            {`function f<T>(m:Map<int,T>):T{return m[0];}`, "no safe zero"},
		"nil write":                       {`class C{}function f(m:Map<int,C>):void{m[0]=nil;}`, "cannot use nil"},
		"null write":                      {`class C{}function f(m:Map<int,C>):void{m[0]=null;}`, "cannot use null"},
		"unguarded checked class":         {`class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];return v.n;}`, "may be absent"},
		"unguarded struct":                {`class C{}struct R{public v:C;}function f(m:Map<int,R>):R{const [v,ok]=m[0];return v;}`, "may be absent"},
		"discarded guard":                 {`class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,_]=m[0];return v.n;}`, "may be absent"},
		"callback call":                   {`alias F=()=>int;function f(m:Map<int,F>):int{const [v,ok]=m[0];return v();}`, "may be absent"},
		"false guard":                     {`class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];if(!ok){return v.n;}return 0;}`, "may be absent"},
		"guard reset":                     {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];ok=true;if(ok){return v.n;}return 0;}`, "may be absent"},
		"flag alias":                      {`class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];const flag=ok;if(flag){return v.n;}return 0;}`, "may be absent"},
		"old success":                     {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];const saved=ok;[v,ok]=m[1];if(saved){return v.n;}return 0;}`, "may be absent"},
		"different success":               {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];let flag=false;[v,flag]=m[1];if(ok){return v.n;}return 0;}`, "may be absent"},
		"reassignment invalidates proof":  {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];if(!ok){return 0;}[v,ok]=m[1];return v.n;}`, "may be absent"},
		"guard escape":                    {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];const p=&ok;if(ok){return v.n;}return 0;}`, "may be absent"},
		"value escape":                    {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];if(!ok){return 0;}const p=&v;return v.n;}`, "may be absent"},
		"join":                            {`class C{public n:int=1;}function f(m:Map<int,C>,b:boolean):int{const [v,ok]=m[0];if(ok&&b){}return v.n;}`, "may be absent"},
		"mutable capture":                 {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];if(!ok){return 0;}const run=():int=>v.n;return run();}`, "may be absent"},
		"mutable pair in closure":         {`class C{public n:int=1;}function f(m:Map<int,C>):int{let [v,ok]=m[0];const run=():int=>{if(ok){return v.n;}return 0;};ok=true;return run();}`, "may be absent"},
		"global assignment":               {`class C{}let value:C=new C();function f(m:Map<int,C>):void{let ok=false;[value,ok]=m[0];}`, "local payload"},
		"checked receive":                 {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{const [v,ok]=<-ch;return v.n;}`, "may be absent"},
		"checked downcast":                {`class B{}class D extends B{public n:int=1;}function f(b:B):int{const [v,ok]=b as? D;return v.n;}`, "may be absent"},
		"forwarded downcast":              {`class B{}class D extends B{}function forward(b:B):(D,boolean){return b as? D;}`, "destructuring"},
		"forwarded assertion arguments":   {`class B{}class D extends B{}function use(v:D,ok:boolean):void{}function f(b:B):void{use(b as? D);}`, "destructuring"},
		"loop backedge":                   {`class C{public n:int=1;}function f(m:Map<int,C>,b:boolean):int{let [v,ok]=m[0];if(!ok){return 0;}let n=0;while(b){n+=v.n;[v,ok]=m[1];}return n;}`, "may be absent"},
		"branch flag reset":               {`class C{public n:int=1;}function f(m:Map<int,C>,b:boolean):int{let [v,ok]=m[0];if(b){ok=true;}if(ok){return v.n;}return 0;}`, "may be absent"},
		"nullable payload still nullable": {`class C{public n:int=1;}function f(m:Map<int,C|null>):int{const [v,ok]=m[0];if(ok){return v.n;}return 0;}`, "nullable"},
		"assertion cannot replace guard":  {`class C{public n:int=1;}function f(m:Map<int,C>):int{const [v,ok]=m[0];return v!.n;}`, "may be absent"},
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
