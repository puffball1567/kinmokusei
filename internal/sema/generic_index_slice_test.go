package sema

import (
	"strings"
	"testing"
)

func TestGenericIndexAndSlice(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"slice read write", `constraint S<E>=~E[];function f<E,T extends S<E>>(v:T,e:E):E{v[0]=e;return v[0];}`, ""},
		{"named slice union", `type A=distinct int[];type B=distinct int[];constraint S=A|B;function f<T extends S>(v:T):T{return v[1:2:3];}`, ""},
		{"array read slice", `constraint A<E>=~[2]E;function f<E,T extends A<E>>(v:T):E[]{v[0]=v[1];return v[:];}`, ""},
		{"array pointer", `constraint A<E>=~*[2]E;function f<E,T extends A<E>>(v:T):E[]{v[0]=v[1];return v[:];}`, ""},
		{"map checked", `constraint M<E>=~Map<string,E>;function f<E,T extends M<E>>(v:T,e:E):E{v["a"]=e;const [value,present]=v["a"];return value;}`, ""},
		{"map dependent key", `constraint M<K extends comparable,E>=~Map<K,E>;function f<K extends comparable,E,T extends M<K,E>>(v:T,k:K):E{return v[k];}`, ""},
		{"string", `constraint S=~string;function f<T extends S>(v:T):T{const b:byte=v[0];return v[1:];}`, ""},
		{"nullable root", `constraint S=~int[];function f<T extends S>(v:T|null):T{if(v!==null){return v[:];}return T(nil);}`, ""},
		{"source class", `class C{public value:int=1;}constraint S=~C[];function f<T extends S>(v:T):int{return v[:][0].value;}`, ""},
		{"nullable element", `class C{public value:int=1;}alias Maybe=C|null;constraint S=~Maybe[];function f<T extends S>(v:T):int{return v[0].value;}`, "nullable"},
		{"nullable reslice", `class C{public value:int=1;}alias Maybe=C|null;constraint S=~Maybe[];function f<T extends S>(v:T):int{return v[:][0].value;}`, "nullable"},
		{"nullable map", `class C{public value:int=1;}constraint M=~Map<string,C|null>;function f<T extends M>(v:T):int{const [value,ok]=v["a"];return value.value;}`, "nullable"},
		{"index bounds", `constraint A=~[2]int;function f<T extends A>(v:T):int{return v[2];}`, "out of bounds"},
		{"pointer index bounds", `constraint A=~*[2]int;function f<T extends A>(v:T):int{return v[-1];}`, "negative"},
		{"slice bounds", `constraint A=~[2]int;function f<T extends A>(v:T):int[]{return v[:3];}`, "exceeds fixed array length"},
		{"slice reversed", `constraint S=~int[];function f<T extends S>(v:T):T{return v[2:1];}`, "low exceeds high"},
		{"slice full bounds", `constraint S=~int[];function f<T extends S>(v:T):T{return v[0:2:1];}`, "high exceeds max"},
		{"string full slice", `constraint S=~string;function f<T extends S>(v:T):T{return v[0:1:2];}`, "3-index slice"},
		{"string write", `constraint S=~string;function f<T extends S>(v:T):void{v[0]=1;}`, "not assignable"},
		{"string address", `constraint S=~string;function f<T extends S>(v:T):*byte{return &v[0];}`, "addressable"},
		{"map address", `constraint S=~Map<int,int>;function f<T extends S>(v:T):*int{return &v[0];}`, "addressable"},
		{"map slice", `constraint S=~Map<int,int>;function f<T extends S>(v:T):T{return v[:];}`, "cannot be sliced"},
		{"checked slice", `constraint S=~int[];function f<T extends S>(v:T):void{const [a,b]=v[0];}`, "requires a map"},
		{"array temporary", `constraint A=~[2]int;function f<T extends A>(v:()=>T):int[]{return v()[:];}`, "addressable"},
		{"array temporary write", `constraint A=~[2]int;function f<T extends A>(v:()=>T):void{v()[0]=1;}`, "not assignable"},
		{"unconstrained", `function f<T>(v:T):int{return v[0];}`, "cannot be indexed"},
		{"mixed index shapes", `constraint S=~int[]|~[2]int;function f<T extends S>(v:T):int{return v[0];}`, "cannot be indexed"},
		{"mixed slice shapes", `constraint S=~byte[]|~string;function f<T extends S>(v:T):T{return v[:];}`, "cannot be sliced"},
		{"wrong element", `constraint S=~int[];function f<T extends S>(v:T):void{v[0]="x";}`, "cannot use string as int"},
		{"wrong key", `constraint S=~Map<string,int>;function f<T extends S>(v:T):int{return v[0];}`, "cannot use"},
		{"nullable root index", `constraint S=~int[];function f<T extends S>(v:T|null):int{return v[0];}`, "nullable"},
		{"nullable root slice", `constraint S=~int[];function f<T extends S>(v:T|null):T{return v[:];}`, "nullable"},
		{"guarded root index", `constraint S=~int[];function f<T extends S>(v:T|null):int{if(v!==null){return v[0];}return 0;}`, ""},
		{"fractional index", `constraint S=~int[];function f<T extends S>(v:T):int{return v[1.5];}`, "integer"},
		{"fractional slice", `constraint S=~int[];function f<T extends S>(v:T):T{return v[:1.5];}`, "integer"},
		{"map byte overflow", `constraint S=~Map<byte,int>;function f<T extends S>(v:T):int{return v[256];}`, "overflows"},
		{"ordinary map overflow", `function f(v:Map<byte,int>):int{return v[256];}`, "overflows"},
		{"named map overflow", `type M=distinct Map<byte,int>;function f(v:M):int{return v[256];}`, "overflows"},
		{"map constant fraction", `constraint S=~Map<int,int>;function f<T extends S>(v:T):int{return v[1.5];}`, "cannot"},
		{"map byte constant alias", `const tooBig=256;constraint S=~Map<byte,int>;function f<T extends S>(v:T):void{v[tooBig]=1;}`, "overflows"},
		{"pointer class", `class C{public value:int=1;}constraint S=~*[2]C;function f<T extends S>(v:T):int{return v[:][0].value;}`, ""},
		{"mixed nullable shapes", `class C{}alias Maybe=C|null;type A<E>=distinct E[];type B<E>=distinct E[];constraint S=A<C>|B<Maybe>;function f<T extends S>(v:T):C{return v[0];}`, "cannot yet be used"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
