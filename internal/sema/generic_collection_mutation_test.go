package sema

import (
	"strings"
	"testing"
)

func TestGenericCollectionMutation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"clear slices", `constraint C=~int[]|~string[];function f<T extends C>(v:T):void{clear(v);}`, ""},
		{"clear mixed", `constraint C=~int[]|~Map<string,int>|~Map<int,string>;function f<T extends C>(v:T):void{clear(v);}`, ""},
		{"clear dependent", `constraint C<E>=~E[]|~Map<string,E>;function f<E,T extends C<E>>(v:T):void{clear(v);}`, ""},
		{"clear scalar", `constraint C=~int[]|~int;function f<T extends C>(v:T):void{clear(v);}`, "clear requires"},
		{"clear array", `constraint C=~int[]|~[3]int;function f<T extends C>(v:T):void{clear(v);}`, "clear requires"},
		{"clear channel", `constraint C=~GoChannel<int>;function f<T extends C>(v:T):void{clear(v);}`, "clear requires"},
		{"clear any", `function f<T>(v:T):void{clear(v);}`, "clear requires"},
		{"clear nullable", `constraint C=~int[];function f<T extends C>(v:T|null):void{clear(v);}`, ""},
		{"clear result", `constraint C=~int[];function f<T extends C>(v:T):int{return clear(v);}`, "void"},
		{"clear type arguments", `constraint C=~int[];function f<T extends C>(v:T):void{clear<T>(v);}`, "expects 0 type arguments"},
		{"clear arity", `constraint C=~int[];function f<T extends C>(v:T):void{clear(v,v);}`, "expects 1 arguments"},
		{"clear spread", `constraint C=~int[];function f<T extends C>(v:T):void{clear(v...);}`, "does not accept spread"},
		{"delete mixed values", `constraint C=~Map<string,int>|~Map<string,string>;function f<T extends C>(v:T):void{delete(v,"x");}`, ""},
		{"delete dependent key", `constraint C<K extends comparable>=~Map<K,int>|~Map<K,string>;function f<K extends comparable,T extends C<K>>(v:T,k:K):void{delete(v,k);}`, ""},
		{"delete scalar", `constraint C=~Map<string,int>|~int;function f<T extends C>(v:T):void{delete(v,"x");}`, "delete requires"},
		{"delete slice", `constraint C=~Map<string,int>|~int[];function f<T extends C>(v:T):void{delete(v,"x");}`, "delete requires"},
		{"delete mixed keys", `constraint C=~Map<string,int>|~Map<int,int>;function f<T extends C>(v:T):void{delete(v,"x");}`, "delete requires"},
		{"delete any", `function f<T>(v:T):void{delete(v,"x");}`, "delete requires"},
		{"delete nullable map", `constraint C=~Map<string,int>;function f<T extends C>(v:T|null):void{delete(v,"x");}`, ""},
		{"delete wrong key", `constraint C=~Map<string,int>;function f<T extends C>(v:T):void{delete(v,1);}`, "cannot use"},
		{"delete byte key", `constraint C=~Map<byte,int>;function f<T extends C>(v:T):void{delete(v,255);}`, ""},
		{"delete byte overflow", `constraint C=~Map<byte,int>;function f<T extends C>(v:T):void{delete(v,256);}`, "overflows"},
		{"delete float constant", `constraint C=~Map<byte,int>;function f<T extends C>(v:T):void{delete(v,2.0);}`, ""},
		{"delete fractional", `constraint C=~Map<byte,int>;function f<T extends C>(v:T):void{delete(v,2.5);}`, "truncated"},
		{"delete source key", `class Key{}constraint C=~Map<Key,int>|~Map<Key,string>;function f<T extends C>(v:T,k:Key):void{delete(v,k);}`, ""},
		{"delete nonnullable key", `class Key{}constraint C=~Map<Key,int>|~Map<Key,string>;function f<T extends C>(v:T):void{delete(v,null);}`, "cannot use"},
		{"delete nullable key", `class Key{}constraint C=~Map<Key|null,int>|~Map<Key|null,string>;function f<T extends C>(v:T):void{delete(v,null);}`, ""},
		{"delete mixed nullability", `class Key{}constraint C=~Map<Key,int>|~Map<Key|null,string>;function f<T extends C>(v:T,k:Key):void{delete(v,k);}`, "delete requires"},
		{"delete named key", `type Key=distinct string;constraint C=~Map<Key,int>;function f<T extends C>(v:T,k:string):void{delete(v,k);}`, "cannot use"},
		{"delete named literal", `type Key=distinct string;constraint C=~Map<Key,int>;function f<T extends C>(v:T):void{delete(v,"x");}`, ""},
		{"delete arity", `constraint C=~Map<string,int>;function f<T extends C>(v:T):void{delete(v);}`, "expects 2 arguments"},
		{"delete type arguments", `constraint C=~Map<string,int>;function f<T extends C>(v:T):void{delete<T>(v,"x");}`, "expects 0 type arguments"},
		{"delete spread", `constraint C=~Map<string,int>;function f<T extends C>(v:T):void{delete(v,"x"...);}`, "does not accept spread"},
		{"delete does not enable index", `constraint C=~Map<string,int>|~Map<string,string>;function f<T extends C>(v:T):void{delete(v,"x");const x=v["x"];}`, "cannot be indexed"},
		{"concrete overflow", `function f(v:Map<byte,int>):void{delete(v,256);}`, "overflows"},
		{"concrete fractional", `function f(v:Map<byte,int>):void{delete(v,1.5);}`, "truncated"},
		{"constant alias overflow", `const original=256;const alias=original;constraint C=~Map<byte,int>;function f<T extends C>(v:T):void{delete(v,alias);}`, "overflows"},
		{"constant alias valid", `const original=255;const alias=original;constraint C=~Map<byte,int>;function f<T extends C>(v:T):void{delete(v,alias);}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
