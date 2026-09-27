package sema

import (
	goast "go/ast"
	"go/importer"
	goparser "go/parser"
	"go/token"
	gotypes "go/types"
	"strings"
	"testing"
)

func TestUnsafeMultipleResultsMatchGo(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, km, goSource string
		valid              bool
	}{
		{"slice", `function pair(p:*int):(*int,int){return p,1;}function f(p:*int):int[]{return u.Slice(pair(p));}`, `func pair(p *int)(*int,int){return p,1};func f(p *int)[]int{return u.Slice(pair(p))}`, true},
		{"string", `function pair(p:*byte):(*byte,uint64){return p,1;}function f(p:*byte):string{return u.String(pair(p));}`, `func pair(p *byte)(*byte,uint64){return p,1};func f(p *byte)string{return u.String(pair(p))}`, true},
		{"add", `function pair(p:u.Pointer):(u.Pointer,int64){return p,-1;}function f(p:u.Pointer):u.Pointer{return u.Add(pair(p));}`, `func pair(p u.Pointer)(u.Pointer,int64){return p,-1};func f(p u.Pointer)u.Pointer{return u.Add(pair(p))}`, true},
		{"generic slice", `function pair<T>(p:*T):(*T,int){return p,1;}function f<T>(p:*T):T[]{return u.Slice(pair(p));}`, `func pair[T any](p *T)(*T,int){return p,1};func f[T any](p *T)[]T{return u.Slice(pair(p))}`, true},
		{"generic integer", `constraint I=~int|~uint64;function pair<N extends I>(p:*byte,n:N):(*byte,N){return p,n;}function f<N extends I>(p:*byte,n:N):string{return u.String(pair(p,n));}`, `type I interface{~int|~uint64};func pair[N I](p *byte,n N)(*byte,N){return p,n};func f[N I](p *byte,n N)string{return u.String(pair(p,n))}`, true},
		{"named pointer", `type P=distinct *int;function pair(p:P):(P,int){return p,1;}function f(p:P):int[]{return u.Slice(pair(p));}`, `type P *int;func pair(p P)(P,int){return p,1};func f(p P)[]int{return u.Slice(pair(p))}`, true},
		{"slice wrong pointer", `function pair():(int,int){return 0,1;}function f():int[]{return u.Slice(pair());}`, `func pair()(int,int){return 0,1};func f()[]int{return u.Slice(pair())}`, false},
		{"string wrong pointer", `function pair(p:*int):(*int,int){return p,1;}function f(p:*int):string{return u.String(pair(p));}`, `func pair(p *int)(*int,int){return p,1};func f(p *int)string{return u.String(pair(p))}`, false},
		{"add wrong pointer", `function pair(p:*int):(*int,int){return p,1;}function f(p:*int):u.Pointer{return u.Add(pair(p));}`, `func pair(p *int)(*int,int){return p,1};func f(p *int)u.Pointer{return u.Add(pair(p))}`, false},
		{"runtime float length", `function pair(p:*int):(*int,float){return p,1;}function f(p:*int):int[]{return u.Slice(pair(p));}`, `func pair(p *int)(*int,float64){return p,1};func f(p *int)[]int{return u.Slice(pair(p))}`, false},
		{"runtime float offset", `function pair(p:u.Pointer):(u.Pointer,float){return p,1;}function f(p:u.Pointer):u.Pointer{return u.Add(pair(p));}`, `func pair(p u.Pointer)(u.Pointer,float64){return p,1};func f(p u.Pointer)u.Pointer{return u.Add(pair(p))}`, false},
		{"wrong count", `function pair(p:*int):(*int,int,int){return p,1,2;}function f(p:*int):int[]{return u.Slice(pair(p));}`, `func pair(p *int)(*int,int,int){return p,1,2};func f(p *int)[]int{return u.Slice(pair(p))}`, false},
		{"mixed arguments", `function pair(p:*int):(*int,int){return p,1;}function f(p:*int):int[]{return u.Slice(pair(p),1);}`, `func pair(p *int)(*int,int){return p,1};func f(p *int)[]int{return u.Slice(pair(p),1)}`, false},
		{"spread", `function pair(p:*int):(*int,int){return p,1;}function f(p:*int):int[]{return u.Slice(pair(p)...);}`, `func pair(p *int)(*int,int){return p,1};func f(p *int)[]int{return u.Slice(pair(p)...)}`, false},
		{"single operand builtin", `function pair():(int,int){return 1,2;}function f():uintptr{return u.Sizeof(pair());}`, `func pair()(int,int){return 1,2};func f()uintptr{return u.Sizeof(pair())}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := goparser.ParseFile(fset, "reference.go", `package reference;import u "unsafe";`+test.goSource, 0)
			if err != nil {
				t.Fatal(err)
			}
			config := gotypes.Config{Importer: importer.Default()}
			_, err = config.Check("reference", fset, []*goast.File{file}, nil)
			if (err == nil) != test.valid {
				t.Fatalf("Go oracle: %v (valid=%v)", err, test.valid)
			}
			diagnostics := checkSourceWithPolicy(t, `import go u from "unsafe";`+test.km, GoInteropPolicy{AllowUnsafe: true})
			if (len(diagnostics) == 0) != test.valid {
				t.Fatalf("Kinmokusei: %v (valid=%v)", diagnostics, test.valid)
			}
		})
	}
}

func TestUnsafeMultipleResultsBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ source, want string }{
		{`function pair(p:*int):(*int,int){return p,1;}function f(p:*int):int[]{return u.Slice<int>(pair(p));}`, "does not accept type arguments"},
		{`function pair(p:*int):Result<int>{return ok(1);}function f(p:*int):int[]{return u.Slice(pair(p));}`, "Result values must"},
		{`function pair(p:*int):(*int,int){return p,1;}function f(p:*int):void{u.Slice(pair(p));}`, "result must be used"},
		{`function pair(p:*int):(*int,int){return p,1;}function f(p:*int):void{defer u.Slice(pair(p));}`, "cannot discard the result"},
		{`function pair(p:*int):(*int,int){return p,1;}function f(p:*int):void{go u.Slice(pair(p));}`, "cannot discard the result"},
	} {
		got := strings.Join(checkSourceWithPolicy(t, `import go u from "unsafe";`+test.source, GoInteropPolicy{AllowUnsafe: true}), "\n")
		if !strings.Contains(got, test.want) {
			t.Fatalf("want %q, got %s", test.want, got)
		}
	}
	if got := checkSourceWithPolicy(t, `import go u from "unsafe";function pair(p:*int):(*int,int){return p,1;}function f(p:*int):int[]{return u.Slice(pair(p));}`, GoInteropPolicy{}); len(got) == 0 {
		t.Fatal("unsafe policy bypassed")
	}
}
