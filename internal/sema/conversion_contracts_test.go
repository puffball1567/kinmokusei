package sema

import (
	"strings"
	"testing"
)

func TestConversionSourceContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source string
		reject       bool
	}{
		{"nullable Go conversion", `import go json from "encoding/json"; function f(v: byte[] | null): json.RawMessage { return json.RawMessage(v); }`, true},
		{"narrowed Go conversion", `import go json from "encoding/json"; function f(v: byte[] | null): json.RawMessage { if(v === null) { return json.RawMessage(makeSlice<byte>(0)); } return json.RawMessage(v); }`, false},
		{"Result payload", `alias Maybe = *int | null; type A = distinct () => Result<*int>; type B = distinct () => Result<Maybe>; function f(v: B): A { return A(v); }`, true},
		{"ordinary result", `alias Maybe = *int | null; type A = distinct () => *int; type B = distinct () => Maybe; function f(v: B): A { return A(v); }`, true},
		{"parameter", `alias Maybe = *int | null; type A = distinct (v: Maybe) => int; type B = distinct (v: *int) => int; function f(v: B): A { return A(v); }`, true},
		{"multiple result", `alias Maybe = *int | null; type A = distinct () => (*int, int); type B = distinct () => (Maybe, int); function f(v: B): A { return A(v); }`, true},
		{"Result to raw", `alias Maybe = *int | null; type A = distinct () => (*int, error); type B = distinct () => Result<Maybe>; function f(v: B): A { return A(v); }`, true},
		{"matching nullable result", `alias Maybe = *int | null; type A = distinct () => Result<Maybe>; type B = distinct () => Result<Maybe>; function f(v: B): A { return A(v); }`, false},
		{"matching Result to raw", `type A = distinct () => (*int, error); type B = distinct () => Result<*int>; function f(v: B): A { return A(v); }`, false},
		{"matching ordinary result", `type A = distinct () => *int; type B = distinct () => *int; function f(v: B): A { return A(v); }`, false},
		{"nested callable", `alias Maybe = *int | null; alias Ref = () => *int; alias Opt = () => Maybe; type A = distinct () => Ref; type B = distinct () => Opt; function f(v: B): A { return A(v); }`, true},
		{"slice elements", `alias Ptr = *int; alias Maybe = *int | null; type A = distinct Ptr[]; function f(v: Maybe[]): A { return A(v); }`, true},
		{"defined slice elements", `alias Ptr = *int; alias Maybe = *int | null; type A = distinct Ptr[]; type B = distinct Maybe[]; function f(v: B): A { return A(v); }`, true},
		{"generic defined slice elements", `alias Maybe = *int | null; type A<T> = distinct T[]; function f(v: A<Maybe>): A<*int> { return A<*int>(v); }`, true},
		{"nullable widening also changes writable contract", `alias Ptr = *int; alias Maybe = *int | null; type A = distinct Maybe[]; function f(v: Ptr[]): A { return A(v); }`, true},
		{"matching nullable slice", `alias Maybe = *int | null; type A = distinct Maybe[]; function f(v: Maybe[]): A { return A(v); }`, false},
		{"constraint target", `alias Ptr = *int; alias Maybe = *int | null; constraint A = ~Ptr[]; function f<T extends A>(v: Maybe[]): T { return T(v); }`, true},
		{"constraint source", `alias Ptr = *int; alias Maybe = *int | null; constraint A = ~Maybe[]; type B = distinct Ptr[]; function f<T extends A>(v: T): B { return B(v); }`, true},
		{"between constraints", `alias Ptr = *int; alias Maybe = *int | null; constraint A = ~Maybe[]; constraint B = ~Ptr[]; function f<T extends A, U extends B>(v: T): U { return U(v); }`, true},
		{"function constraint", `alias Maybe = *int | null; alias Fn = () => Maybe; constraint A = ~Fn; type B = distinct () => *int; function f<T extends A>(v: T): B { return B(v); }`, true},
		{"map key", `alias Maybe = *int | null; type A = distinct Map<*int, int>; type B = distinct Map<Maybe, int>; function f(v: B): A { return A(v); }`, true},
		{"map value", `alias Maybe = *int | null; type A = distinct Map<int, *int>; type B = distinct Map<int, Maybe>; function f(v: B): A { return A(v); }`, true},
		{"array element", `alias Maybe = *int | null; type A = distinct [1]*int; type B = distinct [1]Maybe; function f(v: B): A { return A(v); }`, true},
		{"channel element", `alias Maybe = *int | null; type A = distinct GoChannel<*int>; type B = distinct GoChannel<Maybe>; function f(v: B): A { return A(v); }`, true},
		{"struct field", `alias Maybe = *int | null; struct A { public value: *int; } struct B { public value: Maybe; } function f(v: B): A { return A(v); }`, true},
		{"struct and object field", `alias Maybe = *int | null; struct A { public value: *int; } function f(v: {Value: Maybe}): A { return A(v); }`, true},
		{"recursive struct identity", `struct A { public next: *A; } function f(v: A): A { return A(v); }`, false},
		{"Go callback", `import go http from "net/http"; alias Maybe = *http.Request | null; function f(v: (w: http.ResponseWriter, r: Maybe) => void): http.HandlerFunc { return http.HandlerFunc(v); }`, true},
		{"matching Go callback", `import go http from "net/http"; function f(v: (w: http.ResponseWriter, r: *http.Request) => void): http.HandlerFunc { return http.HandlerFunc(v); }`, false},
		{"Go Result void callback", `import go fs from "io/fs"; function f(v: (path: bstring, entry: fs.DirEntry, err: error) => Result<void>): fs.WalkDirFunc { return fs.WalkDirFunc(v); }`, false},
		{"matching constraint", `alias Maybe = *int | null; constraint A = ~Maybe[]; function f<T extends A>(v: Maybe[]): T { return T(v); }`, false},
		{"recursive identity", `type A = distinct A[]; function f(v: A): A { return A(v); }`, false},
		{"recursive underlying", `type A = distinct A[]; alias B = A[]; function f(v: B): A { return A(v); }`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if test.reject {
				if !strings.Contains(strings.Join(diagnostics, "\n"), "cannot convert") {
					t.Fatalf("diagnostics = %v, want conversion rejection", diagnostics)
				}
			} else if len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
			if test.reject && strings.Contains(test.source, "*int | null") {
				// Prove that the qualifier, not an unrelated Go storage mismatch,
				// is the reason this conversion is rejected.
				matched := strings.ReplaceAll(test.source, "*int | null", "*int")
				if diagnostics := checkSource(t, matched); len(diagnostics) != 0 {
					t.Fatalf("matching contracts rejected: %v", diagnostics)
				}
			}
		})
	}
}
