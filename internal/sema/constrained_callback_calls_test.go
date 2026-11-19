package sema

import (
	"strings"
	"testing"
)

func TestConstrainedCallbackCalls(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, source, want string }{
		{"dependent callback", `alias Callback<E>=(v:E)=>E;constraint C<E>=~Callback<E>;function run<E,F extends C<E>>(fn:F,v:E):E{return fn(v);}`, ""},
		{"named union", `type A=distinct (v:int)=>int;type B=distinct (other:int)=>int;constraint C=A|B;function run<F extends C>(fn:F):int{return fn(7);}`, ""},
		{"nullable parameter and result", `class Item{}alias Callback=(v:Item|null)=>Item|null;constraint C=~Callback;function run<F extends C>(fn:F):Item|null{return fn(null);}`, ""},
		{"nullable callback must be narrowed", `alias Callback=(v:int)=>int;constraint C=~Callback;function run<F extends C>(fn:F|null):int{if(fn===null){return 0;}return fn(7);}`, ""},
		{"nullable callback rejected", `alias Callback=(v:int)=>int;constraint C=~Callback;function run<F extends C>(fn:F|null):int{return fn(7);}`, "must be checked against null"},
		{"wrong argument", `alias Callback=(v:string)=>int;constraint C=~Callback;function run<F extends C>(fn:F):int{return fn(b"raw");}`, "cannot use bstring as string"},
		{"multiple results", `constraint C=~(v:int)=>(int,string);function run<F extends C>(fn:F):(int,string){return fn(7);}`, ""},
		{"Result forwards", `constraint C=~(v:int)=>Result<int>;function run<F extends C>(fn:F):Result<int>{return fn(7);}`, ""},
		{"Result handled", `constraint C=~(v:int)=>Result<int>;function run<F extends C>(fn:F):Result<int>{const n=fn(7)?;return ok(n+1);}`, ""},
		{"Result discarded", `constraint C=~(v:int)=>Result<int>;function run<F extends C>(fn:F):void{fn(7);}`, "Result"},
		{"raw pair as Result callback", `constraint C=~(v:int)=>Result<int>;function run<F extends C>(fn:F):Result<int>{return fn(7);}function bad(fn:(v:int)=>(int,error)):Result<int>{return run(fn);}`, "type parameter constraint"},
		{"Result as raw pair callback", `constraint C=~(v:int)=>(int,error);function run<F extends C>(fn:F):(int,error){return fn(7);}function bad(fn:(v:int)=>Result<int>):(int,error){return run(fn);}`, "type parameter constraint"},
		{"Result intersection conflict", `constraint A=~()=>Result<int>;constraint B=~()=>(int,error);constraint C=A&B;`, "incompatible"},
		{"variadic spread", `constraint C=~(...v:int[])=>int;function run<F extends C>(fn:F,vs:int[]):int{return fn(vs...);}`, ""},
		{"sole multiple-result argument", `constraint C=~(a:int,b:int)=>int;function pair():(int,int){return 1,2;}function run<F extends C>(fn:F):int{return fn(pair());}`, ""},
		{"mixed shape", `alias Callback=()=>int;constraint C=~Callback|~int;function run<F extends C>(fn:F):int{return fn();}`, "requires a common function signature"},
		{"mixed signatures", `alias One=(v:int)=>int;alias Two=(v:int,n:int)=>int;constraint C=~One|~Two;function run<F extends C>(fn:F):int{return fn(7);}`, "requires a common function signature"},
		{"mixed source text contracts", `type A=distinct (v:string)=>int;type B=distinct (v:bstring)=>int;constraint C=A|B;function run<F extends C>(fn:F):int{return fn("hi");}`, "matching source contracts"},
		{"mixed source Result contracts", `type A=distinct ()=>Result<int>;type B=distinct ()=>(int,error);constraint C=A|B;function run<F extends C>(fn:F):Result<int>{return fn();}`, "matching source contracts"},
		{"unrestricted", `function run<F>(fn:F):void{fn();}`, "requires a common function signature"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			messages := strings.Join(checkSource(t, tt.source), "\n")
			if tt.want == "" && messages != "" || tt.want != "" && !strings.Contains(messages, tt.want) {
				t.Fatalf("diagnostics=%q want=%q", messages, tt.want)
			}
		})
	}
}
