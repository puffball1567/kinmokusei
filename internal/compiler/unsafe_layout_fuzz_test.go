package compiler

import (
	"go/types"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/sema"
)

var unsafeLayoutSeeds = []string{
	`import go u from "unsafe";const width=int(u.Sizeof(int(0)));function f():int{return width;}`,
	`import go {Sizeof as size} from "unsafe";class C{public static const width:int=int(size(int64(0)));}`,
	`import go u from "unsafe";function f<T>(x:T):int{const n=u.Sizeof(x);const p=&n;return int(*p);}`,
	`import go u from "unsafe";function f<T>(x:T[]):int{const n=u.Sizeof(x);return int(n);}`,
	`import go u from "unsafe";struct Box<T>{public x:T;}function f<T>(x:Box<T>):int{return int(u.Alignof(x));}`,
	`import go u from "unsafe";function f(n:uint):int{return int(u.Sizeof(2147483648<<n));}`,
	`import go u from "unsafe";function f(n:uint):int{return int(u.Sizeof(1.0<<n));}`,
	`import go u from "unsafe";function get():int{return 1;}function f(a:[1][2]int,i:int):int{const n=len(a[i+int(u.Sizeof(get()))]);const p=&n;return *p;}`,
	`import go u from "unsafe";function f(x:*int|null):int{return int(u.Sizeof(x));}`,
	`import go u from "unsafe";const mask=^uint(0);const converted=uint32(mask);`,
	`import go u from "unsafe";struct S{private pad:byte;private value:int64;}function f(s:S):int{return int(u.Offsetof(s.value));}`,
	`import go u from "unsafe";struct Box<T>{private pad:byte;public value:T;}function f<T>(b:Box<T>):int{const n=u.Offsetof(b.pad);const p=&n;return int(*p);}`,
	`import go u from "unsafe";struct S{private value:int64;}alias A=S;type D=distinct S;function f(s:A):int{const d=D(s);return int(u.Offsetof(d.value));}`,
	`import go u from "unsafe";function f(s:{z:int64,a:byte}):int{return int(u.Offsetof(s.z));}`,
}

func TestUnsafeLayoutPipelineProperties(t *testing.T) {
	t.Parallel()
	for _, arch := range []string{"386", "amd64"} {
		generated := 0
		for _, seed := range unsafeLayoutSeeds {
			_, reached, err := compilePipelinePropertyWithPolicy(seed, sema.GoInteropPolicy{AllowUnsafe: true, Sizes: types.SizesFor("gc", arch)})
			if err != nil {
				t.Fatalf("%s: %s: %v", arch, seed, err)
			}
			if reached {
				generated++
			}
		}
		if generated < 7 {
			t.Fatalf("%s: only %d seeds reached codegen", arch, generated)
		}
	}
}

func FuzzUnsafeLayoutPipeline(f *testing.F) {
	for _, seed := range unsafeLayoutSeeds {
		f.Add(seed, false)
		f.Add(seed, true)
	}
	f.Fuzz(func(t *testing.T, input string, narrow bool) {
		if len(input) > 64<<10 {
			t.Skip()
		}
		arch := "amd64"
		if narrow {
			arch = "386"
		}
		if _, _, err := compilePipelinePropertyWithPolicy(input, sema.GoInteropPolicy{AllowUnsafe: true, Sizes: types.SizesFor("gc", arch)}); err != nil {
			t.Fatalf("%s: input %q: %v", arch, input, err)
		}
	})
}
