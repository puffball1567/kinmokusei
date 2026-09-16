package sema

import (
	"strings"
	"testing"
)

func TestScalarConstantContexts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"string aliases", `type Text=distinct string;const a="a";const b=a+"b";function f():Text{return b;}`, ""},
		{"boolean aliases", `type Flag=distinct boolean;const a=true;const b=!a||a;function f():Flag{return b;}`, ""},
		{"typed string", `type Text=distinct string;const a:Text="a";const b="b"+a;function f():Text{return b;}`, ""},
		{"typed bool", `type Flag=distinct boolean;const a:Flag=true;const b=!a;const c=true&&b;function f():Flag{return c;}`, ""},
		{"named boolean control", `type Flag=distinct boolean;function f(v:Flag):int{if(v){return 1;}while(v){break;}for(;v;){break;}return 0;}`, ""},
		{"generic string", `type Text=distinct string;function choose<T>(a:T,b:T):T{return a;}function f(v:Text):Text{const a="x";return choose(a,v);}`, ""},
		{"generic boolean", `type Flag=distinct boolean;function choose<T>(a:T,b:T):T{return b;}function f(v:Flag):Flag{const a=true;return choose(a,v);}`, ""},
		{"explicit generic", `type Text=distinct string;function keep<T>(x:T):T{return x;}function f():Text{return keep<Text>("x");}`, ""},
		{"len narrow conversion", `const a="abc";function f():byte{return byte(len(a));}`, ""},
		{"len alias", `const a="温泉";const n=len(a);const m=n+1;function f():int{return m;}`, ""},
		{"converted len", `type Text=distinct string;const a=Text("abc");function f():int{return len(a);}`, ""},
		{"len stays int", `const a="abc";function f():byte{return len(a);}`, "of type int"},
		{"len alias stays int", `const a="abc";const n=len(a);function f():byte{return n;}`, "of type int"},
		{"Go import", `import go http from "net/http";type Text=distinct string;const a=http.MethodGet;const b=a;function f():Text{return b;}`, ""},
		{"Go direct import", `import go http from "net/http";type Text=distinct string;function f():Text{return http.MethodGet;}`, ""},
		{"index", `const a="ab";const b=a+"c";function f():byte{return b[len(b)-1];}`, ""},
		{"index overflow", `const a="ab";const b=a+"c";function f():byte{return b[3];}`, "out of bounds"},
		{"slice overflow", `const a="ab";const b=a;function f():string{return b[:3];}`, "exceeds string length 2"},
		{"named slice overflow", `type Text=distinct string;const a:Text="ab";function f():Text{return a[:3];}`, "exceeds string length 2"},
		{"mutable typed string", `type Text=distinct string;let a="a";const b=a;function f():Text{return b;}`, "cannot use string as Text"},
		{"explicit typed string stays typed", `type Text=distinct string;const a:string="a";const b=a;function f():Text{return b;}`, "cannot use string as Text"},
		{"explicit typed bool stays typed", `type Flag=distinct boolean;const a:boolean=true;const b=a;function f():Flag{return b;}`, "cannot use boolean as Flag"},
		{"runtime len", `let a="abc";const n=len(a);function f():byte{return n;}`, "cannot use int as byte"},
		{"call len", `function text():string{return "abc";}const n=len(text());function f():byte{return n;}`, "cannot use int as byte"},
		{"slice len", `const a="abc";const n=len(a[:2]);function f():byte{return n;}`, "cannot use int as byte"},
		{"slice becomes typed", `type Text=distinct string;function f():Text{return "abc"[:2];}`, "cannot use string as Text"},
		{"alias slice becomes typed", `type Text=distinct string;const a="abc";function f():Text{return a[:2];}`, "cannot use string as Text"},
		{"named slice keeps type", `type Text=distinct string;const a:Text="abc";function f():Text{return a[:2];}`, ""},
		{"loop storage", `function f():byte{for(const a="abc";false;){const n=len(a);return n;}return 0;}`, "cannot use int as byte"},
		{"string address", `function f():void{const a="x";const b=a;const p=&b;}`, "addressable"},
		{"boolean address", `function f():void{const a=true;const b=a;const p=&b;}`, "addressable"},
		{"len address", `function f():void{const n=len("abc");const p=&n;}`, "addressable"},
		{"runtime address", `function text():string{return "x";}function f():void{const a=text();const b=a;const p=&b;}`, ""},
		{"generic conversion runtime", `constraint Text=~string;function f<T extends Text>():void{const a=T("x");const p=&a;}`, ""},
		{"short circuit remains runtime", `function yes():boolean{return true;}function f():void{const a=false&&yes();const p=&a;}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
