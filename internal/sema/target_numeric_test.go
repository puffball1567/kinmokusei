package sema

import (
	goast "go/ast"
	"go/importer"
	goparser "go/parser"
	gotoken "go/token"
	gotypes "go/types"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func TestTargetIntegerConstantsMatchGo(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, km, goSource string }{
		{"int upper", `function f():int{return 2147483648;}`, `func f()int{return 2147483648}`},
		{"array length", `function f(xs:[2147483648]byte):int{return len(xs);}`, `func f(xs [2147483648]byte)int{return len(xs)}`},
		{"enum implicit overflow", `enum Code{Last=2147483647,Next}`, `type Code int;const(Last Code=2147483647;Next Code=2147483648)`},
		{"int lower", `function f():int{return -2147483649;}`, `func f()int{return -2147483649}`},
		{"uint upper", `function f():uint{return 4294967296;}`, `func f()uint{return 4294967296}`},
		{"wide explicit", `function f():uint64{return 4294967296;}`, `func f()uint64{return 4294967296}`},
		{"inferred storage", `function f():void{let value=2147483648;}`, `func f(){value:=2147483648;_ = value}`},
		{"constant precision", `const large=1<<80;function f():int{return large>>80;}`, `const large=1<<80;func f()int{return large>>80}`},
		{"typed overflow", `function f():int{return int(2147483647)+1;}`, `func f()int{return int(2147483647)+1}`},
		{"global typed overflow", `const value:int=2147483648;`, `const value int=2147483648`},
		{"global inferred storage", `let value=2147483648;`, `var value=2147483648`},
		{"global complement", `const mask=^uint(0);function f():uint32{return uint32(mask);}`, `const mask=^uint(0);func f()uint32{return uint32(mask)}`},
		{"global forward complement", `const result=uint32(mask);const mask=^uint(0);`, `const result=uint32(mask);const mask=^uint(0)`},
		{"class constant complement", `class C{public static const n:uint32=uint32(^uint(0));}`, `const n uint32=uint32(^uint(0))`},
		{"constant machine complement", `function f():uint32{return uint32(^uint(0));}`, `func f()uint32{return uint32(^uint(0))}`},
		{"runtime machine complement", `function f(x:uint32):uint32{return x+uint32(^uint(0));}`, `func f(x uint32)uint32{return x+uint32(^uint(0))}`},
		{"int conversion", `function f():int{return int(1e10);}`, `func f()int{return int(1e10)}`},
		{"index", `function f(xs:int[]):int{return xs[2147483648];}`, `func f(xs []int)int{return xs[2147483648]}`},
		{"index maximum", `function f(xs:int[]):int{return xs[int(^uint(0)>>1)];}`, `func f(xs []int)int{return xs[int(^uint(0)>>1)]}`},
		{"slice", `function f(xs:int[]):int[]{return xs[:2147483648];}`, `func f(xs []int)[]int{return xs[:2147483648]}`},
		{"allocation", `function f():int[]{return make[int[]](2147483648);}`, `func f()[]int{return make([]int,2147483648)}`},
		{"map hint", `function f():Map<int,int>{return makeMap<int,int>(2147483648);}`, `func f()map[int]int{return make(map[int]int,2147483648)}`},
		{"channel capacity", `function f():int{return cap(goChannel<int>(2147483648));}`, `func f()int{return cap(make(chan int,2147483648))}`},
		{"runtime shift int", `function f(n:uint):int{return 2147483648<<n;}`, `func f(n uint)int{return 2147483648<<n}`},
		{"runtime shift wide", `function f(n:uint):uint64{return 4294967296<<n;}`, `func f(n uint)uint64{return 4294967296<<n}`},
		{"generic bound", `constraint Word=~int|~uint;function f<T extends Word>():T{return T(2147483648);}`, `type word interface{~int|~uint};func f[T word]()T{return T(2147483648)}`},
		{"generic wide", `constraint Wide=~int64|~uint64;function f<T extends Wide>():T{return T(2147483648);}`, `type wide interface{~int64|~uint64};func f[T wide]()T{return T(2147483648)}`},
		{"integer range", `function f():void{for(const n of 2147483648){}}`, `func f(){for n:=range 2147483648{_ = n}}`},
		{"typed range", `function f():void{for(const n of uint64(2147483648)){}}`, `func f(){for n:=range uint64(2147483648){_ = n}}`},
		{"Go named call", `import go {Compare as compare} from "cmp";function f():int{return compare<int>(2147483648,0);}`, `import "cmp";func f()int{return cmp.Compare[int](2147483648,0)}`},
		{"Go inferred call", `import go cmp from "cmp";function f():int{return cmp.Compare(2147483648,0);}`, `import "cmp";func f()int{return cmp.Compare(2147483648,0)}`},
		{"Go inferred wide call", `import go cmp from "cmp";function f():int{return cmp.Compare(2147483648,uint64(0));}`, `import "cmp";func f()int{return cmp.Compare(2147483648,uint64(0))}`},
		{"Go interface argument", `import go fmt from "fmt";function f():string{return fmt.Sprint(2147483648);}`, `import "fmt";func f()string{return fmt.Sprint(2147483648)}`},
		{"layout word size", `import go u from "unsafe";const size=u.Sizeof(int(0));function f():byte{return byte(248+size);}`, `import "unsafe";const size=unsafe.Sizeof(int(0));func f()byte{return byte(248+size)}`},
		{"layout alignment", `import go u from "unsafe";const size=u.Alignof(int64(0));function f():byte{return byte(248+size);}`, `import "unsafe";const size=unsafe.Alignof(int64(0));func f()byte{return byte(248+size)}`},
		{"layout offset", `import go u from "unsafe";import go r from "reflect";let h:r.StringHeader=r.StringHeader{};const offset=u.Offsetof(h.Len);function f():byte{return byte(248+offset);}`, `import("unsafe";"reflect");var h reflect.StringHeader;const offset=unsafe.Offsetof(h.Len);func f()byte{return byte(248+offset)}`},
		{"layout inferred argument", `import go u from "unsafe";const size=u.Sizeof(2147483648);`, `import "unsafe";const size=unsafe.Sizeof(2147483648)`},
		{"layout shifted argument", `import go u from "unsafe";function f(n:uint):int{return int(u.Sizeof(2147483648<<n));}`, `import "unsafe";func f(n uint)int{return int(unsafe.Sizeof(2147483648<<n))}`},
		{"source layout offset", `import go u from "unsafe";struct S{public pad:byte;private value:int64;}let s:S=S{pad:0,value:0};const offset=u.Offsetof(s.value);function f():byte{return byte(248+offset);}`, `import "unsafe";type S struct{pad byte;value int64};var s S;const offset=unsafe.Offsetof(s.value);func f()byte{return byte(248+offset)}`},
		{"source generic offset", `import go u from "unsafe";struct S<T>{public pad:byte;public value:T;}function f<T>(s:S<T>):int{const offset=u.Offsetof(s.pad);const p=&offset;return int(*p);}`, `import "unsafe";type S[T any] struct{pad byte;value T};func f[T any](s S[T])int{offset:=unsafe.Offsetof(s.pad);p:=&offset;return int(*p)}`},
		{"unsafe length", `import go {Slice as view} from "unsafe";function f(p:*int):int[]{return view(p,2147483648);}`, `import "unsafe";func f(p *int)[]int{return unsafe.Slice(p,2147483648)}`},
		{"unsafe offset", `import go u from "unsafe";function f(p:u.Pointer):u.Pointer{return u.Add(p,2147483648);}`, `import "unsafe";func f(p unsafe.Pointer)unsafe.Pointer{return unsafe.Add(p,2147483648)}`},
		{"typed switch", `function f(x:uint):int{switch(x){case ^uint(0){return 1;}case uint(4294967295){return 2;}default{return 0;}}}`, `func f(x uint)int{switch x{case ^uint(0):return 1;case uint(4294967295):return 2;default:return 0}}`},
	}
	for _, arch := range []string{"386", "amd64", "arm", "arm64"} {
		for _, test := range tests {
			t.Run(arch+"/"+test.name, func(t *testing.T) {
				tokens, ld := lexer.Lex("target.km", test.km)
				program, pd := parser.Parse(tokens)
				if len(ld) != 0 || len(pd) != 0 {
					t.Fatalf("parse %v %v", ld, pd)
				}
				sizes := gotypes.SizesFor("gc", arch)
				diagnostics := CheckScopedWithGoImporterAndPolicy(program, nil, nil, GoInteropPolicy{Sizes: sizes, AllowUnsafe: true})
				fset := gotoken.NewFileSet()
				file, err := goparser.ParseFile(fset, "reference.go", "package reference;"+test.goSource, 0)
				if err != nil {
					t.Fatal(err)
				}
				config := gotypes.Config{Sizes: sizes, Importer: importer.Default()}
				_, goErr := config.Check("reference", fset, []*goast.File{file}, nil)
				if (len(diagnostics) == 0) != (goErr == nil) {
					t.Fatalf("Go=%v Kinmokusei=%v", goErr, diagnostics)
				}
				for _, d := range diagnostics {
					if d.Span.Path != "target.km" {
						t.Fatalf("non-source diagnostic: %v", d)
					}
				}
			})
		}
	}
}

func TestTargetNumericExpressionContexts(t *testing.T) {
	t.Parallel()
	c := &Checker{goSizes: gotypes.SizesFor("gc", "386")}
	for _, test := range []struct {
		input, constant string
		valid           bool
	}{
		{"uint32(^uint(0))", "4294967295", true},
		{"x+uint32(^uint(0))", "", true},
		{"accept(uint32(^uint(0)))", "", true},
		{"int(2147483647)+1", "", false},
		{"uint8(256)", "", false},
		{"accept(4294967296)", "", false},
		{"4294967296<<n", "", true},
	} {
		t.Run(test.input, func(t *testing.T) {
			pkg := gotypes.NewPackage("probe", "probe")
			pkg.Scope().Insert(gotypes.NewVar(0, pkg, "x", gotypes.Typ[gotypes.Uint32]))
			pkg.Scope().Insert(gotypes.NewVar(0, pkg, "n", gotypes.Typ[gotypes.Uint]))
			sig := gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(gotypes.NewVar(0, pkg, "", gotypes.Typ[gotypes.Uint32])), nil, false)
			pkg.Scope().Insert(gotypes.NewFunc(0, pkg, "accept", sig))
			expr, err := goparser.ParseExpr(test.input)
			if err != nil {
				t.Fatal(err)
			}
			value, err := c.evalNumericGo(pkg, expr)
			if (err == nil) != test.valid {
				t.Fatalf("value=%v err=%v", value, err)
			}
			if test.constant != "" && (value.Value == nil || value.Value.ExactString() != test.constant) {
				t.Fatalf("value=%v", value)
			}
			if pkg.Scope().Len() != 3 {
				t.Fatal("probe modified the original scope")
			}
		})
	}
}
