package sema

import (
	"strings"
	"testing"
)

func TestGenericSliceBuiltins(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"append empty", `constraint S=~int[];function f<T extends S>(v:T):T{return append(v);}`, ""},
		{"append missing spread source", `constraint S=~int[];function f<T extends S>(v:T):T{return append(v...);}`, "spread append expects"},
		{"append concrete missing spread source", `function f(v:int[]):int[]{return append(v...);}`, "spread append expects"},
		{"append individual", `constraint S=~int[];function f<T extends S>(v:T):T{return append(v,1,2);}`, ""},
		{"append dependent", `constraint S<E>=~E[];function f<E,T extends S<E>>(v:T,e:E):T{return append(v,e);}`, ""},
		{"append separate parameters", `constraint S<E>=~E[];function f<E,T extends S<E>,U extends S<E>>(v:T,s:U):T{return append(v,s...);}`, ""},
		{"append byte string", `constraint S=~byte[];constraint Text=~string;function f<T extends S,U extends Text>(v:T,s:U):T{return append(v,s...);}`, ""},
		{"copy separate parameters", `constraint S<E>=~E[];function f<E,T extends S<E>,U extends S<E>>(v:T,s:U):int{return copy(v,s);}`, ""},
		{"copy byte string", `constraint S=~byte[];constraint Text=~string;function f<T extends S,U extends Text>(v:T,s:U):int{return copy(v,s);}`, ""},
		{"named alternatives", `type A=distinct int[];type B=distinct int[];constraint S=A|B;function f<T extends S>(v:T):T{copy(v,v);return append(v,v...);}`, ""},
		{"copy dependent nullable", `class Base{}alias Maybe=Base|null;constraint S<E>=~E[];class Copier<E>{public function run<T extends S<E>>(v:T,s:T):int{return copy(v,s);}}function f(v:Maybe[]):int{return new Copier<Maybe>().run(v,v);}`, ""},
		{"append callback context", `alias Fn=(n:int)=>int;constraint S=~Fn[];function f<T extends S>(v:T):T{return append(v,(n)=>n+1);}`, ""},
		{"append object context", `alias Point={x:int};constraint S=~Point[];function f<T extends S>(v:T):T{return append(v,{x:1});}`, ""},
		{"append array", `constraint S=~[2]int;function f<T extends S>(v:T):T{return append(v,1);}`, "append requires"},
		{"append mixed elements", `constraint S=~int[]|~string[];function f<T extends S>(v:T):T{return append(v);}`, "append requires"},
		{"append mixed shapes", `constraint S=~byte[]|~string;function f<T extends S>(v:T):T{return append(v,"x"...);}`, "append requires"},
		{"append unconstrained", `function f<T>(v:T):T{return append(v);}`, "append requires"},
		{"append wrong value", `constraint S=~int[];function f<T extends S>(v:T):T{return append(v,"x");}`, "cannot use"},
		{"append overflow", `constraint S=~byte[];function f<T extends S>(v:T):T{return append(v,256);}`, "cannot be represented"},
		{"append fractional", `constraint S=~byte[];function f<T extends S>(v:T):T{return append(v,1.5);}`, "truncated"},
		{"append mismatched spread", `constraint S=~int[];constraint U=~string[];function f<T extends S,V extends U>(v:T,s:V):T{return append(v,s...);}`, "does not match destination element"},
		{"copy mixed elements", `constraint S=~int[]|~string[];function f<T extends S>(v:T):int{return copy(v,v);}`, "copy destination must be a slice"},
		{"copy mixed source", `constraint S=~byte[]|~string;function f<T extends S>(v:byte[],s:T):int{return copy(v,s);}`, "copy source must be"},
		{"copy wrong source", `constraint S=~int[];constraint U=~string[];function f<T extends S,V extends U>(v:T,s:V):int{return copy(v,s);}`, "does not match destination element"},
		{"distinct byte string", `type Byte=distinct byte;constraint S=~Byte[];function f<T extends S>(v:T):int{return copy(v,"x");}`, "copy source must be"},
		{"class append", `class Base{}class Child extends Base{}constraint S=~Base[];function f<T extends S>(v:T):T{return append(v,new Child());}`, ""},
		{"class nullable", `class Base{}alias Maybe=Base|null;constraint S=~Maybe[];function f<T extends S>(v:T):T{return append(v,null);}`, ""},
		{"class nonnullable", `class Base{}constraint S=~Base[];function f<T extends S>(v:T):T{return append(v,null);}`, "cannot use null"},
		{"class spread invariant", `class Base{}class Child extends Base{}constraint S=~Base[];function f<T extends S>(v:T,s:Child[]):T{return append(v,s...);}`, "does not match destination element"},
		{"copy nullable invariant", `class Base{}alias Maybe=Base|null;constraint S=~Base[];function f<T extends S>(v:T,s:Maybe[]):int{return copy(v,s);}`, "does not match destination element"},
		{"copy generic class invariant", `class Box<E>{constructor(public value:E){}}function f(v:Box<int>[],s:Box<string>[]):int{return copy(v,s);}`, "does not match destination element"},
		{"copy generic class named argument", `type Items=distinct int[];class Box<E>{constructor(public value:E){}}function f(v:Box<int[]>[],s:Box<Items>[]):int{return copy(v,s);}`, "does not match destination element"},
		{"copy nested class slices", `class Box{}function f(v:Box[][],s:Box[][]):int{return copy(v,s);}`, ""},
		{"copy object nullable field", `alias Maybe=int[]|null;alias A={value:Maybe};alias B={value:int[]};function f(v:A[],s:B[]):int{return copy(v,s);}`, "does not match destination element"},
		{"copy nested nullable invariant", `alias Maybe=int[]|null;function f(v:Maybe[],s:int[][]):int{return copy(v,s);}`, "does not match destination element"},
		{"append nested nullable invariant", `alias Maybe=int[]|null;function f(v:Maybe[],s:int[][]):Maybe[]{return append(v,s...);}`, "does not match destination element"},
		{"generic nullable argument", `class Base{}alias Maybe=Base|null;constraint S=~Base[];function add<T extends S>(v:T):T{return append(v,new Base());}function f(v:Maybe[]):void{const x=add(v);}`, "nullable type information"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
