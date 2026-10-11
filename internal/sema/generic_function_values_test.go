package sema

import (
	"strings"
	"testing"
)

func TestGenericFunctionValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"angle", `function id<T>(v:T):T{return v;}const f=id<int>;const n=f(1);`, ""},
		{"bracket", `function id<T>(v:T):T{return v;}const f=id[int];const n=f(1);`, ""},
		{"multiple arguments", `function pair<T,U>(a:T,b:U):(T,U){return a,b;}const f=pair[int,bstring];function use():(int,bstring){const [a,b]=f(1,b"x");return a,b;}`, ""},
		{"optional semicolon", "function id<T>(v:T):T{return v;}\nconst f=id<int>\nconst n=f(1)", ""},
		{"nil comparison", `function id<T>(v:T):T{return v;}const ready=id<int>!==nil;`, ""},
		{"nested types", `function id<T>(v:T):T{return v;}const f=id[Map<int,int[]>];`, ""},
		{"fixed array", `function id<T>(v:T):T{return v;}const f=id[[2]int];`, ""},
		{"function type", `function id<T>(v:T):T{return v;}const f=id[(v:int)=>int];`, ""},
		{"pointer type", `struct S{public n:int;}function id<T>(v:T):T{return v;}const f=id[*S];`, ""},
		{"outer parameter", `function id<T>(v:T):T{return v;}function outer<T>(v:T):T{const f=id<T>;return f(v);}`, ""},
		{"verified text", `function id<T>(v:T):T{return v;}const f:(v:string)=>string=id<string>;const s=f("text");`, ""},
		{"nullable", `class C{}function id<T>(v:T):T{return v;}const f=id<C|null>;function use(v:C|null):C|null{return f(v);}`, ""},
		{"variadic", `function first<T>(...vs:T[]):T{return vs[0];}const f=first<int>;const n=f(1,2);`, ""},
		{"Result", `function success<T>(v:T):Result<T>{return ok(v);}const f=success<int>;function use():Result<int>{return f(1);}`, ""},
		{"callback", `function id<T>(v:T):T{return v;}function use(f:(v:int)=>int):int{return f(3);}const n=use(id<int>);`, ""},
		{"Go function", `import go slices from "slices";const f=slices.Clone[int[],int];const xs=f([1,2]);`, ""},
		{"named Go import", `import go {Clone} from "slices";const f=Clone<int[],int>;const xs=f([1,2]);`, ""},
		{"constraint", `function id<T extends comparable>(v:T):T{return v;}const f=id<int[]>;`, "constraint"},
		{"Go constraint", `import go slices from "slices";const f=slices.Clone[int,int];`, "cannot instantiate"},
		{"Go verified record", `import go slices from "slices";struct Record{public text:string;}const f=slices.Clone<Record[],Record>;`, "verified native text contract"},
		{"nullable callback mismatch", `class C{}function id<T>(v:T):T{return v;}const f:(v:C)=>C=id<C|null>;`, "cannot use"},
		{"stored Result cannot be ignored", `function success<T>(v:T):Result<T>{return ok(v);}const f=success<int>;function use():void{f(1);}`, "Result"},
		{"instantiation is not assignable", `function id<T>(v:T):T{return v;}const f=id<int>;function use():void{(id<int>)=f;}`, "not assignable"},
		{"partial", `function pair<T,U>(a:T,b:U):(T,U){return a,b;}const f=pair<int>;`, "requires all 2"},
		{"partial Go", `import go slices from "slices";const f=slices.Clone<int[]>;`, "requires all 2"},
		{"extra", `function id<T>(v:T):T{return v;}const f=id<int,int>;`, "requires all 1"},
		{"unknown", `function id<T>(v:T):T{return v;}const f=id<Missing>;`, "unknown type"},
		{"non generic", `function id(v:int):int{return v;}const f=id<int>;`, "requires a generic function"},
		{"generic method", `struct S{public function id<T>(v:T):T{return v;}}const s=S{};const f=s.id<int>;`, "generic method values are not yet supported"},
		{"text mismatch", `function id<T>(v:T):T{return v;}const f:(v:bstring)=>string=id<bstring>;`, "cannot use"},
		{"ordinary index", `function use(xs:int[],index:int):int{const n=xs[index];return n;}`, ""},
		{"shadowed index", `function id<T>(v:T):T{return v;}function use(id:int[],index:int):int{return id[index];}`, ""},
		{"uninstantiated", `function id<T>(v:T):T{return v;}const f=id;`, "must be called"},
		{"initialization cycle", `const f:(v:int)=>int=id<int>;function id<T>(v:T):T{f(1);return v;}`, "initialization cycle"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
