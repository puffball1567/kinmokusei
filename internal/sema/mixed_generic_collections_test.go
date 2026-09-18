package sema

import (
	"strings"
	"testing"
)

func TestMixedGenericCollections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"array lengths", `constraint S=~[2]int|~[3]int;function f<T extends S>(v:T):int{v[0]=v[1];return v[0];}`, ""},
		{"dependent element", `constraint S<E>=~E[]|~[2]E|~*[3]E;function f<E,T extends S<E>>(v:T,e:E):E{const p=&v[0];*p=e;return v[1];}`, ""},
		{"named array pointer", `type Pair=distinct [2]int;constraint S=~int[]|~*Pair;function f<T extends S>(v:T):int{return v[0];}`, ""},
		{"named sequence", `type A=distinct int[];type B=distinct [2]int;constraint S=A|B;function f<T extends S>(v:T):int{return v[0];}`, ""},
		{"string array", `constraint S=~string|~[2]byte|~*[2]byte;function f<T extends S>(v:T):byte{return v[0];}`, ""},
		{"text", `constraint S=~string|~byte[];function f<T extends S>(v:T):T{const b:byte=v[0];return v[1:];}`, ""},
		{"text reversed union", `constraint S=~byte[]|~string;function f<T extends S>(v:T):byte{return v[0];}`, ""},
		{"byte alias", `alias B=byte;constraint S=~string|~B[];function f<T extends S>(v:T):T{return v[:];}`, ""},
		{"class element", `class C{public value:int=1;}constraint S=~C[]|~[2]C;function f<T extends S>(v:T):int{return v[0].value;}`, ""},
		{"nullable element", `class C{public value:int=1;}alias M=C|null;constraint S=~M[]|~[2]M;function f<T extends S>(v:T):int{return v[0].value;}`, "nullable"},
		{"nullable element guard", `class C{public value:int=1;}alias M=C|null;constraint S=~M[]|~[2]M;function f<T extends S>(v:T):int{const item=v[0];if(item!==null){return item.value;}return 0;}`, ""},
		{"nullable disagreement", `class C{}alias M=C|null;constraint S=~C[]|~[2]M;function f<T extends S>(v:T):C{return v[0];}`, "cannot be indexed"},
		{"named nullable disagreement", `class C{}alias M=C|null;type A=distinct C[];type B=distinct M[];constraint S=A|B;function f<T extends S>(v:T):C{return v[0];}`, "cannot be indexed"},
		{"nullable call", `class C{}alias M=C|null;constraint S=~C[]|~[2]C;function f<T extends S>(v:T):C{return v[0];}function bad(v:M[]):C{return f(v);}`, "nullable type information"},
		{"nullable array call", `class C{}alias M=C|null;constraint S=~C[]|~[2]C;function f<T extends S>(v:T):C{return v[0];}function bad(v:[2]M):C{return f(v);}`, "nullable type information"},
		{"nullable forwarding", `class C{}alias M=C|null;constraint S=~C[]|~[2]C;constraint U=~M[]|~[2]M;function f<T extends S>(v:T):C{return v[0];}function bad<T extends U>(v:T):C{return f(v);}`, "nullable type information"},
		{"nullable method specialization", `class C{}alias M=C|null;constraint S<E>=~E[]|~[2]E;class A<E>{public function first<T extends S<E>>(v:T):E{return v[0];}}function f(v:M[]):M{return new A<M>().first<M[]>(v);}`, ""},
		{"nullable specialized argument", `class C{}alias M=C|null;constraint S<E>=~E[]|~[2]E;class A<E>{public function first<T extends S<E>>(v:T):E{return v[0];}}function bad(v:M[]):C{return new A<C>().first<M[]>(v);}`, "nullable type information"},
		{"nested nullable call", `alias M=int[]|null;constraint S=~int[][]|~[2]int[];function f<T extends S>(v:T):int[]{return v[0];}function bad(v:M[]):int[]{return f(v);}`, "nullable type information"},
		{"object nullable call", `class C{}alias A={item:C};alias B={item:C|null};constraint S=~A[]|~[2]A;function f<T extends S>(v:T):C{return v[0].item;}function bad(v:B[]):C{return f(v);}`, "nullable type information"},
		{"common object nullable call", `class C{}alias A={item:C};alias B={item:C|null};constraint S=~A[];function f<T extends S>(v:T):C{return v[0].item;}function bad(v:B[]):C{return f(v);}`, "nullable type information"},
		{"object nullable element", `class C{}alias A={item:C};alias B={item:C|null};constraint S=~A[]|~[2]B;function f<T extends S>(v:T):C{return v[0].item;}`, "cannot be indexed"},
		{"object nullable preserved", `class C{}alias A={item:C|null};constraint S=~A[]|~[2]A;function f<T extends S>(v:T):C{return v[0].item;}`, "cannot use C | null as C"},
		{"dependent nullable substitution", `class C{}alias M=C|null;constraint S<E>=~E[]|~[2]E;function f<E,T extends S<E>>(v:T):E{return v[0];}function bad(v:M[]):C{return f<C,M[]>(v);}`, "nullable type information"},
		{"different elements", `constraint S=~int[]|~[2]string;function f<T extends S>(v:T):int{return v[0];}`, "cannot be indexed"},
		{"nominal elements", `type N=distinct int;constraint S=~int[]|~[2]N;function f<T extends S>(v:T):int{return v[0];}`, "cannot be indexed"},
		{"map sequence", `constraint S=~Map<int,int>|~int[];function f<T extends S>(v:T):int{return v[0];}`, "cannot be indexed"},
		{"map values", `constraint S=~Map<int,int>|~Map<int,string>;function f<T extends S>(v:T):int{return v[0];}`, "cannot be indexed"},
		{"non sequence", `constraint S=~int[]|~int;function f<T extends S>(v:T):int{return v[0];}`, "cannot be indexed"},
		{"array minimum bound", `constraint S=~[3]int|~[2]int;function f<T extends S>(v:T):int{return v[2];}`, "out of bounds"},
		{"slice array bound", `constraint S=~int[]|~[2]int;function f<T extends S>(v:T):int{return v[2];}`, "out of bounds"},
		{"negative", `constraint S=~int[]|~[2]int;function f<T extends S>(v:T):int{return v[-1];}`, "negative"},
		{"fractional", `constraint S=~int[]|~[2]int;function f<T extends S>(v:T):int{return v[0.5];}`, "integer"},
		{"string write", `constraint S=~string|~byte[];function f<T extends S>(v:T):void{v[0]=1;}`, "not assignable"},
		{"string address", `constraint S=~string|~byte[];function f<T extends S>(v:T):*byte{return &v[0];}`, "addressable"},
		{"checked sequence", `constraint S=~int[]|~[2]int;function f<T extends S>(v:T):void{const [x,ok]=v[0];}`, "requires a map"},
		{"array temporary", `constraint S=~int[]|~[2]int;function f<T extends S>(v:()=>T):void{v()[0]=1;}`, "not assignable"},
		{"array temporary address", `constraint S=~int[]|~[2]int;function f<T extends S>(v:()=>T):*int{return &v()[0];}`, "addressable"},
		{"pointer temporary", `constraint S=~int[]|~*[2]int;function f<T extends S>(v:()=>T):void{v()[0]=1;}`, ""},
		{"full text slice", `constraint S=~string|~byte[];function f<T extends S>(v:T):T{return v[0:1:2];}`, "3-index slice"},
		{"text slice order", `constraint S=~string|~byte[];function f<T extends S>(v:T):T{return v[2:1];}`, "low exceeds high"},
		{"nominal byte", `type B=distinct byte;constraint S=~string|~B[];function f<T extends S>(v:T):T{return v[:];}`, "cannot be sliced"},
		{"mixed array slice", `constraint S=~int[]|~[2]int;function f<T extends S>(v:T):int[]{return v[:];}`, "cannot be sliced"},
		{"different array slice", `constraint S=~[3]int|~[2]int;function f<T extends S>(v:T):int[]{return v[:];}`, "cannot be sliced"},
		{"comparable filtering", `constraint Both=~int[]|~[2]int|~[3]int;constraint S=comparable&Both;function f<T extends S>(v:T):int{return v[0];}`, ""},
		{"global arrow", `class C{public value:int=1;}constraint S=~C[]|~[2]C;function first<T extends S>(v:T):C{return v[0];}const read=():C=>first([new C()]);const value=read();`, ""},
		{"global nullable arrow", `class C{}alias M=C|null;constraint S=~C[]|~[2]C;function first<T extends S>(v:T):C{return v[0];}const read=(v:M[]):C=>first(v);`, "nullable type information"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
