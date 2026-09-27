package sema

import (
	"strings"
	"testing"
)

func TestMakeCollection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"slice", `function f():int[]{return make[int[]](2,4);}`, ""},
		{"named slice", `type S=distinct int[];function f():S{return make[S](2);}`, ""},
		{"map", `function f():Map<string,int>{return make[Map<string,int>]();}`, ""},
		{"channel", `function f():GoChannel<int>{return make[GoChannel<int>](1);}`, ""},
		{"send channel", `function f():GoSendChannel<int>{return make[GoSendChannel<int>]();}`, ""},
		{"receive channel", `function f():GoReceiveChannel<int>{return make[GoReceiveChannel<int>](2);}`, ""},
		{"generic slice", `constraint S<E>=~E[];function f<E,T extends S<E>>(n:int):T{return make[T](n,n+1);}`, ""},
		{"generic map", `constraint M<V>=~Map<string,V>;function f<V,T extends M<V>>():T{return make[T]();}`, ""},
		{"generic send", `constraint C=~GoChannel<int>|~GoSendChannel<int>;function f<T extends C>():T{return make[T](2);}`, ""},
		{"generic receive", `constraint C=~GoChannel<int>|~GoReceiveChannel<int>;function f<T extends C>():T{return make[T]();}`, ""},
		{"named alternatives", `type A=distinct int[];type B=distinct int[];constraint C=A|B;function f<T extends C>():T{return make[T](2);}`, ""},
		{"generic sizes", `constraint N=~int|~uint64;function f<T extends N>(n:T):int[]{return make[int[]](n,n);}`, ""},
		{"constant sizes", `const n=2.0;function f():int[]{return make[int[]](n,4.0);}`, ""},
		{"nullable elements", `class C{}function f():Map<string,C|null>{return make[Map<string,C|null>]();}`, ""},
		{"generic method", `constraint S<E>=~E[];class Allocator<E>{public function run<T extends S<E>>(n:int):T{return make[T](n);}}`, ""},
		{"shadow", `function make(n:int):int{return n;}function f():int{return make(2);}`, ""},
		{"local shadow", `function f():int{const make=(n:int):int=>n;return make(2);}`, ""},
		{"unused allocation", `function f():void{make[int[]](2);}`, "make result must be used"},
		{"unused ordered allocation", `function f():void{make[int[]](2,4);}`, "make result must be used"},
		{"explicit discard", `function f():void{_=make[int[]](2);_=make[GoChannel<int>]();}`, ""},
		{"constructor slice proof", `class User{}type S=distinct int[];class Holder{private user:User;constructor(){for(const n of make[S](2)){this.user=new User();}}}`, ""},
		{"constructor empty slice", `class User{}class Holder{private user:User;constructor(){for(const n of make[int[]](0)){this.user=new User();}}}`, "every constructor path"},
		{"constructor map hint", `class User{}class Holder{private user:User;constructor(){for(const n of make[Map<int,int>](2)){this.user=new User();}}}`, "every constructor path"},
		{"constructor channel buffer", `class User{}class Holder{private user:User;constructor(){for(const n of make[GoChannel<int>](2)){this.user=new User();}}}`, "every constructor path"},
		{"missing type", `function f():void{make(2);}`, "one collection type argument"},
		{"extra type", `function f():void{make[int[],int](2);}`, "one collection type argument"},
		{"unknown type", `function f():void{make[Missing](2);}`, "unknown type"},
		{"not collection", `function f():void{make[int](2);}`, "make target must be"},
		{"array", `function f():void{make[[2]int](2);}`, "make target must be"},
		{"nullable target", `function f():void{make[int[]|null](2);}`, "make target must be"},
		{"unconstrained", `function f<T>():T{return make[T](2);}`, "make target must be"},
		{"mixed shapes", `constraint S=~int[]|~Map<string,int>;function f<T extends S>():T{return make[T](2);}`, "make target must be"},
		{"mixed elements", `constraint S=~int[]|~string[];function f<T extends S>():T{return make[T](2);}`, "make target must be"},
		{"mixed maps", `constraint S=~Map<string,int>|~Map<string,string>;function f<T extends S>():T{return make[T]();}`, "make target must be"},
		{"conflicting directions", `constraint C=~GoSendChannel<int>|~GoReceiveChannel<int>;function f<T extends C>():T{return make[T]();}`, "make target must be"},
		{"mixed channel elements", `constraint C=~GoChannel<int>|~GoChannel<string>;function f<T extends C>():T{return make[T]();}`, "make target must be"},
		{"missing length", `function f():void{make[int[]]();}`, "between 1 and 2 size arguments"},
		{"excess length", `function f():void{make[Map<string,int>](1,2);}`, "between 0 and 1 size arguments"},
		{"channel capacity", `function f():void{make[GoChannel<int>](1,2);}`, "between 0 and 1 size arguments"},
		{"spread", `function f(n:int[]):void{make[int[]](n...);}`, "does not accept spread"},
		{"fraction", `function f():void{make[int[]](1.5);}`, "size must be an integer"},
		{"typed float", `function f(n:float):void{make[int[]](n);}`, "size must be an integer"},
		{"negative", `function f():void{make[int[]](-1);}`, "cannot be negative"},
		{"overflow", `function f():void{make[int[]](9223372036854775808);}`, "out of range"},
		{"capacity order", `function f():void{make[int[]](3,2);}`, "capacity cannot be smaller"},
		{"nullable access", `class C{public value:int=1;}alias Maybe=C|null;function f():int{const xs=make[Maybe[]](1);return xs[0].value;}`, "nullable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
