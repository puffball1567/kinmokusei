package sema

import (
	"strings"
	"testing"
)

func TestUnsafeCollectionsRequireExplicitPolicy(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`import go u from "unsafe";constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):*E{return u.SliceData(v);}`,
		`import go {Slice} from "unsafe";constraint P<E>=~*E;function f<E,T extends P<E>>(v:T):E[]{return Slice(v,1.0);}`,
	} {
		if diagnostics := checkSource(t, source); !strings.Contains(strings.Join(diagnostics, "\n"), "requires [go.interop]") {
			t.Fatalf("missing unsafe policy diagnostic: %v", diagnostics)
		}
	}
}

func TestUnsafeCollectionContexts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"integer float constant", `function f(p:*byte):byte[]{return u.Slice(p,2.0);}`, ""},
		{"integer complex constant", `function f(p:*byte):byte[]{return u.Slice(p,complex(2,0));}`, ""},
		{"constant alias", `const a=2.0;const b=a;function f(p:*byte):string{return u.String(p,b);}`, ""},
		{"negative alias", `const a=-2;const b=a;function f(p:*byte):byte[]{return u.Slice(p,b);}`, "cannot be negative"},
		{"negative imported", `import go io from "io";function f(p:*byte):string{return u.String(p,io.SeekStart-1);}`, "cannot be negative"},
		{"constant overflow", `const a=9223372036854775808;function f(p:u.Pointer):u.Pointer{return u.Add(p,a);}`, "out of range"},
		{"negative offset allowed", `const a=-1.0;function f(p:u.Pointer):u.Pointer{return u.Add(p,a);}`, ""},
		{"shift count context", `function f(p:*byte,n:uint):byte[]{return u.Slice(p,1.0<<n);}`, ""},
		{"fraction", `function f(p:*byte):string{return u.String(p,1.5);}`, "must be an integer"},
		{"runtime float", `function f(p:*byte,n:float):byte[]{return u.Slice(p,n);}`, "must be an integer"},
		{"runtime binding not constant", `function f(p:*byte):byte[]{let n=2.0;const a=n;return u.Slice(p,a);}`, "must be an integer"},
		{"generic count", `constraint N=~int|~uint64;function f<T extends N>(p:*byte,n:T):byte[]{return u.Slice(p,n);}`, ""},
		{"generic slice data", `constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):*E{return u.SliceData(v);}`, ""},
		{"generic pointer slice", `constraint P<E>=~*E;function f<E,T extends P<E>>(p:T,n:int):E[]{return u.Slice(p,n);}`, ""},
		{"generic pointer union", `type A=distinct *int;type B=distinct *int;constraint P=A|B;function f<T extends P>(p:T):int[]{return u.Slice(p,1);}`, ""},
		{"mixed slice element", `constraint S=~int[]|~string[];function f<T extends S>(v:T):void{const p=u.SliceData(v);}`, "must be a slice"},
		{"mixed pointer element", `constraint P=~*int|~*string;function f<T extends P>(v:T):void{const p=u.Slice(v,1);}`, "must be a typed Go pointer"},
		{"unconstrained slice", `function f<T>(v:T):void{const p=u.SliceData(v);}`, "must be a slice"},
		{"unconstrained pointer", `function f<T>(v:T):void{const p=u.Slice(v,1);}`, "must be a typed Go pointer"},
		{"class slice data", `class C{public value:int=1;}function f(v:C[]):int{const p=u.SliceData(v);return (*p).value;}`, ""},
		{"class round trip", `class C{public value:int=1;}function f(v:C[]):int{const p=u.SliceData(v);const xs=u.Slice(p,1);return xs[0].value;}`, ""},
		{"nullable class preserved", `class C{public value:int=1;}alias M=C|null;function f(v:M[]):int{const p=u.SliceData(v);const xs=u.Slice(p,1);return xs[0].value;}`, "nullable"},
		{"generic nullable preserved", `class C{public value:int=1;}alias M=C|null;constraint S=~M[];function f<T extends S>(v:T):int{const p=u.SliceData(v);return (*p).value;}`, "nullable"},
		{"nullable slice allowed", `function f(v:int[]|null):*int{return u.SliceData(v);}`, ""},
		{"named builtin", `import go {SliceData} from "unsafe";constraint S=~int[];function f<T extends S>(v:T):*int{return SliceData(v);}`, ""},
		{"conflicting source elements", `class C{}alias M=C|null;type A=distinct C[];type B=distinct M[];constraint S=A|B;function f<T extends S>(v:T):void{const p=u.SliceData(v);}`, "must be a slice"},
		{"typed unsigned overflow", `import go math from "math";function f(p:*byte):byte[]{return u.Slice(p,math.MaxUint64);}`, "out of range"},
		{"typed float constant", `const n:float=2;function f(p:*byte):byte[]{return u.Slice(p,n);}`, "must be an integer"},
		{"integer constructor constant", `const n=min(-1,2);function f(p:*byte):string{return u.String(p,n);}`, "cannot be negative"},
		{"named import shadow", `import go {Slice} from "unsafe";function f():int{const Slice=(n:int):int=>n;return Slice(2);}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSourceWithPolicy(t, `import go u from "unsafe";`+test.source, GoInteropPolicy{AllowUnsafe: true}), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
