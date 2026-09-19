package sema

import (
	"strings"
	"testing"
)

func TestGenericArrayConversions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"copy generic", `constraint S=~int[];function f<T extends S>(v:T):[2]int{return copyArray[[2]int](v);}`, ""},
		{"view generic", `constraint S=~int[];function f<T extends S>(v:T):*[2]int{return viewArray[[2]int](v);}`, ""},
		{"dependent copy", `constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):[2]E{return copyArray[[2]E](v);}`, ""},
		{"dependent view", `constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):*[2]E{return viewArray[[2]E](v);}`, ""},
		{"named source union", `type A=distinct int[];type B=distinct int[];constraint S=A|B;function f<T extends S>(v:T):[2]int{return copyArray[[2]int](v);}`, ""},
		{"named target", `type A<E>=distinct [2]E;constraint S<E>=~E[];function f<E,T extends S<E>>(v:T):A<E>{return copyArray[A<E>](v);}`, ""},
		{"nullable source", `constraint S=~int[];function f<T extends S>(v:T|null):[0]int{return copyArray[[0]int](v);}`, ""},
		{"mixed elements", `constraint S=~int[]|~string[];function f<T extends S>(v:T):[2]int{return copyArray[[2]int](v);}`, "requires a slice source"},
		{"mixed shape", `constraint S=~int[]|~[2]int;function f<T extends S>(v:T):*[2]int{return viewArray[[2]int](v);}`, "requires a slice source"},
		{"unconstrained", `function f<T>(v:T):[2]int{return copyArray[[2]int](v);}`, "requires a slice source"},
		{"wrong element", `constraint S=~string[];function f<T extends S>(v:T):*[2]int{return viewArray[[2]int](v);}`, "cannot convert slice"},
		{"distinct element", `type N=distinct int;constraint S=~N[];function f<T extends S>(v:T):[2]int{return copyArray[[2]int](v);}`, "cannot convert slice"},
		{"copy source class", `class C{public value:int=1;}function f(v:C[]):int{const a=copyArray[[1]C](v);return a[0].value;}`, ""},
		{"specialized copy class", `class C{public value:int=1;}function pair<E>(v:E[]):[1]E{return copyArray[[1]E](v);}function f(v:C[]):int{const a=pair(v);return a[0].value;}`, ""},
		{"specialized view class", `class C{public value:int=1;}function pair<E>(v:E[]):*[1]E{return viewArray[[1]E](v);}function f(v:C[]):int{const a=pair(v);const s=a[:];return s[0].value+len(a)+cap(a);}`, ""},
		{"specialized nullable view", `class C{public value:int=1;}alias Maybe=C|null;function pair<E>(v:E[]):*[1]E{return viewArray[[1]E](v);}function f(v:Maybe[]):int{return pair(v)[0].value;}`, "nullable"},
		{"specialized nullable slice", `class C{public value:int=1;}alias Maybe=C|null;function pair<E>(v:E[]):[1]E{return copyArray[[1]E](v);}function f(v:Maybe[]):int{const a=pair(v);const s=a[:];return s[0].value;}`, "nullable"},
		{"specialized constant length", `class C{}function pair<E>(v:E[]):*[1]E{return viewArray[[1]E](v);}function f(v:C[]):void{const a=pair(v);const n=cap(a);const p=&n;}`, "addressable"},
		{"specialized index bounds", `class C{}function pair<E>(v:E[]):[1]E{return copyArray[[1]E](v);}function f(v:C[]):C{return pair(v)[1];}`, "out of bounds"},
		{"specialized slice bounds", `class C{}function pair<E>(v:E[]):*[1]E{return viewArray[[1]E](v);}function f(v:C[]):C[]{return pair(v)[:2];}`, "exceeds fixed array length"},
		{"view source class", `class C{public value:int=1;}constraint S=~C[];function f<T extends S>(v:T):int{const a=viewArray[[1]C](v);return a[0].value;}`, ""},
		{"view slice class", `class C{public value:int=1;}function f(v:C[]):int{const a=viewArray[[1]C](v);const s=a[:];return s[0].value;}`, ""},
		{"copy slice class", `class C{public value:int=1;}function f(v:C[]):int{const a=copyArray[[1]C](v);const s=a[:];return s[0].value;}`, ""},
		{"view nullable guard", `class C{public value:int=1;}alias Maybe=C|null;function f(v:Maybe[]):int{const a=viewArray[[1]Maybe](v);const e=a[0];if(e!==null){return e.value;}return 0;}`, ""},
		{"view nullable read", `class C{public value:int=1;}alias Maybe=C|null;function f(v:Maybe[]):int{const a=viewArray[[1]Maybe](v);return a[0].value;}`, "nullable"},
		{"view slice nullable read", `class C{public value:int=1;}alias Maybe=C|null;function f(v:Maybe[]):int{const a=viewArray[[1]Maybe](v);const s=a[:];return s[0].value;}`, "nullable"},
		{"copy slice nullable read", `class C{public value:int=1;}alias Maybe=C|null;function f(v:Maybe[]):int{const a=copyArray[[1]Maybe](v);const s=a[:];return s[0].value;}`, "nullable"},
		{"copy narrows nullability", `class C{}alias Maybe=C|null;function f(v:Maybe[]):[1]C{return copyArray[[1]C](v);}`, "cannot convert slice"},
		{"view widens nullability", `class C{}alias Maybe=C|null;function f(v:C[]):*[1]Maybe{return viewArray[[1]Maybe](v);}`, "cannot convert slice"},
		{"nested nullability", `alias Maybe=int[]|null;function f(v:Maybe[]):[1]int[]{return copyArray[[1]int[]](v);}`, "cannot convert slice"},
		{"class covariance", `class Base{}class Child extends Base{}function f(v:Child[]):*[1]Base{return viewArray[[1]Base](v);}`, "cannot convert slice"},
		{"generic index bound", `constraint S=~int[];function f<T extends S>(v:T):int{return viewArray[[2]int](v)[2];}`, "out of bounds"},
		{"generic constant length", `constraint S=~int[];function f<T extends S>(v:T):void{const n=len(copyArray[[2]int](v));const p=&n;}`, "addressable"},
		{"generic short source remains runtime", `constraint S=~int[];function f<T extends S>(v:T):[9]int{return copyArray[[9]int](v);}`, ""},
		{"parameter target", `constraint A=~[2]int;function f<T extends A>(v:int[]):T{return copyArray<T>(v);}`, ""},
		{"parameter lengths", `constraint A<E>=~[0]E|~[2]E|~[3]E;function f<E,T extends A<E>>(v:E[]):T{return copyArray<T>(v);}`, ""},
		{"parameter source and target", `constraint A<E>=~[2]E;constraint S<E>=~E[];function f<E,T extends A<E>,U extends S<E>>(v:U):T{return copyArray<T>(v);}`, ""},
		{"parameter named targets", `type A=distinct [2]int;type B=distinct [3]int;constraint C=A|B;function f<T extends C>(v:int[]):T{return copyArray<T>(v);}`, ""},
		{"parameter method", `constraint A<E>=~[2]E;class Copier<E>{public function copy<T extends A<E>>(v:E[]):T{return copyArray<T>(v);}}`, ""},
		{"parameter view remains unsupported", `constraint A=~[2]int;function f<T extends A>(v:int[]):*T{return viewArray<T>(v);}`, "target must be a fixed array type"},
		{"parameter mixed shape", `constraint A=~[2]int|~int[];function f<T extends A>(v:int[]):T{return copyArray<T>(v);}`, "target must be a fixed array type"},
		{"parameter mixed element", `constraint A=~[2]int|~[3]string;function f<T extends A>(v:int[]):T{return copyArray<T>(v);}`, "target must be a fixed array type"},
		{"parameter unconstrained target", `function f<T>(v:int[]):T{return copyArray<T>(v);}`, "target must be a fixed array type"},
		{"parameter nullable element mismatch", `class C{}alias Maybe=C|null;constraint A=~[1]C|~[2]Maybe;function f<T extends A>(v:C[]):T{return copyArray<T>(v);}`, "target must be a fixed array type"},
		{"parameter nullable source mismatch", `class C{}alias Maybe=C|null;constraint A=~[1]C;function f<T extends A>(v:Maybe[]):T{return copyArray<T>(v);}`, "cannot convert slice"},
		{"parameter nullable read", `class C{public value:int=1;}alias Maybe=C|null;constraint A=~[1]Maybe;function f<T extends A>(v:Maybe[]):int{return copyArray<T>(v)[0].value;}`, "nullable"},
		{"parameter nested nullability", `alias Maybe=int[]|null;constraint A=~[1]int[];function f<T extends A>(v:Maybe[]):T{return copyArray<T>(v);}`, "cannot convert slice"},
		{"parameter class covariance", `class Base{}class Child extends Base{}constraint A=~[1]Base;function f<T extends A>(v:Child[]):T{return copyArray<T>(v);}`, "cannot convert slice"},
		{"parameter distinct element", `type N=distinct int;constraint A=~[1]N;function f<T extends A>(v:int[]):T{return copyArray<T>(v);}`, "cannot convert slice"},
		{"parameter nullable reslice", `class C{public value:int=1;}alias Maybe=C|null;constraint A<E>=~[1]E;function f<E,T extends A<E>>(v:E[]):T{return copyArray<T>(v);}function use(v:Maybe[]):int{const a=f<Maybe,[1]Maybe>(v);return a[:][0].value;}`, "nullable"},
		{"parameter zero length not constant", `constraint A=~[0]int;function f<T extends A>(v:int[]):int{const n=len(copyArray<T>(v));const p=&n;return *p;}`, ""},
		{"arity", `constraint S=~int[];function f<T extends S>(v:T):void{viewArray[[2]int](v,v);}`, "expects one slice argument"},
		{"spread", `constraint S=~int[];function f<T extends S>(v:T):void{copyArray[[2]int](v...);}`, "does not accept spread"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
