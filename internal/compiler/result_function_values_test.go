package compiler

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestResultFunctionValuesPipeline(t *testing.T) {
	t.Parallel()
	for index, input := range []string{
		`function load():Result<int>{return ok(42);}function run():Result<int>{const f=load;return f();}`,
		`const load=():Result<int>=>{return ok(42);};function run():Result<int>{return load();}`,
		`function run():Result<int>{const f=():Result<int>=>{return ok(42);};const value=f()?;return ok(value);}`,
		`function run():Result<int>{const f:()=>Result<int>=()=>{return ok(42);};return f();}`,
		`function run():Result<int>{let f=():Result<int>=>{return ok(1);};f=()=>{return ok(42);};return f();}`,
		`function run():Result<int>{const f=(n:int):Result<int>=>{if(n==0){return ok(1);}const value=f(n-1)?;return ok(n*value);};return f(5);}`,
		`function run():Result<int>{const first=():Result<int>=>{return second();};const second=():Result<int>=>{return ok(42);};return first();}`,
		`function run():Result<int>{for(const f=(n:int):Result<int>=>{if(n==0){return ok(1);}return f(n-1);};false;){return f(2);}return ok(0);}`,
		`function run():Result<void>{const f=():Result<void>=>{return ok();};f()?;return ok();}`,
		`function run():error{const f=():Result<void>=>{return ok();};const [err]=f();return err;}`,
		`function factory():()=>Result<int>{return ()=>{return ok(42);};}function run():Result<int>{return factory()();}`,
		`function factory():Result<()=>Result<int>>{return ok(()=>{return ok(42);});}function run():Result<int>{const f=factory()?;return f();}`,
		`alias Load=()=>Result<int>;function run():Result<int>{const f:Load=()=>{return ok(42);};return f();}`,
		`type Load=distinct ()=>Result<int>;function run():Result<int>{const f:Load=()=>{return ok(42);};return f();}`,
		`type Load<T>=distinct ()=>Result<T>;type Other<T>=distinct Load<T>;function run():Result<int>{const f:Other<int>=()=>{return ok(42);};return f();}`,
		`type A=distinct ()=>Result<int>;type B=distinct ()=>Result<int>;function run(f:A):Result<int>{const g=B(f);return g();}`,
		`type Load=distinct ()=>Result<int>;function run(f:Load|null):Result<int>{if(f===null){return ok(0);}return f();}`,
		`alias Load<T>=()=>Result<T>;function run<T>(value:T):Result<T>{const f:Load<T>=()=>{return ok(value);};return f();}`,
		`function apply<T>(value:T,f:(value:T)=>Result<T>):Result<T>{return f(value);}function run():Result<int>{return apply(42,(value)=>{return ok(value);});}`,
		`function apply<T>(f:()=>Result<T>):Result<T>{return f();}function run():Result<int>{return apply(():Result<int>=>{return ok(42);});}`,
		`alias Load=()=>Result<int>;function run():Result<int>{const values:Load[]=[()=>{return ok(42);}];return values[0]();}`,
		`alias Load=()=>Result<int>;function run():Result<int>{const values:[1]Load=[()=>{return ok(42);}];return values[0]();}`,
		`alias Load=()=>Result<int>;function run():Result<int>{const values=makeMap<string,Load>();values["x"]=()=>{return ok(42);};return values["x"]();}`,
		`struct S{load:()=>Result<int>;}function run():Result<int>{const s=S{load:()=>{return ok(42);}};return s.load();}`,
		`function run():Result<int>{const s:{load:()=>Result<int>}={load:()=>{return ok(42);}};return s.load();}`,
		`class C{public load:()=>Result<int>=()=>{return ok(42);};}function run():Result<int>{return new C().load();}`,
		`class C{public function load():Result<int>{return ok(42);}}function run():Result<int>{const c=new C();const f=c.load;return f();}`,
		`class C{public static function load():Result<int>{return ok(42);}}function run():Result<int>{const f=C.load;return f();}`,
		`interface Loader{function load():Result<int>;}class C implements Loader{public function load():Result<int>{return ok(42);}}function run(c:Loader):Result<int>{const f=c.load;return f();}`,
		`class C{constructor(public value:int){}}struct S{load:()=>Result<C>;}function run():Result<C>{const s=S{load:()=>{return ok(new C(42));}};return s.load();}`,
		`class C{constructor(public value:int){}}function run():Result<C>{const f=():Result<C>=>{return ok(new C(42));};const p=&f;return (*p)();}`,
		`alias Load=()=>Result<int>;function run():Result<int>{const c=goChannel<Load>(1);c<-()=>{return ok(42);};const f=<-c;return f();}`,
		`class C{constructor(public value:int){}}alias Load=()=>Result<C>;function run():Result<C>{const c=goChannel<Load>(1);c<-()=>{return ok(new C(42));};const f=<-c;return f();}`,
		`alias Load=()=>Result<int>;function run(f:Load|null):Result<int>{if(f!==null){return f();}return ok(0);}`,
		`function run():Result<int>{const f=():Result<int>=>{return ok(42);};const task=go f();const value=await task?;return ok(value);}`,
		`function run():Result<int>{const f=():Result<int>=>{try{return ok(42);}finally{}};return f();}`,
		`import go strconv from "strconv";function run():Result<int>{const f:(s:string)=>Result<int>=strconv.Atoi;return f("42");}`,
		`import go { OnceValue } from "sync";function run():Result<int>{const f=OnceValue(()=>{return ():Result<int>=>{return ok(42);};});const load:()=>Result<int>=f();return load();}`,
		`import go fs from "io/fs";function use(f:fs.WalkDirFunc):void{}function run():void{const f=(path:string,entry:fs.DirEntry,err:error):Result<void>=>{return ok();};use(f);}`,
	} {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			t.Log(input)
			_, accepted, err := compilePipelineProperty(input)
			if err != nil || !accepted {
				path := filepath.Join(t.TempDir(), "result.km")
				checked, checkErr := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
				t.Fatalf("accepted=%v err=%v check=%v diagnostics=%v", accepted, err, checkErr, checked.Diagnostics)
			}
		})
	}
}

func TestResultFunctionValueDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, want string }{
		{`function run():void{const f=():Result<int>=>{return ok(42);};f();}`, "must be consumed"},
		{`function run():void{const f=():Result<int>=>{return ok(42);};const value=f();}`, "must be consumed"},
		{`function run():int{const f=():Result<int>=>{return ok(42);};const value=f()?;return value;}`, "operator ? may only be used"},
		{`function run():void{const f=():Result<int>=>{return ok(42);};defer f();}`, "defer cannot discard"},
		{`function run():void{const f=():Result<int>=>{return ok(42);};const [a,b,c]=f();}`, "Result binding count mismatch"},
		{`function run():void{const f=():Result<int>=>{return 42;};}`, "must return ok(...) or fail(...)"},
		{`function run():void{const f:()=>Result<int>=()=>{return ok("bad");};}`, "cannot use string"},
		{`function run():void{const f:()=>Result<int>=()=>ok(42);}`, "require a block body"},
		{`function run(f:(v:Result<int>)=>int):void{}`, "not in its parameters"},
		{`function run(f:()=>Result<int>[]):void{}`, "Result cannot be nested inside an array"},
		{`function run(f:()=>Result<Result<int>>):void{}`, "nested Result"},
		{`function run(f:()=>Task<int>):void{}`, "Task is not supported"},
		{`function run():void{const f=()=>{return ok(42);};}`, "only be used inside a Result-returning function"},
		{`function run():Result<int>{const f=()=>{return ok(42);};return f();}`, "only be used inside a Result-returning function"},
		{`function run():Result<int>{const f=():Result<int>=>{return ok(42);};return ok(f()?);}`, "result propagation may only be used"},
		{`import go strconv from "strconv";function run():Result<int>{return strconv.Atoi("42");}`, "not implicitly converted"},
		{`alias Maybe=int[]|null;function run():void{const f:()=>Result<int[]>=():Result<Maybe>=>{return ok(null);};}`, "cannot use"},
		{`alias Maybe=int[]|null;type Load=distinct ()=>Result<int[]>;function run():void{const f:Load=():Result<Maybe>=>{return ok(null);};}`, "cannot use"},
		{`type A=distinct ()=>Result<int>;type B=distinct ()=>Result<int>;function run(f:A):void{const g:B=f;}`, "cannot use"},
	} {
		path := filepath.Join(t.TempDir(), "invalid.km")
		result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: test.input})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.Diagnostics {
			found = found || strings.Contains(d.Message, test.want)
		}
		if !found {
			t.Fatalf("%s: %v, want %s", test.input, result.Diagnostics, test.want)
		}
	}
}
