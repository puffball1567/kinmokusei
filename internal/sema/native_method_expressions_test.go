package sema

import (
	"strings"
	"testing"
)

func TestNativeMethodExpressions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"struct value", `struct S{public n:int;public function read():int{return this.n;}}function f(v:S):int{const read=S.read;return read(v);}`, ""},
		{"struct pointer", `struct S{public n:int;public pointer function set(n:int):void{this.n=n;}}function f(v:*S):void{(*S).set(v,2);}`, ""},
		{"pointer value wrapper", `struct S{public n:int;public function read():int{return this.n;}}function f(v:*S):int{return (*S).read(v);}`, ""},
		{"struct alias", `struct S{public n:int;public function read():int{return this.n;}}alias A=S;function f(v:A):int{return A.read(v);}`, ""},
		{"specialized alias", `struct Box<T>{public n:T;public function read():T{return this.n;}}alias B=Box<int>;function f(v:B):int{return B.read(v);}`, ""},
		{"specialized verified text", `struct Box<T>{public n:T;public function read():T{return this.n;}}alias B=Box<string>;function f(v:B):string{return B.read(v);}`, ""},
		{"specialized nullable", `class C{public n:int=1;}struct Box<T>{public n:T;public function read():T{return this.n;}}alias B=Box<C|null>;function f(v:B):int{return B.read(v).n;}`, "nullable"},
		{"specialized interface alias", `interface I<T>{function read(v:T):T;}alias B=I<string>;function f(v:B):string{return B.read(v,"text");}`, ""},
		{"nullable callback mismatch", `class C{}struct S{public function read():C|null{return null;}}const f:(v:S)=>C=S.read;`, "cannot use"},
		{"raw callback mismatch", `struct S{public function read():bstring{return b"raw";}}const f:(v:S)=>string=S.read;`, "cannot use"},
		{"defined scalar", `type N=distinct int;public function plus(this:N,n:int):N{return N(int(this)+n);}function f(v:N):N{return N.plus(v,2);}`, ""},
		{"enum value", `enum S:int8{A,B}public function read(this:S):int{return int(this);}function f(v:S):int{const read=S.read;return read(v);}`, ""},
		{"enum pointer", `enum S:int8{A,B}public function set(this:*S,v:S):void{*this=v;}function f(v:*S):void{(*S).set(v,S.B);}`, ""},
		{"enum pointer value wrapper", `enum S:int8{A,B}public function read(this:S):int{return int(this);}function f(v:*S):int{return (*S).read(v);}`, ""},
		{"enum alias", `enum S:int8{A,B}alias A=S;public function read(this:S):int{return int(this);}function f(v:A):int{return A.read(v);}`, ""},
		{"enum shadow value", `enum S:int8{A,B}public function read(this:S):int{return int(this);}function f(S:S):int{return S.read();}`, ""},
		{"enum pointer method on value type", `enum S{A}public function set(this:*S):void{}const f=S.set;`, "no non-generic method"},
		{"enum pointer constant", `enum S{A}const f=(*S).A;`, "no non-generic method"},
		{"enum nominal receiver", `enum S{A}enum T{A}public function read(this:S):int{return int(this);}function f(v:T):int{return S.read(v);}`, "cannot use"},
		{"enum unknown member", `enum S{A}const f=S.Missing;`, "has no member"},
		{"enum constant precedence", `enum S{read}public function read(this:S):int{return int(this);}function f():S{return S.read;}`, ""},
		{"enum method with constant name through pointer", `enum S{read}public function read(this:S):int{return int(this);}function f(v:*S):int{return (*S).read(v);}`, ""},
		{"enum method initialization cycle", `enum S{A}const n:int=S.read(S.A);public function read(this:S):int{return n;}`, "initialization cycle"},
		{"interface", `interface I{function read():int;}function f(v:I):int{return I.read(v);}`, ""},
		{"inherited interface", `interface I{function read():int;}interface J extends I{}function f(v:J):int{return J.read(v);}`, ""},
		{"variadic", `struct S{public function all(...xs:int[]):int{return len(xs);}}function f(v:S):int{const xs=[1,2];return S.all(v,xs...);}`, ""},
		{"result", `struct S{public function read():Result<int>{return ok(1);}}function f(v:S):Result<int>{const read=S.read;return read(v);}`, ""},
		{"multi result", `struct S{public function read():(int,string){return 1,"ok";}}function f(v:S):(int,string){return S.read(v);}`, ""},
		{"shadow value", `struct S{public n:int;public function read():int{return this.n;}}function f(S:S):int{return S.read();}`, ""},
		{"nullable argument", `interface I{function read():int;}function f(v:I|null):int{return I.read(v);}`, "cannot use"},
		{"nullable result", `class C{public n:int=1;}struct S{public function read():C|null{return null;}}function f(v:S):int{return S.read(v).n;}`, "nullable"},
		{"verified result", `struct S{public function read():bstring{return b"raw";}}function f(v:S):string{return S.read(v);}`, "cannot use bstring as string"},
		{"verified argument", `struct S{public function read(v:string):int{return len(v);}}function f(v:S,s:bstring):int{return S.read(v,s);}`, "cannot use bstring as string"},
		{"pointer method on value type", `struct S{public pointer function set():void{}}const f=S.set;`, "no non-generic method"},
		{"pointer interface", `interface I{function read():int;}const f=(*I).read;`, "cannot be used as a Go pointer target"},
		{"field expression", `struct S{public n:int;}const f=S.n;`, "no non-generic method"},
		{"generic receiver", `struct S<T>{public function read():int{return 1;}}const f=S.read;`, "type argument"},
		{"generic method", `struct S{public function read<T>(v:T):T{return v;}}const f=S.read;`, "no non-generic method"},
		{"pointer receiver argument", `struct S{public pointer function set():void{}}function f(v:S):void{(*S).set(v);}`, "cannot use"},
		{"nominal receiver", `struct S{public function read():int{return 1;}}struct T{}function f(v:T):int{return S.read(v);}`, "cannot use"},
		{"standalone pointer type", `struct S{}const f=(*S);`, "cannot be used as a value"},
		{"pointer type address", `struct S{}const f=&(*S);`, "cannot be used as a value"},
		{"method initialization cycle", `const v:int=S.read(S{});struct S{public function read():int{return v;}}`, "initialization cycle"},
		{"bound method initialization cycle", `const v:int=S{}.read();struct S{public function read():int{return v;}}`, "initialization cycle"},
		{"stored method initialization cycle", `const f:(v:S)=>int=S.read;struct S{public function read():int{return f(this);}}`, "initialization cycle"},
		{"defined method initialization cycle", `type N=distinct int;const v:int=N.read(N(0));public function read(this:N):int{return v;}`, "initialization cycle"},
		{"ordinary recursion", `struct S{public function read(n:int):int{if(n===0){return 0;}return S.read(this,n-1);}}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
