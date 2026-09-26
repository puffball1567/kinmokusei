package sema

import (
	"strings"
	"testing"
)

func TestSourceOffsetConstants(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, want string }{
		{"public field", `struct S{public pad:byte;public value:int64;}function f(s:S):void{const offset=u.Offsetof(s.value);const p=&offset;}`, "addressable"},
		{"private field", `struct S{public pad:byte;private value:int64;}function f(s:S):void{const offset=u.Offsetof(s.value);const p=&offset;}`, "addressable"},
		{"pointer field", `struct S{public pad:byte;public value:int64;}function f(s:*S):void{const offset=u.Offsetof(s.value);const p=&offset;}`, "addressable"},
		{"defined field", `struct S{public pad:byte;private value:int64;}type D=distinct S;function f(s:D):void{const offset=u.Offsetof(s.value);const p=&offset;}`, "addressable"},
		{"defined pointer", `struct S{public value:int64;}type D=distinct S;function f(s:*D):void{const offset=u.Offsetof(s.value);const p=&offset;}`, "addressable"},
		{"alias field", `struct S{public value:int64;}alias A=S;function f(s:A):void{const offset=u.Offsetof(s.value);const p=&offset;}`, "addressable"},
		{"object field", `function f(s:{z:int64,a:byte}):void{const offset=u.Offsetof(s.z);const p=&offset;}`, "addressable"},
		{"generic value runtime", `struct S<T>{public pad:byte;private value:T;}function f<T>(s:S<T>):void{const offset=u.Offsetof(s.value);const p=&offset;}`, ""},
		{"generic prefix runtime", `struct S<T>{public pad:byte;private value:T;}function f<T>(s:S<T>):void{const offset=u.Offsetof(s.pad);const p=&offset;}`, ""},
		{"generic header constant", `struct S<T>{public pad:byte;private values:T[];}function f<T>(s:S<T>):void{const offset=u.Offsetof(s.values);const p=&offset;}`, "addressable"},
		{"concrete instance constant", `struct S<T>{public pad:byte;private value:T;}function f(s:S<int64>):void{const offset=u.Offsetof(s.value);const p=&offset;}`, "addressable"},
		{"recursive pointer", `struct S{private next:*S;public value:int;}function f(s:S):int{return int(u.Offsetof(s.next.value));}`, ""},
		{"constant bounds", `struct S{public a:byte;public b:byte;}function f(s:S,a:[1]int):int{return a[u.Offsetof(s.b)];}`, "out of bounds"},
		{"nullable receiver", `struct S{public value:int;}function f(s:*S|null):int{return int(u.Offsetof(s.value));}`, "nullable"},
		{"method rejected", `struct S{public function value():int{return 1;}}function f(s:S):int{return int(u.Offsetof(s.value));}`, "requires a struct field selector"},
		{"property rejected", `class C{public get value():int{return 1;}}function f(c:C):int{return int(u.Offsetof(c.value));}`, "requires a struct field selector"},
		{"class storage rejected", `class C{public value:int=1;}function f(c:C):int{return int(u.Offsetof(c.value));}`, "requires a struct field selector"},
		{"missing field", `struct S{public value:int;}function f(s:S):int{return int(u.Offsetof(s.missing));}`, "has no field"},
		{"imported private field remains rejected", `import go time from "time";function f(t:time.Time):int{return int(u.Offsetof(t.wall));}`, "no exported member"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSourceWithPolicy(t, `import go u from "unsafe";`+test.source, GoInteropPolicy{AllowUnsafe: true}), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("got=%s want=%q", got, test.want)
			}
		})
	}
}
