package compiler

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/sema"
)

var unsafeCollectionSeeds = []string{
	`import go u from "unsafe";constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):*E{return u.SliceData(v);}`,
	`import go u from "unsafe";constraint P<E>=~*E;function f<E,T extends P<E>>(v:T):E[]{return u.Slice(v,1.0);}`,
	`import go u from "unsafe";function f(p:*byte,n:uint):byte[]{return u.Slice(p,1.0<<n);}`,
	`import go u from "unsafe";const n=-1;function f(p:*byte):byte[]{return u.Slice(p,n);}`,
	`import go u from "unsafe";class C{}alias M=C|null;function f(xs:M[]):void{const v=u.Slice(u.SliceData(xs),len(xs));}`,
	`import go u from "unsafe";constraint P=~*int|~*string;function f<T extends P>(v:T):void{const x=u.Slice(v,1);}`,
	`import go {SliceData} from "unsafe";function f(xs:int[]):*int{return SliceData(xs);}`,
	`import go u from "unsafe";function pair(p:*int):(*int,int){return p,1;}function f(p:*int):int[]{return u.Slice(pair(p));}`,
	`import go u from "unsafe";function pair(p:*byte):(*byte,int){return p,1;}function f(p:*byte):string{return u.String(pair(p));}`,
	`import go u from "unsafe";function pair(p:u.Pointer):(u.Pointer,int){return p,1;}function f(p:u.Pointer):u.Pointer{return u.Add(pair(p));}`,
	`import go u from "unsafe";function pair<T>(p:*T):(*T,int){return p,1;}function f<T>(p:*T):T[]{return u.Slice(pair(p));}`,
	`import go u from "unsafe";function pair(p:*int):(*int,float){return p,1;}function f(p:*int):int[]{return u.Slice(pair(p));}`,
	`import go u from "unsafe";function pair():(int,int,int){return 0,1,2;}function f():void{_=u.Slice(pair());}`,
	`import go u from "unsafe";const advance=(p:u.Pointer)=>u.Add(p,1);`,
}

func TestUnsafeCollectionPipelineProperties(t *testing.T) {
	generated := 0
	for _, seed := range unsafeCollectionSeeds {
		_, reached, err := compilePipelinePropertyWithPolicy(seed, sema.GoInteropPolicy{AllowUnsafe: true})
		if err != nil {
			t.Fatalf("input %s: %v", seed, err)
		}
		if reached {
			generated++
		}
	}
	if generated < 10 {
		t.Fatalf("only %d unsafe seeds reached codegen", generated)
	}
}

func FuzzUnsafeCollectionPipeline(f *testing.F) {
	for _, seed := range unsafeCollectionSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 64<<10 {
			t.Skip()
		}
		if _, _, err := compilePipelinePropertyWithPolicy(input, sema.GoInteropPolicy{AllowUnsafe: true}); err != nil {
			t.Fatalf("input %q: %v", input, err)
		}
	})
}
