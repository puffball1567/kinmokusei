package sema

import (
	"strings"
	"testing"
)

func TestTrustedGoReadOnlyCallContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"generic", `import go r from "reflect";function same<T>(a:T,b:T):boolean{return r.DeepEqual(a,b);}function main():void{same<int>(1,1);}`, ""},
		{"slice", `import go r from "reflect";function f(a:string[],b:string[]):boolean{return r.DeepEqual(a,b);}`, ""},
		{"nested", `import go r from "reflect";struct Row{public text:string;public values:Map<int,string[]>;}function f(a:*Row,b:*Row):boolean{return r.DeepEqual(a,b);}`, ""},
		{"nullable class", `import go r from "reflect";class C{public text:string="";}function f(a:C|null,b:C|null):boolean{return r.DeepEqual(a,b);}`, ""},
		{"callback", `import go r from "reflect";function f(a:(v:string)=>void,b:(v:string)=>void):boolean{return r.DeepEqual(a,b);}`, ""},
		{"named alias", `import go {DeepEqual as eq} from "reflect";function f<T>(a:T,b:T):boolean{return eq(a,b);}`, ""},
		{"multiple arguments", `import go r from "reflect";function pair(a:string[]):(string[],string[]){return a,a;}function f(a:string[]):boolean{return r.DeepEqual(pair(a));}`, ""},
		{"task", `import go r from "reflect";function f(a:string[]):boolean{const task=go r.DeepEqual(a,a);return await task;}`, ""},
		{"type name is not value", `import go r from "reflect";function f():boolean{return r.DeepEqual(r.Value,r.Value);}`, "Go type"},
		{"package is not value", `import go r from "reflect";function f():boolean{return r.DeepEqual(r,r);}`, "Go package"},
		{"Result is not value", `import go r from "reflect";function value():Result<string>{return ok("x");}function f():boolean{return r.DeepEqual(value(),value());}`, "Result"},
		{"arity", `import go r from "reflect";function f(a:string[]):boolean{return r.DeepEqual(a);}`, "expects 2"},
		{"capability", `import go r from "reflect";function f<T>(a:T):void{r.ValueOf(a);}`, "cannot use T as any"},
		{"formatter", `import go f from "fmt";function same(a:string[]):void{f.Sprint(a);}`, "cannot use string[] as any"},
		{"encoder", `import go j from "encoding/json";function f(a:string[]):void{const [_,_]=j.Marshal(a);}`, "cannot use string[] as any"},
		{"decoder", `import go j from "encoding/json";struct Row{public text:string;}function f(a:*Row):void{const _=j.Unmarshal([],a);}`, "cannot use *Row as any"},
		{"assertion", `import go r from "reflect";function f(a:r.Value):string{return a.Interface() as! string;}`, "cannot establish"},
		{"nested capability", `import go r from "reflect";function f(a:string[]):boolean{return r.DeepEqual(r.ValueOf(a),r.ValueOf(a));}`, "cannot use string[] as any"},
		{"later capability", `import go r from "reflect";function f(a:string[]):void{r.DeepEqual(a,a);r.ValueOf(a);}`, "cannot use string[] as any"},
		{"nested null access", `import go r from "reflect";class C{public text:string="";}function f(v:C|null):boolean{return r.DeepEqual(v.text,v.text);}`, "must be checked against null"},
		{"Task is not value", `import go r from "reflect";function work():int{return 1;}function f():boolean{const task=go work();return r.DeepEqual(task,task);}`, "Task"},
		{"unknown function value", `import go r from "reflect";const eq=r.DeepEqual;function f(a:string[]):boolean{return eq(a,a);}`, "cannot use string[] as any"},
		{"shadowed package", `import go r from "reflect";class Fake{public function DeepEqual(a:interface{},b:interface{}):boolean{return false;}}function f(a:string[]):boolean{const r=new Fake();return r.DeepEqual(a,a);}`, "cannot use string[] as interface{}"},
		{"shadowed named import", `import go {DeepEqual as eq} from "reflect";function f(a:string[]):boolean{const eq=risky;return eq(a,a);}function risky(a:interface{},b:interface{}):boolean{return false;}`, "cannot use string[] as interface{}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.want == "" {
				if len(diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", diagnostics)
				}
			} else if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics %v; want %q", diagnostics, test.want)
			}
		})
	}
}
