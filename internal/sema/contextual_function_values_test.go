package sema

import (
	"strings"
	"testing"
)

func TestContextualGenericFunctionValues(t *testing.T) {
	t.Parallel()
	prefix := `function id<T>(value:T):T{return value;}function pair<T,U>(a:T,b:U):(T,U){return a,b;}`
	for _, test := range []struct{ name, source, want string }{
		{"global", `const f:(value:int)=>int=id;`, ""},
		{"local", `function use():int{const f:(value:int)=>int=id;return f(7);}`, ""},
		{"assignment", `function use():int{let f:(value:int)=>int=(value)=>value;f=id;return f(7);}`, ""},
		{"return", `function make():(value:int)=>int{return id;}`, ""},
		{"callback", `function apply(f:(v:int)=>int):int{return f(7);}const n=apply(id);`, ""},
		{"generic callback first", `function apply<T>(f:(v:T)=>T,v:T):T{return f(v);}const n=apply(id,7);`, ""},
		{"generic callback last", `function apply<T>(v:T,f:(v:T)=>T):T{return f(v);}const n=apply(7,id);`, ""},
		{"Go generic callback", `import go slices from "slices";import go cmp from "cmp";function use(values:int[]):void{slices.SortFunc(values,cmp.Compare);}`, ""},
		{"unresolved callback cycle", `function apply<T>(f:(v:T)=>T):void{}function use():void{apply(id);}`, "cannot infer generic function callback"},
		{"arrow", `const make:()=>(value:int)=>int=()=>id;`, ""},
		{"partial angle", `const f:(a:int,b:bstring)=>(int,bstring)=pair<int>;`, ""},
		{"partial bracket", `const f:(a:int,b:bstring)=>(int,bstring)=pair[int];`, ""},
		{"result only", `function empty<T>():T[]{return [];}const f:()=>int[]=empty;`, ""},
		{"variadic", `function first<T>(...values:T[]):T{return values[0];}const f:(...values:int[])=>int=first;`, ""},
		{"Result", `function success<T>(value:T):Result<T>{return ok(value);}const f:(v:int)=>Result<int>=success;`, ""},
		{"outer parameter", `function use<T>(v:T):T{const f:(value:T)=>T=id;return f(v);}`, ""},
		{"mixed outer parameter", `alias F=()=>int;constraint Mixed=F|int;function use<T extends Mixed>(v:T):T{const f:(value:T)=>T=id;return f(v);}`, ""},
		{"text", `const f:(value:string)=>string=id;`, ""},
		{"nullable", `class C{}alias Maybe=C|null;const f:(value:Maybe)=>Maybe=id;`, ""},
		{"named callback", `type F=distinct(value:int)=>int;const f:F=id;`, ""},
		{"array", `alias F=(value:int)=>int;const fs:F[]=[id,id];`, ""},
		{"inferred array tail", `const fs=[id<int>,id];`, ""},
		{"object", `const holder:{f:(value:int)=>int}={f:id};`, ""},
		{"field", `class Box{public f:(v:int)=>int=id;}`, ""},
		{"Go", `import go slices from "slices";const f:(values:int[])=>int[]=slices.Clone;`, ""},
		{"Go partial", `import go slices from "slices";const f:(values:int[])=>int[]=slices.Clone<int[]>;`, ""},
		{"Go named import", `import go {Clone as clone} from "slices";const f:(values:int[])=>int[]=clone;`, ""},
		{"Go verified record", `import go slices from "slices";struct Record{public text:string;}const f:(values:Record[])=>Record[]=slices.Clone;`, "verified native text contract"},
		{"generic method", `struct S{public function echo<T>(v:T):T{return v;}}const s=S{};const f:(v:int)=>int=s.echo;`, "generic method values are not yet supported"},
		{"constraint", `function comparable<T extends comparable>(value:T):T{return value;}const f:(value:int[])=>int[]=comparable;`, "constraint"},
		{"conflict", `const f:(value:int)=>bstring=id;`, "already inferred"},
		{"partial conflict", `const f:(a:bstring,b:int)=>(bstring,int)=pair<int>;`, "already inferred"},
		{"unused parameter", `function value<T,U>(v:T):T{return v;}const f:(v:int)=>int=value;`, "cannot infer type argument U"},
		{"nullability conflict", `class C{}const f:(value:C|null)=>C=id;`, "cannot use"},
		{"text conflict", `const f:(value:string)=>bstring=id;`, "already inferred"},
		{"Result conflict", `function success<T>(value:T):Result<T>{return ok(value);}const f:(v:int)=>int=success;`, "cannot use"},
		{"arity", `const f:(a:int,b:int)=>int=id;`, "cannot use"},
		{"variadic mismatch", `function first<T>(...values:T[]):T{return values[0];}const f:(values:int[])=>int=first;`, "already inferred"},
		{"shadowed", `function use(id:int):void{const f:(value:int)=>int=id;}`, "cannot use"},
		{"cycle", `const f:(v:int)=>int=id;function run():int{return f(1);}`, ""},
		{"actual cycle", `const f:(v:int)=>int=loop;function loop<T>(v:T):T{f(1);return v;}`, "initialization cycle"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, prefix+test.source), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}
