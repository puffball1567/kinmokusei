package sema

import (
	"strings"
	"testing"
)

func TestConstrainedCallbackCalls(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, source, want string }{
		{"dependent callback", `alias Callback<E>=(v:E)=>E;constraint C<E>=~Callback<E>;function run<E,F extends C<E>>(fn:F,v:E):E{return fn(v);}`, ""},
		{"context from fixed callback bound", `constraint C=~(v:int)=>int;function apply<F extends C>(fn:F):int{return fn(7);}function run():int{return apply((v)=>v+1);}`, ""},
		{"context from dependent callback bound", `constraint C<E,R>=~(v:E)=>R;function apply<E,R,F extends C<E,R>>(fn:F,v:E):R{return fn(v);}function run():string{return apply((v)=>v+"!","text");}`, ""},
		{"context from a callback annotation", `constraint C<E,R>=~(a:E,b:E)=>R;function pair<E,R,F extends C<E,R>>(fn:F,v:E):R{return fn(v,v);}function run():int{return pair((a:int16,b)=>int(a+b),3);}`, ""},
		{"context for generic declaration", `constraint C=~(v:int)=>int;function apply<F extends C>(fn:F):int{return fn(7);}function identity<T>(v:T):T{return v;}function run():int{return apply(identity);}`, ""},
		{"context for variadic arrow", `constraint C=~(...vs:int[])=>int;function apply<F extends C>(fn:F):int{return fn(7,2);}function run():int{return apply((...vs)=>vs[0]+vs[1]);}`, ""},
		{"context preserves nullable Result payload", `class Item{}constraint C=~(item:Item|null)=>Result<Item|null>;function apply<F extends C>(fn:F):Result<Item|null>{return fn(null);}function run():Result<Item|null>{return apply((item)=>{return ok(item);});}`, ""},
		{"context does not guess unknown input", `constraint C<E>=~(v:E)=>E;function apply<E,F extends C<E>>(fn:F):F{return fn;}function run():void{const f=apply((v)=>v);}`, "cannot infer arrow parameter"},
		{"context does not pick an alternative", `alias A=(v:int)=>int;alias B=(v:string)=>int;constraint C=~A|~B;function apply<F extends C>(fn:F):F{return fn;}function run():void{const f=apply((v)=>1);}`, "cannot infer arrow parameter"},
		{"context does not erase source type differences", `type A=distinct (v:string)=>int;type B=distinct (v:bstring)=>int;constraint C=A|B;function apply<F extends C>(fn:F):F{return fn;}function run():void{const f=apply((v)=>1);}`, "cannot infer arrow parameter"},
		{"context still requires type-set membership", `type A=distinct (v:int)=>int;type B=distinct (v:int)=>int;constraint C=A|B;function apply<F extends C>(fn:F):int{return fn(7);}function run():int{return apply((v)=>v+1);}`, "type parameter constraint"},
		{"context with explicit defined type", `type A=distinct (v:int)=>int;type B=distinct (v:int)=>int;constraint C=A|B;function apply<F extends C>(fn:F):int{return fn(7);}function run():int{return apply<A>((v)=>v+1);}`, ""},
		{"dependent verified result", `alias Reader<E>=()=>E;constraint C<E>=~Reader<E>;function read<E,F extends C<E>>(fn:F):E{return fn();}function run(fn:Reader<string>):string{return read(fn);}`, ""},
		{"dependent nullable result", `class Item{}alias Reader<E>=()=>E;constraint C<E>=~Reader<E>;function read<E,F extends C<E>>(fn:F):E{return fn();}function run(fn:Reader<Item|null>):Item|null{return read(fn);}`, ""},
		{"dependent variadic input", `alias Sink<E>=(...values:E[])=>void;constraint C<E>=~Sink<E>;function write<E,F extends C<E>>(fn:F):void{fn();}function run(fn:Sink<string>):void{write(fn);}`, ""},
		{"dependent nullable Result payload", `class Item{}alias Reader=()=>Result<Item|null>;constraint C<E>=~()=>Result<E>;function read<E,F extends C<E>>(fn:F):Result<E>{return fn();}function run(fn:Reader):Result<Item|null>{return read(fn);}`, ""},
		{"dependent generic named callback", `type Reader<T>=distinct ()=>T;constraint C<E>=~()=>E;function read<E,F extends C<E>>(fn:F):E{return fn();}function run(fn:Reader<int>):int{return read(fn);}`, ""},
		{"dependent forwarded callback", `alias Reader<E>=()=>E;constraint C<E>=~Reader<E>;function read<E,F extends C<E>>(fn:F):E{return fn();}function forward<A,G extends C<A>>(fn:G):A{return read(fn);}function run(fn:Reader<string>):string{return forward(fn);}`, ""},
		{"dependent conflicting source arguments", `alias Reader<E>=()=>E;constraint C<E>=~Reader<E>;function read<E,F extends C<E>>(fn:F,value:E):E{return fn();}function run(fn:Reader<string>,raw:bstring):bstring{return read(fn,raw);}`, "type parameter constraint"},
		{"dependent Result versus raw pair", `constraint C<E>=~()=>Result<E>;function read<E,F extends C<E>>(fn:F):Result<E>{return fn();}function run(fn:()=>(int,error)):Result<int>{return read(fn);}`, "type parameter constraint"},
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
